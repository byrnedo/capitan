package main

import (
	"github.com/codeskyblue/go-sh"
	"sort"
	. "github.com/byrnedo/capitan/logger"
	"strings"
	"fmt"
	"io/ioutil"
)

func exists(name string) (bool) {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--format", "{{ .State.Running }}", name).Output()
	if err != nil {
		return false
	}
	if strings.Trim(string(out), " \n") == "<no value>" {
		return false
	}
	return true

}

func isRunning(name string) (bool) {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--format", "{{ .State.Running }}", name).Output()
	if err != nil {
		return false
	}
	if strings.Trim(string(out), " \n") == "true" {
		return true
	}
	return false
}

func runCmd(args ...interface{}) error {
	ses := sh.NewSession()
	Trace.Println(ses.Env)

	//ses.ShowCMD = true
	out, err := ses.Command("docker", args...).Output()
	Trace.Println(string(out))
	if err != nil {
		return err
	}
	return nil
}


func DockerBuild(settings SettingsList) error {
	sort.Sort(settings)
	for _, set := range settings {
		if len(set.Build) == 0 {
			continue
		}
		Info.Println("Building " + set.Name)
		if err := runCmd("build", "--tag", set.Name, set.Build); err != nil {
			return err
		}

	}
	return nil
}



func DockerRun(settings SettingsList) error {
	sort.Sort(settings)
	for _, set := range settings {

		if exists(set.Name){

			if isRunning(set.Name) {
				Info.Println("Already running:", set.Name)
			} else {
				Info.Println("Starting " + set.Name)
				if err := runCmd("start", set.Name); err != nil {
					return err
				}
			}
			continue
		}

		Info.Println("Running " + set.Name)
		cmd := append([]interface{}{"run", "-d", "-t", "--name", set.Name}, set.Args...)
		cmd = append(cmd, set.Image)
		cmd = append(cmd, set.Command...)
		if err := runCmd(cmd...); err != nil {
			return err
		}
	}
	return nil
}

func DockerStart(settings SettingsList) error {
	sort.Sort(settings)
	for _, set := range settings {
		if isRunning(set.Name) {
			Info.Println(set.Name, "already running")
			continue
		}
		Info.Println("Starting " + set.Name)
		if err := runCmd("start", set.Name); err != nil {
			return err
		}
	}
	return nil
}

func DockerRestart(settings SettingsList, secBeforeKill int) error {
	sort.Reverse(settings)
	for _, set := range settings {
		Info.Println("Restarting " + set.Name)
		if err := runCmd("restart", "--time", fmt.Sprintf("%d",secBeforeKill), set.Name); err != nil {
			return err
		}
	}
	return nil
}

func DockerKill(settings SettingsList, signal string) error {
	sort.Reverse(settings)
	for _, set := range settings {
		if !isRunning(set.Name) {
			Info.Println(set.Name, "already dead")
			continue
		}
		Info.Println("Killing " + set.Name)
		if err := runCmd("kill", "--signal", signal, set.Name); err != nil {
			return err
		}
	}
	return nil
}

func DockerStop(settings SettingsList, secBeforeKill int) error {
	sort.Reverse(settings)
	for _, set := range settings {
		if !isRunning(set.Name) {
			Info.Println(set.Name, "already dead")
			continue
		}
		Info.Println("Stopping " + set.Name)
		if err := runCmd("stop", "--time" , fmt.Sprintf("%d",secBeforeKill), set.Name); err != nil {
			return err
		}
	}
	return nil
}

func DockerRm(settings SettingsList, force bool) error {
	sort.Reverse(settings)
	for _, set := range settings {
		var forceStr = "--force=false"
		if force {
			forceStr = "--force=true"
		}

		Info.Println("Removing " + set.Name)
		if err := runCmd("rm", forceStr, set.Name); err != nil {
			return err
		}
	}
	return nil
}
