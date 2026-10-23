//go:build linux || darwin

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// RunFile holds an advisory lock for the lifetime of a detached native agent.
// The controller probes the same lock rather than trusting a reusable process ID.
func RunFile(ctx context.Context, path string) error {
	lock, err := os.OpenFile(filepath.Join(filepath.Dir(path), "run.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("agent already owns this instance: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	return Run(ctx, c)
}
