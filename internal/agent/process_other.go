//go:build !linux

package agent

import "syscall"

func gameProcessAttributes() *syscall.SysProcAttr { return nil }
