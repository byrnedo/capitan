package main

import (
	"fmt"
	"github.com/byrnedo/capitan/consts"
	"github.com/byrnedo/capitan/container"
	"github.com/byrnedo/capitan/helpers"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"text/template"
)

const containerShowTemplate = `{{.Name}}:
  Name:  {{.ServiceName}}
  State:
    ID: {{.State.ID}}
    Color: {{.State.Color}}
    Running: {{.State.Running}}
    Hash: {{.State.ArgsHash}}
  Type:  {{.ServiceType}}
  Image: {{.Image}}{{if .Build}}
  Build: {{.Build}}{{end}}
  Order: {{.Placement}}
  Links: {{range $ind, $link := .Links}}
    {{$link.Container}}{{if $link.Alias}}:{{$link.Alias}}{{end}}{{end}}
  Hooks: {{range $key, $val := .Hooks}}
    {{$key}}
      {{range $hook := $val.Scripts}}{{$hook}}
      {{end}}{{end}}
  Scale: {{.Scale}}
  Volumes From: {{range $ind, $val := .VolumesFrom}}
    {{$val}}{{end}}
  Run Args:   {{range $ind, $val := .RunArguments}}
    {{$val}}{{end}}
`

var (
	allDone = make(chan bool, 1)
)

type ProjectConfig struct {
	ProjectName          string
	ProjectSeparator     string
	BlueGreenMode	     bool
	IsInteractive        bool
	ContainersState	     []*helpers.ServiceState
	ContainerList        SettingsList
	ContainerCleanupList SettingsList
}

type SettingsList []*container.Container

func (s SettingsList) Len() int {
	return len(s)
}
func (s SettingsList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SettingsList) Less(i, j int) bool {
	if s[i].Placement == s[j].Placement {
		iSuf, iErr := helpers.GetNumericSuffix(s[i].Name, s[i].ProjectNameSeparator)
		jSuf, jErr := helpers.GetNumericSuffix(s[j].Name, s[i].ProjectNameSeparator)
		if iErr == nil && jErr == nil {
			return iSuf < jSuf
		} else {
			return sort.StringsAreSorted([]string{s[i].Name, s[j].Name})
		}
	}
	return s[i].Placement < s[j].Placement
}

func (s SettingsList) Filter(cb func(*container.Container) bool) (filtered SettingsList) {
	filtered = make(SettingsList, 0)
	for _, item := range s {
		if cb(item) {
			filtered = append(filtered, item)
		}
	}
	return
}

func (settings *ProjectConfig) LaunchSignalWatcher() {

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
//		var calls int
		for {
			sig := <-signalChannel
			switch sig {
			case os.Interrupt, syscall.SIGTERM:

				for _, con := range append(settings.ContainerCleanupList, settings.ContainerList...) {
					for _, hooks := range con.Hooks {
						if hooks.Ses != nil {
							Debug.Println("killing hook...")
							hooks.Ses.Kill(syscall.SIGKILL)
						}
					}
				}
//				if settings.IsInteractive {
//					calls++
//					if calls == 1 {
//						go func() {
//							settings.ContainerList.CapitanStop(nil, false)
//							stopDone <- true
//						}()
//					} else if calls == 2 {
//						killBegan <- true
//						settings.ContainerList.CapitanKill(nil, false)
//						killDone <- true
//					}
//				} else {
					os.Exit(1)
//				}
			default:
				Debug.Println("Unhandled signal", sig)
			}
		}
		Info.Println("Done cleaning up")
	}()
}

func newerImage(container string, image string) bool {

	conImage := helpers.GetContainerImageId(container)
	localImage := helpers.GetImageId(image)
	if conImage != "" && localImage != "" && conImage != localImage {
		return true
	}
	return false
}

func haveArgsChanged(container string, runArgs []interface{}) bool {

	uniqueLabel := helpers.HashInterfaceSlice(runArgs)
	if helpers.GetContainerUniqueLabel(container) != uniqueLabel {
		return true
	}
	return false
	// remove and restart

}

func (settings *ProjectConfig) CapitanPs(args []string) error {

	allArgs := append([]interface{}{"ps"}, helpers.ToInterfaceSlice(args)...)
	allArgs = append(allArgs, "-f", fmt.Sprintf("label=%s=%s", consts.ProjectLabelName, settings.ProjectName))

	var (
		err error
		out []byte
	)
	if out, err = helpers.RunCmd(allArgs...); err != nil {
		return err
	}
	Info.Print(string(out))
	return nil

}

func (settings SettingsList) CapitanCreate(dryRun bool) error {
	sort.Sort(settings)

	for _, set := range settings {

		if set.Build != "" {
			Info.Println("Building image")
			if ! dryRun {
				if err := set.BuildImage(); err != nil {
					return err
				}
			}
		}

		if helpers.GetImageId(set.Image) == "" {
			Warning.Printf("Capitan was unable to find image %s locally\n", set.Image)

			Info.Println("Pulling image")
			if ! dryRun {
				if err := helpers.PullImage(set.Image); err != nil {
					return err
				}
			}
		}

		if err := set.Create(dryRun); err != nil {
			return err
		}
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
func (settings SettingsList) CapitanUp(attach bool, dryRun bool, blueGreenMode bool) error {
	sort.Sort(settings)

	wg := sync.WaitGroup{}

	for _, set := range settings {
		var (
			err error
		)

		if set.Build != "" {
			Info.Println("Building image")
			if ! dryRun {
				if err := set.BuildImage(); err != nil {
					return err
				}
			}
		}

		if helpers.GetImageId(set.Image) == "" {
			Warning.Printf("Capitan was unable to find image %s locally\n", set.Image)

			Info.Println("Pulling image")

			if ! dryRun {
				if err := helpers.PullImage(set.Image); err != nil {
					return err
				}
			}
		}

		//create new
		if !helpers.ContainerExists(set.Name) {
			if err = set.Run(attach, dryRun, &wg); err != nil {
				return err
			}
			continue
		}

		// disabling as this doesn't work with swarm (how do I know which node to look at??)
		//		if newerImage(set.Name, set.Image) {
		//			// remove and restart
		//			Info.Println("Removing (different image available):", set.Name)
		//			if err = set.RecreateAndRun(attach, dryRun, &wg); err != nil {
		//				return err
		//			}
		//
		//			continue
		//		}

		if haveArgsChanged(set.Name, set.GetRunArguments()) {
			// remove and restart
			if blueGreenMode {
				Info.Println("Run arguments changed, doing blue-green redeploy:", set.Name)
				if err = set.BlueGreenDeploy(attach, dryRun, &wg); err != nil {
					return err
				}
			} else {
				Info.Println("Removing (run arguments changed):", set.Name)
				if err = set.RecreateAndRun(attach, dryRun, &wg); err != nil {
					return err
				}
			}
			continue
		}

		//attach if running
		if set.State.Running {
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
	if !dryRun && attach {
		<-allDone
	}
	return nil
}

// Starts stopped containers
func (settings SettingsList) CapitanStart(attach bool, dryRun bool) error {
	sort.Sort(settings)
	wg := sync.WaitGroup{}
	for _, set := range settings {

		if set.State.Running {
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
	if !dryRun && attach {
		<-allDone
	}
	return nil
}

// Command to restart all containers
func (settings SettingsList) CapitanRestart(args []string, dryRun bool) error {
	sort.Sort(settings)
	for _, set := range settings {

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
func (settings SettingsList) CapitanIP() error {
	sort.Sort(settings)
	for _, set := range settings {
		ip := set.IP()
		Info.Printf("%s: %s", set.Name, ip)
	}
	return nil
}

// Stream all container logs
func (settings SettingsList) CapitanLogs() error {
	sort.Sort(settings)
	var wg sync.WaitGroup
	for _, set := range settings {
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
func (settings SettingsList) CapitanStats() error {
	var (
		args []interface{}
	)
	sort.Sort(settings)

	args = make([]interface{}, len(settings))

	for i, set := range settings {
		args[i] = set.Name
	}

	ses := sh.NewSession()
	ses.Command("docker", append([]interface{}{"stats"}, args...)...)
	ses.Start()
	ses.Wait()
	return nil
}

// Kill all running containers in project
func (settings SettingsList) CapitanKill(args []string, dryRun bool) error {
	sort.Sort(sort.Reverse(settings))
	for _, set := range settings {
		if !set.State.Running {
			Info.Println("Already stopped", set.Name)
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
func (settings SettingsList) CapitanStop(args []string, dryRun bool) error {
	sort.Sort(sort.Reverse(settings))
	for _, set := range settings {
		if !set.State.Running {
			Info.Println("Already stopped", set.Name)
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
func (settings SettingsList) CapitanRm(args []string, dryRun bool) error {
	sort.Sort(sort.Reverse(settings))
	for _, set := range settings {

		if helpers.ContainerExists(set.Name) {
			Info.Println("Removing " + set.Name)
			if dryRun {
				continue
			}
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
func (settings SettingsList) CapitanBuild(dryRun bool) error {
	sort.Sort(settings)
	for _, set := range settings {
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

// The build command
func (settings SettingsList) CapitanPull(dryRun bool) error {
	sort.Sort(settings)
	for _, set := range settings {
		if len(set.Build) > 0 || set.Image == "" {
			continue
		}
		Info.Println("Pulling", set.Image, "for", set.Name)
		if !dryRun {
			if err := helpers.PullImage(set.Image); err != nil {
				return err
			}
		}

	}
	return nil
}

func (settings SettingsList) CapitanShow() error {
	sort.Sort(settings)
	for _, set := range settings {
		var (
			tmpl *template.Template
			err  error
		)
		if tmpl, err = template.New("containerStringer").Parse(containerShowTemplate); err != nil {
			return err
		}
		if err = tmpl.Execute(os.Stdout, set); err != nil {
			return err
		}

	}
	return nil
}
