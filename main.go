package main

import (
	. "github.com/byrnedo/capitan/logger"
	"github.com/codegangsta/cli"
	"os"
"github.com/byrnedo/capitan/container"
)

var (
	command    string
	args       []string
	verboseLog bool
	dryRun     bool
	attach     bool
)

func main() {
	app := cli.NewApp()
	app.Name = "capitan"
	app.Usage = "Deploy and orchestrate docker containers"
	app.Version = "0.1"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "cmd,c",
			Value:       "./capitan.cfg.sh",
			Usage:       "command to obtain config from",
			Destination: &command,
		},
		cli.BoolFlag{
			Name:        "debug,d",
			Usage:       "print extra log messages",
			Destination: &verboseLog,
		},
		cli.BoolFlag{
			Name:        "dry-run,dry",
			Usage:       "preview outcome, no changes will be made",
			Destination: &dryRun,
		},
	}

	app.Before = func(c *cli.Context) error {
		if verboseLog {
			SetDebug()
		}

		if dryRun {
			Info.Printf("Previewing changes...\n\n")
		}

		args = c.Args()
		return nil
	}

	app.Action = func(c *cli.Context) {
		cli.ShowAppHelp(c)
		os.Exit(1)
	}

	app.Commands = []cli.Command{
		{
			Name:    "up",
			Aliases: []string{},
			Usage:   "Create then run or update containers",
			Action: func(c *cli.Context) {
				settings := getSettings()
				settings.LaunchSignalWatcher()
				if err := settings.ContainerCleanupList.CapitanStop(nil, dryRun); err != nil {
					Warning.Println("Failed to scale down containers:", err)
				}
				if err := settings.ContainerList.CapitanUp(attach, dryRun); err != nil {
					Error.Println("Up failed:", err)
					os.Exit(1)
				}

			},
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "attach,a",
					Usage:       "attach to container output",
					Destination: &attach,
				},
			},
		},
		{
			Name:    "create",
			Aliases: []string{},
			Usage:   "Create containers, but don't run them",
			Action: func(c *cli.Context) {
				settings := getSettings()
				if err := settings.ContainerCleanupList.CapitanStop(nil, dryRun); err != nil {
					Warning.Println("Failed to scale down containers:", err)
				}
				if err := settings.ContainerList.CapitanCreate(dryRun); err != nil {
					Error.Println("Create failed:", err)
					os.Exit(1)
				}

			},
		},
		{
			Name:    "start",
			Aliases: []string{},
			Usage:   "Start stopped containers",
			Action: func(c *cli.Context) {
				settings := getSettings()
				settings.LaunchSignalWatcher()
				if err := settings.ContainerCleanupList.CapitanStop(nil, dryRun); err != nil {
					Warning.Println("Failed to scale down containers:", err)
				}
				if err := settings.ContainerList.CapitanStart(attach, dryRun); err != nil {
					Error.Println("Start failed:", err)
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "attach,a",
					Usage:       "attach to container output",
					Destination: &attach,
				},
			},
		},
		{
			Name:            "scale",
			Aliases:         []string{},
			Usage:           "Number of instances to run of container",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings()
				if err := settings.ContainerCleanupList.Filter(func(i *container.Container)bool {
					return i.ServiceType == c.Args().Get(0)
				}).CapitanStop(nil, dryRun); err != nil {
					Warning.Println("Failed to scale down containers:", err)
				}
				if err := settings.ContainerList.Filter(func(i *container.Container)bool{
					return i.ServiceType == c.Args().Get(0)
				}).CapitanUp(false, dryRun); err != nil {
					Error.Println("Scale failed:", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:            "restart",
			Aliases:         []string{},
			Usage:           "Restart containers",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings()
				if err := settings.ContainerCleanupList.CapitanStop(nil, dryRun); err != nil {
					Warning.Println("Failed to scale down containers:", err)
				}
				if err := settings.ContainerList.CapitanRestart(c.Args(), dryRun); err != nil {
					Error.Println("Restart failed:", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:            "stop",
			Aliases:         []string{},
			Usage:           "Stop running containers",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings()
				combined := append(settings.ContainerList, settings.ContainerCleanupList...)
				if err := combined.CapitanStop(c.Args(), dryRun); err != nil {
					Error.Println("Stop failed:", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:            "kill",
			Aliases:         []string{},
			Usage:           "Kill running containers using SIGKILL or a specified signal",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings()
				combined := append(settings.ContainerList, settings.ContainerCleanupList...)
				if err := combined.CapitanKill(c.Args(), dryRun); err != nil {
					Error.Println("Kill failed:", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:            "rm",
			Aliases:         []string{},
			Usage:           "Remove stopped containers",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings()
				combined := append(settings.ContainerList, settings.ContainerCleanupList...)
				if err := combined.CapitanRm(c.Args(), dryRun); err != nil {
					Error.Println("Rm failed:", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:            "ps",
			Aliases:         []string{},
			Usage:           "Show container status",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings()
				if err := settings.CapitanPs(c.Args()); err != nil {
					Error.Println("Ps failed:", err)
					os.Exit(1)
				}

			},
		},
		{
			Name:            "ip",
			Aliases:         []string{},
			Usage:           "Show container ip addresses",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings()
				if err := settings.ContainerList.CapitanIP(); err != nil {
					Error.Println("IP failed:", err)
					os.Exit(1)
				}

			},
		},
		{
			Name:    "build",
			Aliases: []string{},
			Usage:   "Build any containers with 'build' flag set",
			Action: func(c *cli.Context) {
				settings := getSettings()
				if err := settings.ContainerList.CapitanBuild(dryRun); err != nil {
					Error.Println("Build failed:", err)
					os.Exit(1)
				}

			},
		},
		{
			Name:    "pull",
			Aliases: []string{},
			Usage:   "Pull all images defined in project",
			Action: func(c *cli.Context) {
				settings := getSettings()
				if err := settings.ContainerList.CapitanPull(dryRun); err != nil {
					Error.Println("Pull failed:", err)
					os.Exit(1)
				}

			},
		},
		{
			Name:    "logs",
			Aliases: []string{},
			Usage:   "stream container logs",
			Action: func(c *cli.Context) {
				settings := getSettings()
				combined := append(settings.ContainerList, settings.ContainerCleanupList...)
				if err := combined.CapitanLogs(); err != nil {
					Error.Println("Logs failed:", err)
					os.Exit(1)
				}

			},
		},
		{
			Name:    "stats",
			Aliases: []string{},
			Usage:   "stream stats for all containers in project",
			Action: func(c *cli.Context) {
				settings := getSettings()
				combined := append(settings.ContainerList, settings.ContainerCleanupList...)
				if err := combined.CapitanStats(); err != nil {
					Error.Println("Stats failed:", err)
					os.Exit(1)
				}

			},
		},
	}
	app.Run(os.Args)
}

func getSettings() (settings *ProjectConfig) {
	var (
		err error
	)
	runner := NewSettingsParser(command, args)
	if settings, err = runner.Run(); err != nil {
		Error.Printf("Error running command: %s\n", err)
		os.Exit(1)
	}
	return settings
}
