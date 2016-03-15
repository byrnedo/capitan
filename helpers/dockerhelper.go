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
	"path/filepath"
	"strings"
	"time"
"strconv"
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
func GetContainerUniqueLabel(containerName string) string {
	return getLabel(UniqueLabelName, containerName)
}

// Get the value of the label used to record the run
// service name (for scaling)
func GetContainerServiceNameLabel(containerName string) string {
	return getLabel(ServiceLabelName, containerName)
}

func RenameContainer(currentName string, newName string) error {
	ses := sh.NewSession()
	ses.Stderr = ioutil.Discard
	_, err := ses.Command("docker", "rename", currentName, newName).Output()
	return err
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

type ServiceState struct {
	ID   string
	Name string
	ServiceName string
	InstanceNum int
	Color string
	Running bool
}

func GetProjectState(projName string, projSep string) (svcs map[string]*ServiceState, err error) {
	ses := sh.NewSession()
	out, err := ses.Command("docker",
		"ps",
		"-af",
		fmt.Sprintf("label=%s=%s", ProjectLabelName, projName),
		"--format",
		fmt.Sprintf(`{{.ID}}\t{{.Names}}\t{{.Label "%s"}}\t{{.Label "%s"}}\t{{.Label "%s"}}\t{{.Status}}`, ColorLabelName, ServiceLabelName, ContainerNumberLabelName)).Output()
	if err != nil {
		return
	}
	if len(out) == 0 {
		return
	}

	out = bytes.Trim(out, "\n")

	svcs = make(map[string]*ServiceState, 0)
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		lineParts := bytes.Split(line, []byte{'\t'})

		id := string(lineParts[0])
		names := string(lineParts[1])

		if len(lineParts) < 2 {
			continue
		}
		var color string
		if len(lineParts) > 2 {
			color = string(lineParts[2])
		}
		if color == "" {
			color = "blue"
		}

		var serviceName string
		if len(lineParts) > 3 {
			serviceName = string(lineParts[3])
		}

		var instanceNum int
		if len(lineParts) > 4 {
			if instanceNum, err = strconv.Atoi(string(lineParts[4])); err != nil {
				Warning.Println("Instance number label missing, parsing from name")
				if instanceNum, err = GetNumericSuffix(names, projSep); err != nil {
					return nil, errors.New("Failed to parse instance number for container: " + names)
				}
			}
		}

		var running bool
		if len(lineParts) > 5 {
			if bytes.HasPrefix(lineParts[5], []byte("Up")){
				running = true
			}
		}
		name := filepath.Base(names)
		svcs[serviceName + projSep + strconv.Itoa(instanceNum)] = &ServiceState{
			ID: id,
			Name: name,
			ServiceName: serviceName,
			InstanceNum: instanceNum,
			Color: color,
			Running: running,
		}
	}
	return
}

