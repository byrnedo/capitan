package helpers

import (
	"bytes"
	"errors"
	"fmt"
	. "github.com/byrnedo/capitan/consts"
	"github.com/byrnedo/capitan/logger"
	. "github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"io/ioutil"
	"strings"
	"time"
)

func ContainerExitCode(containerName string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.State.ExitCode}}", containerName).Output()
	if err != nil {
		return ""
	}
	imageId := strings.Trim(string(out), " \n")
	return imageId
}

func WasContainerStartedAfter(name string, afterTime time.Time) (bool, error) {
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

func WasContainerStartedAfterOrRetry(name string, afterTime time.Time, maxAttempts int, interval time.Duration) bool {
	attempts := 0
	for {
		attempts++
		if valid, err := WasContainerStartedAfter(name, afterTime); err == nil {
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
func GetImageId(imageName string) string {
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
func PullImage(imageName string) error {
	err := sh.Command("docker", "pull", imageName).Run()
	return err
}

// Get the image id for a given container
func GetContainerImageId(name string) string {
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
func ContainerExists(name string) bool {
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
func ContainerIsRunning(name string) bool {
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

// Helper to run a docker command
func RunCmd(args ...interface{}) (out []byte, err error) {
	ses := sh.NewSession()

	if logger.GetLevel() == DebugLevel {
		ses.ShowCMD = true
	}

	out, err = ses.Command("docker", args...).Output()
	Debug.Println(string(out))
	if err != nil {
		return out, errors.New("Error running docker command:" + err.Error())
	}
	return out, nil
}

// Get the value of the label used to record the run
// arguments used when creating the container
func GetContainerUniqueLabel(name string) string {
	return getLabel(UniqueLabelName, name)
}

// Get the value of the label used to record the run
// service name (for scaling)
func GetContainerServiceNameLabel(name string) string {
	return getLabel(ServiceLabelName, name)
}

func getLabel(label string, container string) string {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	out, err := ses.Command("docker", "inspect", "--type", "container", "--format", "{{.Config.Labels."+label+"}}", container).Output()
	if err != nil {
		return ""
	}
	value := strings.Trim(string(out), " \n")
	return value
}

type Service struct {
	ID   string
	Name string
}

func InstancesOfService(service string) (svcs []*Service) {
	ses := sh.NewSession()
	out, err := ses.Command("docker", "ps", "-f", fmt.Sprintf("label=%s=%s", ServiceLabelName, service), "--format", "{{.ID}} {{.Names}}").Output()
	if err != nil {
		return
	}
	if len(out) == 0 {
		return
	}

	out = bytes.Trim(out, "\n")

	svcs = make([]*Service, 0)
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		lineParts := bytes.Split(line, []byte{' '})
		if len(lineParts) != 2 {
			continue
		}
		svcs = append(svcs, &Service{ID: string(lineParts[0]), Name: string(lineParts[1])})
	}
	return

}
