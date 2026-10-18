package agent

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Process owns exactly one child and reaps it whether it exits naturally or is stopped.
type Process struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	done    chan struct{}
	running bool
	failure string
}

func (p *Process) Start(c Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return fmt.Errorf("game is already running")
	}
	cmd := exec.Command(c.GameCmd, c.GameArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = io.MultiWriter(os.Stdout, gameLogs)
	cmd.Stderr = io.MultiWriter(os.Stderr, gameLogs)
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return err
	}
	p.cmd, p.stdin, p.done, p.running, p.failure = cmd, stdin, make(chan struct{}), true, ""
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		defer p.mu.Unlock()
		p.running = false
		if err != nil {
			p.failure = err.Error()
		}
		stdin.Close()
		close(p.done)
	}()
	return nil
}

func (p *Process) Stop(c Config) {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	cmd, done := p.cmd, p.done
	fmt.Fprintln(p.stdin, c.GameQuitCmd)
	p.stdin.Close()
	p.mu.Unlock()
	timeout := c.GameQuitTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	select {
	case <-done:
	case <-time.After(timeout):
		cmd.Process.Kill()
		<-done
	}
}

func (p *Process) Command(command string) error {
	if strings.ContainsAny(command, "\r\n") || len(command) > 4096 || strings.TrimSpace(command) == "" {
		return fmt.Errorf("expected one console command")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return fmt.Errorf("game is not running")
	}
	_, err := fmt.Fprintln(p.stdin, command)
	return err
}

func (p *Process) State() (bool, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running, p.failure
}
