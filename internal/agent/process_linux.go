package agent

import "syscall"

func gameProcessAttributes() *syscall.SysProcAttr {
	// A killed agent must not leave an untracked game holding the instance ports.
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
