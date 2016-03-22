package shellsession

import (
	"github.com/byrnedo/capitan/logger"
	"github.com/codeskyblue/go-sh"
)

type ShellSession struct {
	*sh.Session
}

func NewShellSession(init func(*ShellSession)) *ShellSession {
	ses := ShellSession{sh.NewSession()}
	if logger.GetLevel() == logger.DebugLevel {
		ses.ShowCMD = true
	}
	init(&ses)
	return &ses
}

