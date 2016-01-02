package main

import (
	"bytes"
	"github.com/codeskyblue/go-sh"
	"github.com/mgutz/str"
	"os"
	"path"
	"strings"
	"unicode"
)

type FileRunner struct {
	FilePath string
}

func NewFileRunner(path string) *FileRunner {
	return &FileRunner{
		FilePath: path,
	}
}

func (f *FileRunner) Run() (*ProjectSettings, error) {
	var (
		output []byte
		err    error
	)
	if output, err = sh.Command(f.FilePath).Output(); err != nil {
		return nil, err
	}
	settings, err := f.parseOutput(output)
	return &settings, err

}

func (f *FileRunner) parseOutput(out []byte) (ProjectSettings, error) {
	lines := bytes.Split(out, []byte{'\n'})
	settings, err := f.parseSettings(lines)
	return settings, err

}

type ProjectSettings struct {
	ProjectName           string
	ProjectSeparator      string
	ContainerSettingsList SettingsList
}

type Link struct {
	Container string
	Alias string
}

type ContainerSettings struct {
	Name        string
	Placement   int
	Args        []interface{}
	Image       string
	Build       string
	Command     []interface{}
	Links       []Link
	Hooks       map[string]string
	UniqueLabel string
}

type SettingsList []ContainerSettings

func (s SettingsList) Len() int {
	return len(s)
}
func (s SettingsList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SettingsList) Less(i, j int) bool {
	return s[i].Placement < s[j].Placement
}

func (f *FileRunner) parseSettings(lines [][]byte) (projSettings ProjectSettings, err error) {
	//minimum of len1 at this point in parts

	cmdsMap := make(map[string]ContainerSettings, 0)

	projName, _ := os.Getwd()
	projName = toSnake(path.Base(projName))
	projNameArr := strings.Split(projName, "_")
	projSettings.ProjectName = projNameArr[len(projNameArr)-1]

	projSettings.ProjectSeparator = "_"

	for _, line := range lines {

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

		container := string(lineParts[0])

		if _, found := cmdsMap[container]; !found {
			cmdsMap[container] = ContainerSettings{
				Placement: len(cmdsMap),
				Hooks: make(map[string]string, 0),
			}
		}

		action := string(lineParts[1])
		setting := cmdsMap[container]

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

			newLink := Link{
				Container: argParts[0],
				Alias: alias,
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
			if len(args) > 0 {
				argParts := strings.SplitN(args, " ", 1)
				if len(argParts) > 1 {
					switch argParts[0] {
					case "project":
						projSettings.ProjectName = argParts[1]
					case "project_sep":
						projSettings.ProjectSeparator = stripChars(argParts[1], " \t")
					}
				}
			}
		default:
			setting.Args = append(setting.Args, "--"+action)
			setting.Args = append(setting.Args, args)
		}

		cmdsMap[container] = setting
	}

	cmdsList := make(SettingsList, len(cmdsMap))
	var count = 0
	for name, item := range cmdsMap {
		item.Name = projSettings.ProjectName + projSettings.ProjectSeparator + name
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

// ToSnake convert the given string to snake case following the Golang format:
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
