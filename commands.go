package main

import (
	"fmt"
	"github.com/byrnedo/capitan/logger"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"github.com/mgutz/str"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
)

const UniqueLabelName = "capitanRunCmd"

var colorList = []string{
	"white",
	"red",
	"green",
	"yellow",
	"blue",
	"magenta",
	"cyan",
}

var nextColorIndex = rand.Intn(len(colorList) - 1)

//Get the id for a given image name
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

// Get the value of the label used to record the run
// arguments used when creating the container
func getContainerUniqueLabel(name string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.Config.Labels."+UniqueLabelName+"}}", name).Output()
	if err != nil {
		return ""
	}
	label := strings.Trim(string(out), " \n")
	return label

}

// Get the image id for a given container
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

// Checks if a container exists
func containerExists(name string) bool {
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

// Check if a container is running
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

// Helper to run a docker command
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

// The build command
func (settings *ProjectSettings) CapitanBuild(dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		if len(set.Build) == 0 {
			continue
		}
		Info.Println("Building " + set.Name)
		if !dryRun {
			if err := set.BuildImage(); err != nil {
				return err
			}
		}

	}
	return nil
}

// Builds an image for a container
func (set *ContainerSettings) BuildImage() error {
	if err := runHook("before.build", set); err != nil {
		return err
	}
	if _, err := runCmd("build", "--tag", set.Name, set.Build); err != nil {
		return err
	}
	if err := runHook("after.build", set); err != nil {
		return err
	}
	return nil
}

// The 'up' command
//
// Creates a container if it doesn't exist
// Starts a container if stopped
// Recreates a container if the container's image has a newer id locally
// OR if the command used to create the container is now changed (i.e.
// config has changed.
func (settings *ProjectSettings) DockerUp(dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)

	for _, set := range settings.ContainerSettingsList {

		if !containerExists(set.Name) {
			if err := set.Run(dryRun); err != nil {
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
					if err := set.Rm([]string{"-f"}); err != nil {
						return err
					}
				}

				if err := set.Run(dryRun); err != nil {
					return err
				}
				continue
			}
			uniqueLabel := fmt.Sprintf("%s", set.GetRunArguments())
			if getContainerUniqueLabel(set.Name) != uniqueLabel {
				// remove and restart
				Info.Println("Removing (run arguments changed):", set.Name)
				if !dryRun {
					if err := set.Rm([]string{"-f"}); err != nil {
						return err
					}
				}

				if err := set.Run(dryRun); err != nil {
					return err
				}
				continue
			}
		}

		if isRunning(set.Name) {
			Info.Println("Already running:", set.Name)
		} else {
			Info.Println("Starting " + set.Name)
			if dryRun {
				continue
			}
			if err := set.Start(nil); err != nil {
				return err
			}
		}
		continue

	}
	return nil
}

// Run a container
func (set *ContainerSettings) Run(dryRun bool) error {

	Info.Println("Running " + set.Name)
	if dryRun {
		return nil
	}
	if err := runHook("before.run", set); err != nil {
		return err
	}

	cmd := set.GetRunArguments()
	uniqueLabel := UniqueLabelName + "=" + fmt.Sprintf("%s", cmd)
	if _, err := runCmd(append([]interface{}{"run", "-d", "-t", "--label", uniqueLabel}, cmd...)...); err != nil {
		return err
	}

	if err := runHook("after.run", set); err != nil {
		return err
	}

	return nil
}

// Create docker arg slice from container options
func (set *ContainerSettings) GetRunArguments() []interface{} {
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

	cmd := append([]interface{}{"--name", set.Name}, toInterfaceSlice(set.Args)...)
	cmd = append(cmd, linkArgs...)
	cmd = append(cmd, imageName)
	cmd = append(cmd, toInterfaceSlice(set.Command)...)
	return cmd
}

// Starts stopped containers
func (settings *ProjectSettings) DockerStart(dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		if isRunning(set.Name) {
			Info.Println("Already running:", set.Name)
			continue
		}
		Info.Println("Starting " + set.Name)
		if !dryRun {
			if err := set.Start(nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// Start a given container
func (set *ContainerSettings) Start(args []string) error {
	if err := runHook("before.start", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := runCmd(append([]interface{}{"start"}, toInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := runHook("after.start", set); err != nil {
		return err
	}
	return nil
}

// Command to restart all containers
func (settings *ProjectSettings) DockerRestart(args []string, dryRun bool) error {
	sort.Sort(sort.Reverse(settings.ContainerSettingsList))
	for _, set := range settings.ContainerSettingsList {
		Info.Println("Restarting " + set.Name)
		if !dryRun {
			if err := set.Restart(args); err != nil {
				return err
			}
		}
	}
	return nil
}

// Restart the container
func (set *ContainerSettings) Restart(args []string) error {
	if err := runHook("before.start", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := runCmd(append([]interface{}{"restart"}, toInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := runHook("after.start", set); err != nil {
		return err
	}
	return nil
}

// Print all container IPs
func (settings *ProjectSettings) DockerIP() error {
	sort.Sort(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		ip := set.IP()
		Info.Printf("%s: %s", set.Name, ip)
	}
	return nil
}

// Returns a containers IP
func (set *ContainerSettings) IP() string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.NetworkSettings.IPAddress}}", set.Name).Output()
	if err != nil {
		return ""
	}
	ip := strings.Trim(string(out), " \n")
	return ip
}

// Get the next color to be used in log output
func nextColor() string {
	defer func() {
		nextColorIndex++
		if nextColorIndex >= len(colorList) {
			nextColorIndex = 0
		}
	}()
	return colorList[nextColorIndex]
}

// Stream all container logs
func (settings *ProjectSettings) DockerLogs() error {
	sort.Sort(settings.ContainerSettingsList)
	var wg sync.WaitGroup
	for _, set := range settings.ContainerSettingsList {
		var (
			ses *sh.Session
			err error
		)
		if ses, err = set.Logs(); err != nil {
			Error.Println("Error getting log for " + set.Name + ": " + err.Error())
			continue
		}

		wg.Add(1)

		go func() {
			ses.Wait()
			wg.Done()
		}()

	}
	wg.Wait()
	return nil
}

// Start streaming a container's logs
func (set *ContainerSettings) Logs() (*sh.Session, error) {
	color := nextColor()
	ses := sh.NewSession()
	ses.Command("docker", "logs", "--tail", "10", "-f", set.Name)

	ses.Stdout = NewContainerLogWriter(os.Stdout, set.Name, color)
	ses.Stderr = NewContainerLogWriter(os.Stderr, set.Name, color)

	err := ses.Start()
	return ses, err
}

// Stream all container stats
func (settings *ProjectSettings) DockerStats() error {
	var (
		args []interface{}
	)
	sort.Sort(settings.ContainerSettingsList)

	args = make([]interface{}, len(settings.ContainerSettingsList))

	for i, set := range settings.ContainerSettingsList {
		args[i] = set.Name
	}

	ses := sh.NewSession()
	ses.Command("docker", append([]interface{}{"stats"}, args...)...)
	ses.Start()
	ses.Wait()
	return nil
}

// Print `docker ps` ouptut for all containers in project
func (settings *ProjectSettings) DockerPs(args []string) error {
	sort.Sort(settings.ContainerSettingsList)
	allArgs := append([]interface{}{"ps"}, toInterfaceSlice(args)...)
	for _, set := range settings.ContainerSettingsList {
		allArgs = append(allArgs, "-f", "name="+set.Name)
	}
	var (
		err error
		out []byte
	)
	if out, err = runCmd(allArgs...); err != nil {
		return err
	}
	Info.Print(string(out))
	return nil
}

// Kill all running containers in project
func (settings *ProjectSettings) DockerKill(args []string, dryRun bool) error {
	sort.Sort(sort.Reverse(settings.ContainerSettingsList))
	for _, set := range settings.ContainerSettingsList {
		if !isRunning(set.Name) {
			Info.Println("Already dead:", set.Name)
			continue
		}
		Info.Println("Killing " + set.Name)
		if !dryRun {
			if err := set.Kill(args); err != nil {
				return err
			}
		}
	}
	return nil
}

// Kills the container
func (set *ContainerSettings) Kill(args []string) error {
	if err := runHook("before.kill", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := runCmd(append([]interface{}{"kill"}, toInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := runHook("after.kill", set); err != nil {
		return err
	}
	return nil

}

// Stops the containers in the project
func (settings *ProjectSettings) DockerStop(args []string, dryRun bool) error {
	sort.Sort(sort.Reverse(settings.ContainerSettingsList))
	for _, set := range settings.ContainerSettingsList {
		if !isRunning(set.Name) {
			Info.Println("Already dead:", set.Name)
			continue
		}
		Info.Println("Stopping " + set.Name)
		if !dryRun {
			if err := set.Stop(args); err != nil {
				return err
			}
		}
	}
	return nil
}

// Stops the container
func (set *ContainerSettings) Stop(args []string) error {

	if err := runHook("before.stop", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := runCmd(append([]interface{}{"kill"}, toInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := runHook("after.stop", set); err != nil {
		return err
	}
	return nil
}

// Remove all containers in project
func (settings *ProjectSettings) DockerRm(args []string, dryRun bool) error {
	sort.Sort(sort.Reverse(settings.ContainerSettingsList))
	for _, set := range settings.ContainerSettingsList {

		if !dryRun && containerExists(set.Name) {
			Info.Println("Removing " + set.Name)
			if err := set.Rm(args); err != nil {
				return err
			}
		} else {
			Info.Println("Container doesn't exist:", set.Name)
		}
	}
	return nil
}

// Removes the container
func (set *ContainerSettings) Rm(args []string) error {

	if err := runHook("before.rm", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := runCmd(append([]interface{}{"rm"}, toInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := runHook("after.rm", set); err != nil {
		return err
	}
	return nil
}

// Runs a hook command if it exists for a specific container
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
