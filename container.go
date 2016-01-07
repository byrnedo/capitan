package main

import (
	"errors"
	"fmt"
	"github.com/byrnedo/capitan/logger"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

const UniqueLabelName = "capitanRunCmd"

var (
	colorList = []string{
		"white",
		"red",
		"green",
		"yellow",
		"blue",
		"magenta",
		"cyan",
	}

	nextColorIndex = rand.Intn(len(colorList) - 1)
	allDone        = make(chan bool, 1)
)

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

type Link struct {
	Container string
	Alias     string
}

type AppliedAction string

const (
	Run     AppliedAction = "run"
	Start   AppliedAction = "start"
	Stop    AppliedAction = "stop"
	Kill    AppliedAction = "kill"
	Restart AppliedAction = "restart"
	Remove  AppliedAction = "remove"
)

type Container struct {
	Name        string
	Placement   int
	Args        []string
	Image       string
	Build       string
	Command     []string
	Links       []Link
	Hooks       map[string]string
	Action      AppliedAction // used in commands
	UniqueLabel string
}

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
//pull the image for a given image name
func pullImage(imageName string) error {
	ses := sh.NewSession()
	err := ses.Command("docker", "pull", imageName).Run()
	return err
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

func isRunningOrRetry(name string, maxAttempts int, interval time.Duration) bool {
	attempts := 0
	for {
		attempts++
		if isRunning(name) {
			return true
		}

		if attempts >= maxAttempts {
			break
		} else {
			time.Sleep(interval)
		}
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
		return out, errors.New("Error running docker command:" + err.Error())
	}
	return out, nil
}

// Builds an image for a container
func (set *Container) BuildImage() error {
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

// Run a container
func (set *Container) Run(attach bool, dryRun bool, wg *sync.WaitGroup) error {
	set.Action = Run

	Info.Println("Running " + set.Name)
	if dryRun {
		return nil
	}
	if err := runHook("before.run", set); err != nil {
		return err
	}

	var (
		ses *sh.Session
		err error
	)

	if getImageId(set.Image) == "" {
		Warning.Printf("Capitan was unable to find image %s locally\n", set.Image)
		if err = pullImage(set.Image); err != nil {
			return err
		}
	}

	cmd := set.GetRunArguments()
	uniqueLabel := UniqueLabelName + "=" + fmt.Sprintf("%s", cmd)


	if attach {
		cmd = append([]interface{}{"run", "-a", "stdout", "-a", "stderr", "-a", "stdin", "--sig-proxy=false", "--label", uniqueLabel}, cmd...)
		if ses, err = set.startForegroundCommand(cmd); err != nil {
			return err
		}
		wg.Add(1)
		go func(name string) {
			ses.Wait()
			wg.Done()
		}(set.Name)
	} else {
		cmd = append([]interface{}{"run", "-d", "--label", uniqueLabel}, cmd...)
		if ses, err = set.startDaemonCommand(cmd); err != nil {
			return err
		}
		err = ses.Wait()
		if err != nil {
			Error.Println(set.Name, "returned error:", err.Error())
		}
	}

	err = runHook("after.run", set)
	return err
}

func (set *Container) startDaemonCommand(cmd []interface{}) (*sh.Session, error) {
	var (
		ses *sh.Session
		err error
	)
	ses = sh.NewSession()
	if logger.GetLevel() == DebugLevel {
		ses.ShowCMD = true
	}
	err = ses.Command("docker", cmd...).Start()
	return ses, err
}

func (set *Container) startForegroundCommand(cmd []interface{}) (*sh.Session, error) {
	ses := sh.NewSession()
	if logger.GetLevel() == DebugLevel {
		ses.ShowCMD = true
	}
	color := nextColor()
	ses.Stdout = NewContainerLogWriter(os.Stdout, set.Name, color)
	ses.Stderr = NewContainerLogWriter(os.Stderr, set.Name, color)

	if err := ses.Command("docker", cmd...).Start(); err != nil {
		return ses, err
	}

	if !isRunningOrRetry(set.Name, 10, 300*time.Millisecond) {
		return ses, errors.New(set.Name + " failed to start")
	}

	return ses, nil
}

// Create docker arg slice from container options
func (set *Container) GetRunArguments() []interface{} {
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

// Start a given container
func (set *Container) Start(attach bool, wg *sync.WaitGroup) error {
	var (
		ses *sh.Session
		err error
	)
	set.Action = Start
	if err = runHook("before.start", set); err != nil {
		return err
	}
	if attach {
		if ses, err = set.startDaemonCommand(append([]interface{}{"start"}, set.Name)); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
		if ses, err = set.startForegroundCommand(append([]interface{}{"attach", "--sig-proxy=false"}, set.Name)); err != nil {
			return err
		}
		wg.Add(1)
		go func(name string) {
			ses.Wait()
			wg.Done()
		}(set.Name)
	} else {
		if ses, err = set.startDaemonCommand(append([]interface{}{"start"}, set.Name)); err != nil {
			return err
		}
		if err = ses.Wait(); err != nil {
			return errors.New("Exit error for " + set.Name + ":" + err.Error())
		}
	}

	if err := runHook("after.start", set); err != nil {
		return err
	}
	return nil
}

// Restart the container
func (set *Container) Restart(args []string) error {
	set.Action = Restart
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

// Returns a containers IP
func (set *Container) IP() string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.NetworkSettings.IPAddress}}", set.Name).Output()
	if err != nil {
		return ""
	}
	ip := strings.Trim(string(out), " \n")
	return ip
}

// Start streaming a container's logs
func (set *Container) Logs() (*sh.Session, error) {
	color := nextColor()
	ses := sh.NewSession()
	ses.Command("docker", "logs", "--tail", "10", "-f", set.Name)

	ses.Stdout = NewContainerLogWriter(os.Stdout, set.Name, color)
	ses.Stderr = NewContainerLogWriter(os.Stderr, set.Name, color)

	err := ses.Start()
	return ses, err
}

// Kills the container
func (set *Container) Kill(args []string) error {
	set.Action = Kill
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

// Stops the container
func (set *Container) Stop(args []string) error {
	set.Action = Stop
	if err := runHook("before.stop", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := runCmd(append([]interface{}{"stop"}, toInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := runHook("after.stop", set); err != nil {
		return err
	}
	return nil
}

// Removes the container
func (set *Container) Rm(args []string) error {

	set.Action = Remove
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
