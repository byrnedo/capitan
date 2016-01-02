package main

import (
	"fmt"
	"github.com/byrnedo/capitan/logger"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"github.com/mgutz/str"
	"io/ioutil"
	"os"
	"sort"
	"strings"
)

const UniqueLabelName = "capitanRunCmd"

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

func getContainerIPAddress(name string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.NetworkSettings.IPAddress}}", name).Output()
	if err != nil {
		return ""
	}
	ip := strings.Trim(string(out), " \n")
	return ip

}

func getContainerUniqueLabel(name string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.Config.Labels." + UniqueLabelName + "}}", name).Output()
	if err != nil {
		return ""
	}
	label := strings.Trim(string(out), " \n")
	return label

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
			if err := runHook("before.build", &set); err != nil {
				return err
			}
			if _, err := runCmd("build", "--tag", set.Name, set.Build); err != nil {
				return err
			}
			if err := runHook("after.build", &set); err != nil {
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
			if err := runContainer(&set, dryRun); err != nil {
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

				if err := runContainer(&set, dryRun); err != nil {
					return err
				}
				continue
			}
			uniqueLabel := fmt.Sprintf("%s", getRunArguments(&set))
			if getContainerUniqueLabel(set.Name) != uniqueLabel {
				// remove and restart
				Info.Println("Removing (run arguments changed):", set.Name)
				if !dryRun {
					if _, err := runCmd("rm", "-f", set.Name); err != nil {
						return err
					}
				}

				if err := runContainer(&set, dryRun); err != nil {
					return err
				}
				continue
			}
		}

		if isRunning(set.Name) {
			Info.Println("Already running:", set.Name)
		} else {
			Info.Println("Starting " + set.Name)
			if err := runHook("before.start", &set); err != nil {
				return err
			}
			if _, err := runCmd("start", set.Name); err != nil {
				return err
			}
			if err := runHook("after.start", &set); err != nil {
				return err
			}
		}
		continue

	}
	return nil
}

func runContainer(set *ContainerSettings, dryRun bool) error {

	Info.Println("Running " + set.Name)
	if dryRun {
		return nil
	}
	if err := runHook("before.run", set); err != nil {
		return err
	}

	cmd := getRunArguments(set)
	uniqueLabel := UniqueLabelName + "=" + fmt.Sprintf("%s", cmd)
	if _, err := runCmd(append([]interface{}{"run", "-d", "-t", "--label", uniqueLabel}, cmd...)...); err != nil {
		return err
	}

	if err := runHook("after.run", set); err != nil {
		return err
	}

	return nil
}

func getRunArguments(set *ContainerSettings) []interface{} {
	imageName := set.Name
	if len(set.Image) > 0 {
		imageName = set.Image
	}

	var linkArgs = make([]interface{}, 0, len(set.Links)*2)
	for _, link := range set.Links {
		linkStr := link.Container
		if link.Alias != "" {
			linkStr += ":" + link.Alias
		}
		linkArgs = append(linkArgs, "--link", linkStr)
	}

	cmd := append([]interface{}{ "--name", set.Name}, toInterfaceSlice(set.Args)...)
	cmd = append(cmd, linkArgs...)
	cmd = append(cmd, imageName)
	cmd = append(cmd, toInterfaceSlice(set.Command)...)
	return cmd
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
			if err := runHook("before.start", &set); err != nil {
				return err
			}
			if _, err := runCmd("start", set.Name); err != nil {
				return err
			}
			if err := runHook("after.start", &set); err != nil {
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
			if err := runHook("before.restart", &set); err != nil {
				return err
			}
			if _, err := runCmd("restart", "--time", fmt.Sprintf("%d", secBeforeKill), set.Name); err != nil {
				return err
			}
			if err := runHook("after.restart", &set); err != nil {
				return err
			}
		}
	}
	return nil
}

func DockerIp(settings *ProjectSettings) error {
	sort.Sort(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		ip := getContainerIPAddress(set.Name)
		Info.Printf("%s: %s", set.Name, ip)
	}
	return nil
}

func DockerPs(settings *ProjectSettings) error {
	sort.Sort(settings.ContainerSettingsList)
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
			if err := runHook("before.kill", &set); err != nil {
				return err
			}
			if _, err := runCmd("kill", "--signal", signal, set.Name); err != nil {
				return err
			}
			if err := runHook("after.kill", &set); err != nil {
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
			if err := runHook("before.stop", &set); err != nil {
				return err
			}
			if _, err := runCmd("stop", "--time", fmt.Sprintf("%d", secBeforeKill), set.Name); err != nil {
				return err
			}
			if err := runHook("after.stop", &set); err != nil {
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
		if !dryRun && exists(set.Name) {
			if err := runHook("before.rm", &set); err != nil {
				return err
			}
			if _, err := runCmd("rm", forceStr, set.Name); err != nil {
				return err
			}
			if err := runHook("after.rm", &set); err != nil {
				return err
			}
		}
	}
	return nil
}

func runHook(hookName string, settings *ContainerSettings) error {
	var (
		hookScript string
		found      bool
		ses        *sh.Session
		argVs      []string
	)
	if hookScript, found = settings.Hooks[hookName]; !found {
		return nil
	}

	Info.Print("Executing hook: " + hookName + "\n")

	ses = sh.NewSession()
	ses.SetEnv("CAPITAN_CONTAINER_NAME", settings.Name)
	ses.SetEnv("CAPITAN_HOOK_NAME", hookName)

	argVs = str.ToArgv(hookScript)
	if len(argVs) > 1 {
		ses.Command(argVs[0], toInterfaceSlice(argVs[1:])...)
	} else {
		ses.Command(argVs[0])
	}
	ses.Stdout = os.Stdout
	ses.Stderr = os.Stderr
	return ses.Run()
}

func toStringSlice(data []interface{}) (out []string) {
	out = make([]string, len(data))
	for i, item := range data {
		out[i] = fmt.Sprintf("%s", item)
	}
	return
}

func toInterfaceSlice(data []string) (out []interface{}) {
	out = make([]interface{}, len(data))
	for i, item := range data {
		out[i] = item
	}
	return
}
