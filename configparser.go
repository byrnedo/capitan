package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/byrnedo/capitan/container"
	"github.com/byrnedo/capitan/helpers"
	"github.com/byrnedo/capitan/logger"
	"github.com/codegangsta/cli"
	"github.com/codeskyblue/go-sh"
	"github.com/mgutz/str"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"
)

type ConfigParser struct {
	// command to obtain config from
	Command string
	// args given to cli
	Args cli.Args
}

func NewSettingsParser(cmd string, args cli.Args) *ConfigParser {
	return &ConfigParser{
		Command: cmd,
		Args:    args,
	}
}

func (f *ConfigParser) Run() (*ProjectConfig, error) {
	var (
		output   []byte
		err      error
		cmdSlice []string
		cmdArgs  []interface{}
	)
	if len(f.Command) == 0 {
		return nil, errors.New("Command must not be empty")
	}

	if cmdSlice = str.ToArgv(f.Command); len(cmdSlice) > 1 {
		cmdArgs = helpers.ToInterfaceSlice(cmdSlice[1:])
	} else {
		cmdArgs = []interface{}{}
	}

	ses := sh.NewSession()
	if output, err = ses.Command(cmdSlice[0], cmdArgs...).Output(); err != nil {
		return nil, err
	}
	settings, err := f.parseOutput(output)
	return settings, err

}

func (f *ConfigParser) parseOutput(out []byte) (*ProjectConfig, error) {
	lines := bytes.Split(out, []byte{'\n'})
	settings, err := f.parseSettings(lines)
	return settings, err

}

// The main parse function. Creates the final list of containers.
func (f *ConfigParser) parseSettings(lines [][]byte) (projSettings *ProjectConfig, err error) {
	//minimum of len1 at this point in parts

	cmdsMap := make(map[string]container.Container, 0)

	projName, _ := os.Getwd()
	projName = toSnake(path.Base(projName))
	projNameArr := strings.Split(projName, "_")
	projSettings = new(ProjectConfig)
	projSettings.ProjectName = projNameArr[len(projNameArr)-1]

	projSettings.ProjectSeparator = "_"

	for lineNum, line := range lines {

		line = bytes.TrimLeft(line, " ")
		if len(line) == 0 || line[0] == '#' {
			//comment
			continue
		}
		lineParts := bytes.SplitN(line, []byte{' '}, 3)
		if len(lineParts) < 2 {
			//not enough args on line
			continue
		}

		if string(lineParts[0]) == "global" {
			if len(lineParts) > 2 {
				switch string(lineParts[1]) {
				case "project":
					projSettings.ProjectName = string(lineParts[2])
				case "project_sep":
					projSettings.ProjectSeparator = stripChars(string(lineParts[2]), " \t")
				case "blue_green":
					projSettings.BlueGreenMode, _ = strconv.ParseBool(string(lineParts[2]))
				}
			}
			continue

		}

		contr := string(lineParts[0])

		if _, found := cmdsMap[contr]; !found {
			cmdsMap[contr] = container.Container{
				Placement: len(cmdsMap),
				Hooks:     make(map[string]*container.Hook, 0),
				Scale:     1,
			}
		}

		action := string(lineParts[1])
		setting := cmdsMap[contr]

		var args string
		if len(lineParts) > 2 {
			args = string(lineParts[2])
		}
		args = strings.TrimRight(args, " ")
		switch action {
		case "command":
			if len(args) > 0 {
				parsedArgs := str.ToArgv(args)
				for _, arg := range parsedArgs {
					setting.Command = append(setting.Command, arg)
				}
			}
		case "scale":
			if len(args) > 0 {
				scale, err := strconv.Atoi(args)
				if err != nil {
					return projSettings, errors.New(fmt.Sprintf("Failed to parse `scale` on line %d, %s", lineNum+1, err))
				}
				if scale < 1 {
					scale = 1
				}
				setting.Scale = scale
			}
		case "image":
			if len(args) > 0 {
				setting.Image = args
			}
		case "build":
			if len(args) > 0 {
				setting.Build = args
			}
		case "build-args":
			if len(args) > 0 {
				setting.BuildArgs = str.ToArgv(args)
			}
		case "link":

			argParts := strings.SplitN(args, ":", 2)

			var alias string
			if len(argParts) > 1 {
				alias = argParts[1]
			}

			newLink := container.Link{
				Container: argParts[0],
				Alias:     alias,
			}

			setting.Links = append(setting.Links, newLink)

		case "rm":
			setting.Remove = true
		case "hook":
			if len(args) > 0 {
				curHooks := setting.Hooks
				argParts := strings.SplitN(args, " ", 2)
				hookName := argParts[0]
				if len(argParts) > 1 {
					hookScript := argParts[1]

					hook := curHooks[hookName]
					if hook == nil {
						hook = new(container.Hook)
					}
					hook.Scripts = append(hook.Scripts, hookScript)
					curHooks[hookName] = hook
				}
				setting.Hooks = curHooks
			}
		case "volumes-from":
			argParts := strings.SplitN(args, " ", 2)
			setting.VolumesFrom = append(setting.VolumesFrom, argParts[0])
		case "global":
		default:
			if action != "" {
				setting.ContainerArgs = append(setting.ContainerArgs, "--"+action)
				if args != "" {
					setting.ContainerArgs = append(setting.ContainerArgs, args)
				}
			}
		}

		cmdsMap[contr] = setting
	}

	// Post process
	err = f.postProcessConfig(cmdsMap, projSettings)
	return

}

// Now that we have all settings do some house keeping and processing
func (f *ConfigParser) postProcessConfig(parsedConfig map[string]container.Container, projSettings *ProjectConfig) error {

	// TODO duplicate containers for scaling
	projSettings.ContainerList = make(SettingsList, 0)

	for name, item := range parsedConfig {
		item.Name = projSettings.ProjectName + projSettings.ProjectSeparator + name
		item.ServiceType = name
		item.ProjectName = projSettings.ProjectName
		item.ProjectNameSeparator = projSettings.ProjectSeparator

		// default image to name if 'build' is set
		if item.Build != "" {
			item.Image = item.Name
		}

		f.processScaleArg(&item)

		f.processCleanupTasks(projSettings, &item)

		// resolve links
		f.processLinks(parsedConfig, &item)

		// resolve volumes from
		f.processVolumesFrom(parsedConfig, &item)

		ctrsToAdd := f.scaleContainers(&item)

		projSettings.ContainerList = append(projSettings.ContainerList, ctrsToAdd...)
	}

	return nil
}

// Parse the volumes-from args and try and find first container with that type
func (f *ConfigParser) processVolumesFrom(parsedConfig map[string]container.Container, item *container.Container) {
	for i, ctrName := range item.VolumesFrom {
		// TODO Not sure how to do this for scaling
		if _, found := parsedConfig[ctrName]; found {
			ctrName = item.ProjectName + item.ProjectNameSeparator + ctrName + item.ProjectNameSeparator + "1"
			item.VolumesFrom[i] = ctrName
		}
	}
}

// Parse the link args and try and find first container with that type
func (f *ConfigParser) processLinks(parsedConfig map[string]container.Container, item *container.Container) {
	for i, link := range item.Links {
		// TODO right now, for scaling links are bad so just putting it to first container
		container := link.Container
		if _, found := parsedConfig[link.Container]; found {
			container = item.ProjectName + item.ProjectNameSeparator + link.Container + item.ProjectNameSeparator + "1"
		}
		link.Container = container
		item.Links[i] = link
	}
}

// Parse the scale argument and set the container's scale property
func (f *ConfigParser) processScaleArg(ctr *container.Container) {
	if f.Args.Get(0) == "scale" {
		if f.Args.Get(1) == ctr.ServiceType {
			if scaleArg, err := strconv.Atoi(f.Args.Get(2)); err == nil {
				if scaleArg > 0 {
					ctr.Scale = scaleArg
				}
			}
		}
	}
}

// Create list of containers to cleanup when scaling
func (f *ConfigParser) processCleanupTasks(projSettings *ProjectConfig, ctr *container.Container) {
	var tasks SettingsList
	svcs := helpers.InstancesOfService(ctr.Name)
	for _, existing := range svcs {
		instNum, err := helpers.GetNumericSuffix(existing.Name, ctr.ProjectNameSeparator)
		if err != nil || instNum < 0 || instNum > ctr.Scale {
			tempCtr := new(container.Container)
			*tempCtr = *ctr
			tempCtr.Name = existing.Name
			tasks = append(tasks, tempCtr)
		}
	}
	projSettings.ContainerCleanupList = append(projSettings.ContainerCleanupList, tasks...)
	return
}

// Create copies of containers which need to scale
func (f *ConfigParser) scaleContainers(ctr *container.Container) []*container.Container {

	ctrCopies := make([]*container.Container, ctr.Scale)

	for i := 0; i < ctr.Scale; i++ {
		ctrCopies[i] = new(container.Container)
		*ctrCopies[i] = *ctr
		ctrCopies[i].InstanceNumber = i + 1
		ctrCopies[i].Name = fmt.Sprintf("%s%s%d", ctr.Name, ctr.ProjectNameSeparator, i+1)
		ctrCopies[i].ServiceName = ctr.Name

		// HACK for container logging prefix width alignment, eg 'some_container | blahbla'
		if len(ctrCopies[i].Name) > logger.LongestContainerName {
			logger.LongestContainerName = len(ctrCopies[i].Name)
		}

		ctrCopies[i].RunArguments = ctrCopies[i].GetRunArguments()
	}

	return ctrCopies
}

func stripChars(str, chr string) string {
	return strings.Map(func(r rune) rune {
		if strings.IndexRune(chr, r) < 0 {
			return r
		}
		return -1
	}, str)
}

// toSnake convert the given string to snake case following the Golang format:
// acronyms are converted to lower-case and preceded by an underscore.
func toSnake(in string) string {
	runes := []rune(in)
	length := len(runes)

	var out []rune
	for i := 0; i < length; i++ {
		if i > 0 && unicode.IsUpper(runes[i]) && ((i+1 < length && unicode.IsLower(runes[i+1])) || unicode.IsLower(runes[i-1])) {
			out = append(out, '_')
		}
		out = append(out, unicode.ToLower(runes[i]))
	}

	return string(out)
}
