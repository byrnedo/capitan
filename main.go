package main

import (
	. "github.com/byrnedo/capitan/logger"
	"github.com/codegangsta/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "capitan"
	app.Usage = "Deploy and orchestrate docker containers"
	app.Version = "0.1"

	var (
		command    string
		verboseLog bool
		dryRun     bool
		attach     bool
	)

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
				settings := getSettings(command)
				settings.LaunchCleanupWatcher()
				if err := settings.CapitanUp(attach, dryRun); err != nil {
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
			Name:            "ps",
			Aliases:         []string{},
			Usage:           "Show container status",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings(command)
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
				settings := getSettings(command)
				if err := settings.CapitanIP(); err != nil {
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
				settings := getSettings(command)
				if err := settings.CapitanBuild(dryRun); err != nil {
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
				settings := getSettings(command)
				if err := settings.CapitanPull(dryRun); err != nil {
					Error.Println("Pull failed:", err)
					os.Exit(1)
				}

			},
		},
		{
			Name:    "start",
			Aliases: []string{},
			Usage:   "Start stopped containers",
			Action: func(c *cli.Context) {
				settings := getSettings(command)
				settings.LaunchCleanupWatcher()
				if err := settings.CapitanStart(attach, dryRun); err != nil {
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
			Name:            "restart",
			Aliases:         []string{},
			Usage:           "Restart containers",
			SkipFlagParsing: true,
			Action: func(c *cli.Context) {
				settings := getSettings(command)
				if err := settings.CapitanRestart(c.Args(), dryRun); err != nil {
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
				settings := getSettings(command)

				if err := settings.CapitanStop(c.Args(), dryRun); err != nil {
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
				settings := getSettings(command)

				if err := settings.CapitanKill(c.Args(), dryRun); err != nil {
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
				settings := getSettings(command)

				if err := settings.CapitanRm(c.Args(), dryRun); err != nil {
					Error.Println("Rm failed:", err)
					os.Exit(1)
				}
			},
		},
		{
			Name:    "logs",
			Aliases: []string{},
			Usage:   "stream container logs",
			Action: func(c *cli.Context) {
				settings := getSettings(command)
				if err := settings.CapitanLogs(); err != nil {
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
				settings := getSettings(command)
				if err := settings.CapitanStats(); err != nil {
					Error.Println("Stats failed:", err)
					os.Exit(1)
				}

			},
		},
	}
	app.Run(os.Args)
}

func getSettings(settingsCmd string) (settings *ProjectSettings) {
	var (
		err error
	)
	runner := NewSettingsParser(settingsCmd)
	if settings, err = runner.Run(); err != nil {
		Error.Printf("Error running command: %s\n", err)
		os.Exit(1)
	}
	return settings
}
