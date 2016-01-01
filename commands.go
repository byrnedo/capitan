package main

import (
	"fmt"
	"github.com/byrnedo/capitan/logger"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"io/ioutil"
	"sort"
	"strings"
)

func getImageId(imageName string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "image", "--format", "{{.Id}}", imageName).Output()
	if err != nil {
		return ""
	}
	imageId := strings.Trim(string(out), " \n")
	return imageId
}

func getContainerImageId(name string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.Image}}", name).Output()
	if err != nil {
		return ""
	}
	imageId := strings.Trim(string(out), " \n")
	return imageId

}

func exists(name string) bool {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--format", "{{.State.Running}}", name).Output()
	if err != nil {
		return false
	}
	if strings.Trim(string(out), " \n") == "<no value>" {
		return false
	}
	return true

}

func isRunning(name string) bool {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--format", "{{.State.Running}}", name).Output()
	if err != nil {
		return false
	}
	if strings.Trim(string(out), " \n") == "true" {
		return true
	}
	return false
}

func runCmd(args ...interface{}) ([]byte, error) {
	ses := sh.NewSession()

	if logger.GetLevel() == DebugLevel {
		ses.ShowCMD = true
	}

	out, err := ses.Command("docker", args...).Output()
	Debug.Println(string(out))
	if err != nil {
		return out, err
	}
	return out, nil
}

func DockerBuild(settings *ProjectSettings, dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		if len(set.Build) == 0 {
			continue
		}
		Info.Println("Building " + set.Name)
		if !dryRun {
			if _, err := runCmd("build", "--tag", set.Name, set.Build); err != nil {
				return err
			}
		}

	}
	return nil
}

func DockerUp(settings *ProjectSettings, dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)

	for _, set := range settings.ContainerSettingsList {

		if !exists(set.Name) {
			if err := runContainer(set.Name, set.Image, set.Args, set.Command, dryRun); err != nil {
				return err
			}
			continue
		}

		if set.Image != "" {
			conImage := getContainerImageId(set.Name)
			localImage := getImageId(set.Image)
			if conImage != "" && localImage != "" && conImage != localImage {
				// remove and restart
				Info.Println("Removing (different image available):", set.Name)
				if !dryRun {
					if _, err := runCmd("rm", "-f", set.Name); err != nil {
						return err
					}
				}
				if err := runContainer(set.Name, set.Image, set.Args, set.Command, dryRun); err != nil {
					return err
				}
				continue
			}
		}

		if isRunning(set.Name) {
			Info.Println("Already running:", set.Name)
		} else {
			Info.Println("Starting " + set.Name)
			if _, err := runCmd("start", set.Name); err != nil {
				return err
			}
		}
		continue

	}
	return nil
}

func runContainer(name string, image string, args []interface{}, command []interface{}, dryRun bool) error {
	Info.Println("Running " + name)
	if dryRun {
		return nil
	}
	cmd := append([]interface{}{"run", "-d", "-t", "--name", name}, args...)
	cmd = append(cmd, image)
	cmd = append(cmd, command...)
	_, err := runCmd(cmd...)
	return err
}

func DockerStart(settings *ProjectSettings, dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		if isRunning(set.Name) {
			Info.Println(set.Name, "already running")
			continue
		}
		Info.Println("Starting " + set.Name)
		if !dryRun {
			if _, err := runCmd("start", set.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func DockerRestart(settings *ProjectSettings, secBeforeKill int, dryRun bool) error {
	sort.Reverse(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		Info.Println("Restarting " + set.Name)
		if !dryRun {
			if _, err := runCmd("restart", "--time", fmt.Sprintf("%d", secBeforeKill), set.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func DockerPs(settings *ProjectSettings) error {
	sort.Reverse(settings.ContainerSettingsList)
	nameFilter := make([]string, 0)
	for _, set := range settings.ContainerSettingsList {
		nameFilter = append(nameFilter, "-f", "name="+set.Name)
	}
	var (
		err error
		out []byte
	)
	if out, err = runCmd("ps", "-a", nameFilter); err != nil {
		return err
	}
	Info.Print(string(out))
	return nil
}

func DockerKill(settings *ProjectSettings, signal string, dryRun bool) error {
	sort.Reverse(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		if !isRunning(set.Name) {
			Info.Println(set.Name, "already dead")
			continue
		}
		Info.Println("Killing " + set.Name)
		if !dryRun {
			if _, err := runCmd("kill", "--signal", signal, set.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func DockerStop(settings *ProjectSettings, secBeforeKill int, dryRun bool) error {
	sort.Reverse(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		if !isRunning(set.Name) {
			Info.Println(set.Name, "already dead")
			continue
		}
		Info.Println("Stopping " + set.Name)
		if !dryRun {
			if _, err := runCmd("stop", "--time", fmt.Sprintf("%d", secBeforeKill), set.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func DockerRm(settings *ProjectSettings, force bool, dryRun bool) error {
	sort.Reverse(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		var forceStr = "--force=false"
		if force {
			forceStr = "--force=true"
		}

		Info.Println("Removing " + set.Name)
		if !dryRun {
			if _, err := runCmd("rm", forceStr, set.Name); err != nil {
				return err
			}
		}
	}
	return nil
}
