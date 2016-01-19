package container

import (
	"github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
	"strconv"
)

type ShellSession struct {
	*sh.Session
}

func NewShellSession() *ShellSession {
	ses := ShellSession{sh.NewSession()}
	if logger.GetLevel() == logger.DebugLevel {
		ses.ShowCMD = true
	}
	return &ses
}

func NewContainerShellSession(ctr *Container) *ShellSession {
	ses := NewShellSession()
	ses.AddContainerEnvs(ctr)
	return ses
}

func (s *ShellSession) AddContainerEnvs(ctr *Container) {
	s.SetEnv("CAPITAN_CONTAINER_NAME", ctr.Name)
	s.SetEnv("CAPITAN_CONTAINER_SERVICE_TYPE", ctr.ServiceType)
	s.SetEnv("CAPITAN_CONTAINER_INSTANCE_NUMBER", strconv.Itoa(ctr.InstanceNumber))
	s.SetEnv("CAPITAN_PROJECT_NAME", ctr.ProjectName)
}
