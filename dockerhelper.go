package main

import (
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"io/ioutil"
	"strings"
	"time"
	"errors"
)

func containerExitCode(containerName string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.State.ExitCode}}", containerName).Output()
	if err != nil {
		return ""
	}
	imageId := strings.Trim(string(out), " \n")
	return imageId
}

func wasContainerStartedAfter(name string, afterTime time.Time) (bool, error) {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.State.StartedAt}}", name).Output()
	if err != nil {
		return false, err
	}
	timeStr := strings.Trim(string(out), " \n")

	startedAt, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return false, err
	}

	if startedAt.IsZero() {
		return false, errors.New("blank time found")
	}

	return afterTime.Before(startedAt), nil
}

func wasContainerStartedAfterOrRetry(name string, afterTime time.Time, maxAttempts int, interval time.Duration) bool {
	attempts := 0
	for {
		attempts++
		if valid, err := wasContainerStartedAfter(name, afterTime); err == nil {
			Debug.Println(err)
			return valid
		}

		if attempts >= maxAttempts {
			break
		} else {
			time.Sleep(interval)
		}
	}
	return false
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
