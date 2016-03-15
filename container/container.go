package container

import (
	"errors"
	"fmt"
	. "github.com/byrnedo/capitan/consts"
	"github.com/byrnedo/capitan/helpers"
	"github.com/byrnedo/capitan/logger"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
"strconv"
)

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

type Hook struct {
	Scripts []string
	Ses     *ShellSession
}

type Hooks map[string]*Hook

// Runs a hook command if it exists for a specific container
func (h Hooks) Run(hookName string, ctr *Container) error {
	var (
		hook  *Hook
		found bool
		err   error
	)

	if hook, found = h[hookName]; !found {
		return nil
	}

	for _, script := range hook.Scripts {
		hook.Ses = NewContainerShellSession(ctr)
		hook.Ses.SetEnv("CAPITAN_HOOK_NAME", hookName)

		hook.Ses.Command("bash", "-c", script)

		hook.Ses.Stdout = os.Stdout
		hook.Ses.Stderr = os.Stderr
		hook.Ses.Stdin = os.Stdin

		if err = hook.Ses.Run(); err != nil {
			return err
		}
	}
	return nil
}

type BlueGreenMode int

const (
	BGModeOff BlueGreenMode = iota
	BGModeOn
	BGModeUnknown
)

type Container struct {
	// Container name
	Name string
	// name of service (not including number)
	ServiceName string
	// non unique service id, eg the first col in config, "mongo" or "php"
	ServiceType string
	// the order defined in the config output
	Placement int
	// arguments to container
	ContainerArgs []string
	// image to use
	Image string
	// if supplied will do docker build on this path
	Build string
	// The arguments for the build command
	BuildArgs []string
	// command for container
	Command []string
	// links
	Links []Link
	// volumes from list
	VolumesFrom []string
	// hooks map for this definition
	Hooks Hooks
	// used in commands
	Action AppliedAction
	// the total number of containers to scale to.
	Scale int
	// the arguments for docker run / create
	RunArguments []interface{}
	// the project name
	ProjectName string
	// the project name separator, usually "_"
	ProjectNameSeparator string
	// the number of this container, relates to scale
	InstanceNumber int
	// Rm command given, therefore dont run as daemon
	Remove bool
	// Whether or not Blue/Green mode is enabled for this container
	BlueGreenMode BlueGreenMode
	IsBlueGreen bool
	// The current state of the container
	State *helpers.ServiceState
}

func (set *Container) NewName() {
	set.Name = fmt.Sprintf("%s%s%s%s%d", set.ServiceName, set.ProjectNameSeparator, set.State.Color, set.ProjectNameSeparator, set.InstanceNumber)
}

// Builds an image for a container
func (set *Container) BuildImage() error {
	if err := set.Hooks.Run("before.build", set); err != nil {
		return err
	}

	args := append([]interface{}{
		"build",
	}, helpers.ToInterfaceSlice(set.BuildArgs)...)

	args = append(args, "--tag", set.Image, set.Build)

	if _, err := helpers.RunCmd(args...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.build", set); err != nil {
		return err
	}
	return nil
}

func (set *Container) launchWithRmInForeground(cmd []interface{}) error {
	var (
		ses *ShellSession
		err error
	)

	initialArgs := []interface{}{
		"run",
		"-a", "stdout",
		"-a", "stderr",
		"-a", "stdin",
		"--sig-proxy=false",
		"--rm",
	}

	cmd = append(initialArgs, cmd...)
	if ses, err = set.startLoggedCommand(cmd); err != nil {
		return err
	}

	err = ses.Wait()
	if err != nil {
		return errors.New(set.Name + " exited with error: " + err.Error())
	}

	return nil
}

func (set *Container) launchInForeground(cmd []interface{}, wg *sync.WaitGroup) error {

	var (
		ses *ShellSession
		err error
	)

	beforeStart := time.Now()

	initialArgs := []interface{}{
		"run",
		"-a", "stdout",
		"-a", "stderr",
		"-a", "stdin",
		"--sig-proxy=false",
	}

	if set.Remove {
		initialArgs = append(initialArgs, "--rm")
	}

	cmd = append(initialArgs, cmd...)
	if ses, err = set.startLoggedCommand(cmd); err != nil {
		return err
	}
	wg.Add(1)
	go func(name string) {
		ses.Wait()
		wg.Done()
	}(set.Name)



	if !helpers.WasContainerStartedAfterOrRetry(set.Name, beforeStart, 10, 200 * time.Millisecond) {
		return errors.New(set.Name + " failed to start")
	}

	Debug.Println("Container deemed to have started after", beforeStart)
	if !helpers.ContainerIsRunning(set.Name) {
		exitCode := helpers.ContainerExitCode(set.Name)
		if exitCode != "0" {
			return errors.New(set.Name + " exited with non-zero exit code " + exitCode)
		}
	}

	return nil

}

func (set *Container) BlueGreenCopy() (newCon *Container) {
	// rename the current
	// if id then we have a color,
	var newColor = "blue"
	Debug.Println(set.State)
	if set.State.Color == "blue" {
		newColor = "green"
	}

	newCon = new(Container)
	*newCon = *set
	newCon.State.Color = newColor
	newCon.NewName()
	return


}

func (set *Container) BlueGreenDeploy(attach bool, dryRun bool, wg *sync.WaitGroup) error {

	newCon := set.BlueGreenCopy()

	if err := newCon.Run(attach, dryRun, wg); err != nil {
		// put back the old
		Warning.Println("Error running new container, killing...")
		newCon.Kill(nil)
		return  err
	}

	// shutdown the old
	ContainerInfoLog(newCon.Name, "Removing old container "+set.Name+"...")
	if ! dryRun {
		if err := set.Rm([]string{"-f"}); err != nil {
			Error.Println("Error stopping old container")
			return err
		}
	}

	return nil
}

func (set *Container) RecreateAndRun(attach bool, dryRun bool, wg *sync.WaitGroup) error {
	if !dryRun {
		set.Rm([]string{"-f"})
	}

	if err := set.Run(attach, dryRun, wg); err != nil {
		return err
	}
	return nil
}

func createCapitanContainerLabels(ctr *Container, args []interface{}) []interface{} {
	return []interface{}{
		"--label",
		UniqueLabelName + "=" + helpers.HashInterfaceSlice(args),
		"--label",
		ServiceLabelName + "=" + ctr.ServiceName,
		"--label",
		ServiceLabelType + "=" + ctr.ServiceType,
		"--label",
		ProjectLabelName + "=" + ctr.ProjectName,
		"--label",
		ContainerNumberLabelName + "=" + strconv.Itoa(ctr.InstanceNumber),
		"--label",
		ColorLabelName + "=" + ctr.State.Color,
	}
}

// Run a container
func (set *Container) Create(dryRun bool) error {
	set.Action = Run

	ContainerInfoLog(set.Name, "Creating...")
	if dryRun {
		return nil
	}
	if err := set.Hooks.Run("before.create", set); err != nil {
		return err
	}

	cmd := set.GetRunArguments()
	labels := createCapitanContainerLabels(set, cmd)
	cmd = append(labels, cmd...)

	cmd = append([]interface{}{"create"}, cmd...)
	if err := set.launchDaemonCommand(cmd); err != nil {
		return err
	}

	return set.Hooks.Run("after.create", set)
}

// Run a container
func (set *Container) Run(attach bool, dryRun bool, wg *sync.WaitGroup) error {
	set.Action = Run

	ContainerInfoLog(set.Name,"Running...")
	if dryRun {
		return nil
	}
	if err := set.Hooks.Run("before.run", set); err != nil {
		return err
	}

	cmd := set.GetRunArguments()
	labels := createCapitanContainerLabels(set, cmd)
	cmd = append(labels, cmd...)

	if set.Remove {
		if err := set.launchWithRmInForeground(cmd); err != nil {
			return err
		}
	} else if attach {
		if err := set.launchInForeground(cmd, wg); err != nil {
			return err
		}
	} else {
		cmd = append([]interface{}{"run", "-d"}, cmd...)
		if err := set.launchDaemonCommand(cmd); err != nil {
			return err
		}
	}

	return set.Hooks.Run("after.run", set)
}

func (set *Container) launchDaemonCommand(cmd []interface{}) error {
	var (
		ses *ShellSession
		err error
	)
	ses = NewContainerShellSession(set)

	concStr := "docker "
	for _, arg := range cmd {
		concStr += fmt.Sprintf("%s", arg) + " "
	}
	concStr = strings.Trim(concStr, " ")

	err = ses.Command("bash", "-c", concStr).Run()
	return err
}

func (set *Container) startLoggedCommand(cmd []interface{}) (*ShellSession, error) {
	ses := NewContainerShellSession(set)

	color := nextColor()
	ses.Stdout = NewContainerLogWriter(os.Stdout, set.Name, color)
	ses.Stderr = NewContainerLogWriter(os.Stderr, set.Name, color)

	concStr := "docker "
	for _, arg := range cmd {
		concStr += fmt.Sprintf("%s", arg) + " "
	}
	concStr = strings.Trim(concStr, " ")

	err := ses.Command("bash", "-c", concStr).Start()

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

	var volumesFromArgs = make([]interface{}, 0, len(set.VolumesFrom)*2)
	for _, vol := range set.VolumesFrom {
		volumesFromArgs = append(volumesFromArgs, "--volumes-from", vol)
	}

	cmd := append([]interface{}{"--name", set.Name}, helpers.ToInterfaceSlice(set.ContainerArgs)...)
	cmd = append(cmd, linkArgs...)
	cmd = append(cmd, volumesFromArgs...)
	cmd = append(cmd, imageName)
	cmd = append(cmd, helpers.ToInterfaceSlice(set.Command)...)
	return cmd
}

func (set *Container) Attach(wg *sync.WaitGroup) error {
	var (
		err error
		ses *ShellSession
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
// TODO needs to respect scale
func (set *Container) Start(attach bool, wg *sync.WaitGroup) error {
	var (
		err error
	)
	set.Action = Start
	if set.State.Running {
		ContainerInfoLog(set.Name, "Already running.")
		if attach {
			if err = set.Attach(wg); err != nil {
				return err
			}
		}
		return nil
	}

	if err = set.Hooks.Run("before.start", set); err != nil {
		return err
	}

	if err = set.launchDaemonCommand(append([]interface{}{"start"}, set.Name)); err != nil {
		return err
	}
	if attach {
		if err = set.Attach(wg); err != nil {
			return err
		}
	}

	if err := set.Hooks.Run("after.start", set); err != nil {
		return err
	}
	return nil
}

// Restart the container
// TODO needs to respect scale
func (set *Container) Restart(args []string) error {
	set.Action = Restart
	if err := set.Hooks.Run("before.start", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"restart"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.start", set); err != nil {
		return err
	}
	return nil
}

// Returns a containers IP
// TODO needs to respect scale
func (set *Container) IPs() string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{range $i, $p := .NetworkSettings.Networks}}{{$p.IPAddress}}@{{$i}},{{end}}", set.Name).Output()
	if err != nil {
		return ""
	}
	ip := strings.Trim(string(out), ",\n")
	return ip
}

// Start streaming a container's logs
// TODO needs to respect scale
func (set *Container) Logs() (*sh.Session, error) {
	color := nextColor()
	ses := sh.NewSession()

	if logger.GetLevel() == DebugLevel {
		ses.ShowCMD = true
	}
	ses.Command("docker", "logs", "--tail", "10", "-f", set.Name)

	ses.Stdout = NewContainerLogWriter(os.Stdout, set.Name, color)
	ses.Stderr = NewContainerLogWriter(os.Stderr, set.Name, color)

	err := ses.Start()
	return ses, err
}

// Kills the container
// TODO needs to respect scale
func (set *Container) Kill(args []string) error {
	set.Action = Kill
	if err := set.Hooks.Run("before.kill", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"kill"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.kill", set); err != nil {
		return err
	}
	return nil

}

// Stops the container
// TODO needs to respect scale
func (set *Container) Stop(args []string) error {
	set.Action = Stop
	if err := set.Hooks.Run("before.stop", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"stop"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.stop", set); err != nil {
		return err
	}
	return nil
}

// Removes the container
// TODO needs to respect scale
func (set *Container) Rm(args []string) error {

	set.Action = Remove
	if err := set.Hooks.Run("before.rm", set); err != nil {
		return err
	}
	args = append(args, set.Name)
	if _, err := helpers.RunCmd(append([]interface{}{"rm"}, helpers.ToInterfaceSlice(args)...)...); err != nil {
		return err
	}
	if err := set.Hooks.Run("after.rm", set); err != nil {
		return err
	}
	return nil
}
