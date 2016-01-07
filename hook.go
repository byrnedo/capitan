package main

import (
	"github.com/codeskyblue/go-sh"
	"github.com/mgutz/str"
	"os"
)

// Runs a hook command if it exists for a specific container
func runHook(hookName string, settings *Container) error {
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
