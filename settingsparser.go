package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/byrnedo/capitan/container"
	"github.com/byrnedo/capitan/helpers"
	"github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"github.com/mgutz/str"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"
)

type SettingsParser struct {
	Command string
}

func NewSettingsParser(cmd string) *SettingsParser {
	return &SettingsParser{
		Command: cmd,
	}
}

func (f *SettingsParser) Run() (*ProjectSettings, error) {
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
	return &settings, err

}

func (f *SettingsParser) parseOutput(out []byte) (ProjectSettings, error) {
	lines := bytes.Split(out, []byte{'\n'})
	settings, err := f.parseSettings(lines)
	return settings, err

}

func (f *SettingsParser) parseSettings(lines [][]byte) (projSettings ProjectSettings, err error) {
	//minimum of len1 at this point in parts

	cmdsMap := make(map[string]container.Container, 0)

	projName, _ := os.Getwd()
	projName = toSnake(path.Base(projName))
	projNameArr := strings.Split(projName, "_")
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
				}
			}
			continue

		}

		contr := string(lineParts[0])

		if _, found := cmdsMap[contr]; !found {
			cmdsMap[contr] = container.Container{
				Placement: len(cmdsMap),
				Hooks:     make(map[string]string, 0),
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

		case "hook":
			if len(args) > 0 {
				curHooks := setting.Hooks
				argParts := strings.SplitN(args, " ", 2)
				if len(argParts) > 1 {
					curHooks[argParts[0]] = argParts[1]
				}
				setting.Hooks = curHooks
			}
		case "global":
		default:
			setting.Args = append(setting.Args, "--"+action)
			setting.Args = append(setting.Args, args)
		}

		cmdsMap[contr] = setting
	}

	// Post process
	// TODO duplicate containers for scaling
	cmdsList := make(SettingsList, len(cmdsMap))
	var count = 0
	for name, item := range cmdsMap {
		item.Name = projSettings.ProjectName + projSettings.ProjectSeparator + name

		if item.Build != "" {
			item.Image = item.Name
		}
		// Hack for logging prefix width alignment, eg 'some_container | blahbla'
		if len(item.Name) > logger.LongestContainerName {
			logger.LongestContainerName = len(item.Name)
		}
		for i, link := range item.Links {
			link.Container = projSettings.ProjectName + projSettings.ProjectSeparator + link.Container
			item.Links[i] = link
		}
		cmdsList[count] = item
		count++
	}
	projSettings.ContainerSettingsList = cmdsList
	return

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
