package main

import (
	"github.com/codeskyblue/go-sh"
	. "github.com/byrnedo/capitan/logger"
	"bytes"
	"strings"
	"github.com/mgutz/str"
)


type FileRunner struct {
	FilePath string
}

func NewFileRunner(path string) *FileRunner {
	return &FileRunner{
		FilePath: path,
	}
}


func (f *FileRunner) Run() (SettingsList, error){
	var (
		output []byte
		err error
	)
	pwd, _ := sh.Command("pwd").Output()
	Trace.Println(string(pwd))
	if output, err = sh.Command(f.FilePath).Output(); err != nil {
		return nil, err
	}
	cmdMap, err := f.parseOutput(output)
	return cmdMap, err

}

func (f *FileRunner) parseOutput(out []byte) (SettingsList, error) {
	lines := bytes.Split(out, []byte{'\n'} )
	cmdMap,err := f.createCommands(lines)
	return cmdMap, err

}


type Settings struct {
	Name string
	Placement int
	Args []interface{}
	Image string
	Build string
	Command []interface{}
	Depends []string
	Hooks map[string]string
	UniqueLabel string
}

type SettingsList []Settings

func (s SettingsList) Len() int {
	return len(s)
}
func (s SettingsList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SettingsList) Less(i, j int) bool {
	return s[i].Placement < s[j].Placement
}

func (f *FileRunner) createCommands(lines [][]byte) (cmdsList SettingsList, err error) {
	//minimum of len1 at this point in parts

	cmdsMap := make(map[string]Settings,0)

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

		if _, found := cmdsMap[container]; ! found {
			cmdsMap[container] = Settings{
				Placement: len(cmdsMap),
				Args: make([]interface{}, 0),
				Depends: make([]string, 0),
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
		switch(action){
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
		case "depends":
			setting.Depends = append(setting.Depends, args)
		case "hook":
			if len(args) > 1 {
				curHooks := setting.Hooks
				argParts := strings.SplitN(args, " ", 1)
				if len(argParts) > 1{
					curHooks[argParts[0]] = argParts[1]
				}
			}
		default:
			setting.Args = append(setting.Args, "--" + action)
			setting.Args = append(setting.Args, args)
		}

		cmdsMap[container] = setting
	}

	cmdsList = make(SettingsList, len(cmdsMap))
	var count = 0
	for name, item := range cmdsMap {
		item.Name = name
		cmdsList[count] = item
		count ++
	}
	return

}

