package container

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
	"github.com/mgutz/str"
	"github.com/byrnedo/capitan/helpers"
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

type Hooks map[string]string

// Runs a hook command if it exists for a specific container
func (h Hooks) Run(hookName string, containerName string) error {
	var (
		hookScript string
		found      bool
		ses        *sh.Session
		argVs      []string
	)
	if hookScript, found = h[hookName]; !found {
		return nil
	}

	ses = sh.NewSession()
	ses.SetEnv("CAPITAN_CONTAINER_NAME", containerName)
	ses.SetEnv("CAPITAN_HOOK_NAME", hookName)

	argVs = str.ToArgv(hookScript)
	if len(argVs) > 1 {
		ses.Command(argVs[0], helpers.ToInterfaceSlice(argVs[1:])...)
	} else {
		ses.Command(argVs[0])
	}
	ses.Stdout = os.Stdout
	ses.Stderr = os.Stderr
	return ses.Run()
}

type Container struct {
	Name        string
	Placement   int
	Args        []string
	Image       string
	Build       string
	Command     []string
	Links       []Link
	Hooks       Hooks
	Action      AppliedAction // used in commands
	UniqueLabel string
}


// Builds an image for a container
func (set *Container) BuildImage() error {
	if err := set.Hooks.Run("before.build", set.Name); err != nil {
		return err
	}
	if _, err := helpers.RunCmd("build", "--tag", set.Name, set.Build); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.build", set.Name); err != nil {
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
	if err := set.Hooks.Run("before.run", set.Name); err != nil {
		return err
	}

	var (
		ses *sh.Session
		err error
	)

	if helpers.GetImageId(set.Image) == "" {
		Warning.Printf("Capitan was unable to find image %s locally\n", set.Image)
		if err = helpers.PullImage(set.Image); err != nil {
			return err
		}
	}

	cmd := set.GetRunArguments()
	uniqueLabel := UniqueLabelName + "=" + fmt.Sprintf("%s", cmd)

	if attach {

		beforeStart := time.Now()

		cmd = append([]interface{}{"run", "-a", "stdout", "-a", "stderr", "-a", "stdin", "--sig-proxy=false", "--label", uniqueLabel}, cmd...)
		if ses, err = set.startLoggedCommand(cmd); err != nil {
			return err
		}
		wg.Add(1)
		go func(name string) {
			ses.Wait()
			wg.Done()
		}(set.Name)

		if !helpers.WasContainerStartedAfterOrRetry(set.Name, beforeStart, 10, 200*time.Millisecond) {
			return errors.New(set.Name + " failed to start")
		}

		Debug.Println("Container deemed to have started after", beforeStart)

		if !helpers.ContainerIsRunning(set.Name) {
			exitCode := helpers.ContainerExitCode(set.Name)
			if exitCode != "0" {
				return errors.New(set.Name + " exited with non-zero exit code " + exitCode)
			}
		}

	} else {
		cmd = append([]interface{}{"run", "-d", "--label", uniqueLabel}, cmd...)
		if err = set.runDaemonCommand(cmd); err != nil {
			return err
		}
	}

	err = set.Hooks.Run("after.run", set.Name)
	return err
}

func (set *Container) runDaemonCommand(cmd []interface{}) error {
	var (
		ses *sh.Session
		err error
	)
	ses = sh.NewSession()
	if logger.GetLevel() == DebugLevel {
		ses.ShowCMD = true
	}
	err = ses.Command("docker", cmd...).Run()
	return err
}

func (set *Container) startLoggedCommand(cmd []interface{}) (*sh.Session, error) {
	ses := sh.NewSession()
	if logger.GetLevel() == DebugLevel {
		ses.ShowCMD = true
	}
	color := nextColor()
	ses.Stdout = NewContainerLogWriter(os.Stdout, set.Name, color)
	ses.Stderr = NewContainerLogWriter(os.Stderr, set.Name, color)

	err := ses.Command("docker", cmd...).Start()

	return ses, err
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

	cmd := append([]interface{}{"--name", set.Name}, helpers.ToInterfaceSlice(set.Args)...)
	cmd = append(cmd, linkArgs...)
	cmd = append(cmd, imageName)
	cmd = append(cmd, helpers.ToInterfaceSlice(set.Command)...)
	return cmd
}

func (set *Container) Attach(wg *sync.WaitGroup) error {
	var (
		err error
		ses *sh.Session
	)
	if ses, err = set.startLoggedCommand(append([]interface{}{"attach", "--sig-proxy=false"}, set.Name)); err != nil {
		return err
	}
	wg.Add(1)

	go func(name string) {
		ses.Wait()
		wg.Done()
	}(set.Name)
	return nil
}

// Start a given container
func (set *Container) Start(attach bool, wg *sync.WaitGroup) error {
	var (
		err error
	)
	set.Action = Start
	if helpers.ContainerIsRunning(set.Name) {
		Info.Println("Already running", set.Name)
		if attach {
			if err = set.Attach(wg); err != nil {
				return err
			}
		}
		return nil
	}

	if err = set.Hooks.Run("before.start", set.Name); err != nil {
		return err
	}

	if err = set.runDaemonCommand(append([]interface{}{"start"}, set.Name)); err != nil {
		return err
	}
	if attach {
		if err = set.Attach(wg); err != nil {
			return err
		}
	}

	if err := set.Hooks.Run("after.start", set.Name); err != nil {
		return err
	}
	return nil
}

// Restart the container
func (set *Container) Restart(args []string) error {
	set.Action = Restart
	if err := set.Hooks.Run("before.start", set.Name); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"restart"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.start", set.Name); err != nil {
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
	if err := set.Hooks.Run("before.kill", set.Name); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"kill"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.kill", set.Name); err != nil {
		return err
	}
	return nil

}

// Stops the container
func (set *Container) Stop(args []string) error {
	set.Action = Stop
	if err := set.Hooks.Run("before.stop", set.Name); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"stop"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.stop", set.Name); err != nil {
		return err
	}
	return nil
}

// Removes the container
func (set *Container) Rm(args []string) error {

	set.Action = Remove
	if err := set.Hooks.Run("before.rm", set.Name); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"rm"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.rm", set.Name); err != nil {
		return err
	}
	return nil
}
