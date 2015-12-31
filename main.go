package main

import (
	"github.com/codegangsta/cli"
	"os"
	. "github.com/byrnedo/capitan/logger"
)


func main() {
	app := cli.NewApp()
	app.Name = "capitan"
	app.Usage = "Deploy and orchestrate docker containers"

	var (
		filePath string
		verboseLog bool
	)

	app.Flags = []cli.Flag {
		cli.StringFlag{
			Name:        "file,f",
			Value:       "./capitan.cfg.sh",
			Usage:       "config file to read",
			Destination: &filePath,
		},
		cli.BoolFlag{
			Name:        "debug,d",
			Usage:       "print extra log messages",
			Destination: &verboseLog,
		},
	}

	app.Before = func(c *cli.Context) error {
		if verboseLog {
			SetDebug()
		}
		return nil
	}

	app.Action = func(c *cli.Context) {
		println("Please give a command")
	}


	app.Commands = []cli.Command{
		{
			Name:      "up",
			Aliases:     []string{},
			Usage:     "Create then run or update containers",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)
				if err := DockerRun(settings); err != nil {
					Error.Println(err)
					os.Exit(1)
				}

			},
		},
		{
			Name:      "ps",
			Aliases:     []string{},
			Usage:     "Show container status",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)
				if err := DockerPs(settings); err != nil {
					Error.Println(err)
					os.Exit(1)
				}

			},
		},
		{
			Name:      "build",
			Aliases:     []string{},
			Usage:     "Build any containers with 'build' flag set",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)
				if err := DockerBuild(settings); err != nil {
					Error.Println(err)
					os.Exit(1)
				}

			},
		},
		{
			Name:      "start",
			Aliases:     []string{},
			Usage:     "Start stopped containers",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)
				if err := DockerStart(settings); err != nil {
					Error.Println(err)
					os.Exit(1)
				}

			},
		},
		{
			Name:      "restart",
			Aliases:     []string{},
			Usage:     "Restart containers",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)
				if err := DockerRestart(settings, c.Int("time")); err != nil {
					Error.Println(err)
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{
				cli.IntFlag {
					Name: "time,t",
					Usage: "Time in seconds to wait before killing",
					Value: 10,
				},
			},
		},
		{
			Name:      "stop",
			Aliases:     []string{},
			Usage:     "Stop running containers",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)

				if err := DockerStop(settings, c.Int("time")); err != nil {
					Error.Println(err)
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{
				cli.IntFlag {
					Name: "time,t",
					Usage: "Time in seconds to wait before killing",
					Value: 10,
				},
			},
		},
		{
			Name:      "kill",
			Aliases:     []string{},
			Usage:     "Kill running containers using SIGKILL or a specified signal",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)

				if err := DockerKill(settings, c.String("signal")); err != nil {
					Error.Println(err)
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{
				cli.StringFlag {
					Name: "signal,s",
					Usage: "Signal to send to the containers",
					Value: "KILL",
				},
			},
		},
		{
			Name:      "rm",
			Aliases:     []string{},
			Usage:     "Remove stopped containers",
			Action: func(c *cli.Context) {
				settings := getSettings(filePath)

				if err := DockerRm(settings, c.Bool("force")); err != nil {
					Error.Println(err)
					os.Exit(1)
				}
			},
			Flags: []cli.Flag{
				cli.BoolFlag {
					Name: "force,f",
					Usage: "Force remove if container is running",
				},
			},
		},
	}



	app.Run(os.Args)
}

func getSettings(filePath string) ( cmds SettingsList ) {
	var (
		err error
	)
	runner := NewFileRunner(filePath)
	if cmds, err = runner.Run(); err != nil {
		Error.Printf("Error running file: %s\n", err)
		os.Exit(1)
	}
	return cmds
}
