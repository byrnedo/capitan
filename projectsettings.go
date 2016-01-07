package main

import (
	"fmt"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
)

type ProjectSettings struct {
	ProjectName           string
	ProjectSeparator      string
	ContainerSettingsList SettingsList
}

type SettingsList []Container

func (s SettingsList) Len() int {
	return len(s)
}
func (s SettingsList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SettingsList) Less(i, j int) bool {
	return s[i].Placement < s[j].Placement
}

func (settings *ProjectSettings) LaunchCleanupWatcher() {

	var (
		killBegan     = make(chan bool, 1)
		killDone      = make(chan bool, 1)
		stopDone      = make(chan bool, 1)
		signalChannel = make(chan os.Signal)
	)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	go func() {

		var (
			killing bool
		)

		for {
			select {
			case <-killBegan:
				killing = true
			case <-stopDone:
				if !killing {
					allDone <- true
				}
			case <-killDone:
				allDone <- true
			}
		}
	}()

	go func() {
		var calls int
		for {
			sig := <-signalChannel
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				calls++
				if calls == 1 {
					go func() {
						settings.DockerStop(nil, false)
						stopDone <- true
					}()
				} else if calls == 2 {
					killBegan <- true
					settings.DockerKill(nil, false)
					killDone <- true
				}
			default:
				Debug.Println("Unhandled signal", sig)
			}
		}
		Info.Println("Done cleaning up")
	}()
}

// The 'up' command
//
// Creates a container if it doesn't exist
// Starts a container if stopped
// Recreates a container if the container's image has a newer id locally
// OR if the command used to create the container is now changed (i.e.
// config has changed.
func (settings *ProjectSettings) DockerUp(attach bool, dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)

	wg := sync.WaitGroup{}

	for _, set := range settings.ContainerSettingsList {
		var (
			err error
		)

		//create new
		if !containerExists(set.Name) {
			if err = set.Run(attach, dryRun, &wg); err != nil {
				return err
			}
			continue
		}

		// check image change or args change
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

				if err = set.Run(attach, dryRun, &wg); err != nil {
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

				if err = set.Run(attach, dryRun, &wg); err != nil {
					return err
				}
				continue
			}
		}

		//attach if running
		if isRunning(set.Name) {
			Info.Println("Already running " + set.Name)
			if attach {
				Info.Println("Attaching")
				if err := set.Attach(&wg); err != nil {
					return err
				}
			}
			continue
		}

		Info.Println("Starting " + set.Name)

		if dryRun {
			continue
		}

		//start if stopped
		if err = set.Start(attach, &wg); err != nil {
			return err
		}
		continue

	}
	wg.Wait()
	if attach {
		<-allDone
	}
	return nil
}

// Starts stopped containers
func (settings *ProjectSettings) DockerStart(attach bool, dryRun bool) error {
	sort.Sort(settings.ContainerSettingsList)
	wg := sync.WaitGroup{}
	for _, set := range settings.ContainerSettingsList {
		if isRunning(set.Name) {
			Info.Println("Already running " + set.Name)
			if attach {
				Info.Println("Attaching")
				if err := set.Attach(&wg); err != nil {
					return err
				}
			}
			continue
		}
		Info.Println("Starting " + set.Name)
		if !dryRun {
			if err := set.Start(attach, &wg); err != nil {
				return err
			}
		}
	}
	wg.Wait()
	if attach {
		<-allDone
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

// Print all container IPs
func (settings *ProjectSettings) DockerIP() error {
	sort.Sort(settings.ContainerSettingsList)
	for _, set := range settings.ContainerSettingsList {
		ip := set.IP()
		Info.Printf("%s: %s", set.Name, ip)
	}
	return nil
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
