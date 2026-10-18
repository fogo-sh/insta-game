package agent

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestProcessReapsNaturalExitAndCanRestart(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("requires sh")
	}
	var p Process
	c := Config{GameCmd: shell, GameArgs: []string{"-c", "exit 7"}, GameQuitTimeout: time.Second}
	if err := p.Start(c); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for {
		running, failure := p.State()
		if !running {
			if failure == "" {
				t.Fatal("lost exit failure")
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("process was not reaped")
		case <-time.After(time.Millisecond):
		}
	}
	c.GameArgs = []string{"-c", "read line"}
	c.GameQuitCmd = "quit"
	if err := p.Start(c); err != nil {
		t.Fatal(err)
	}
	p.Stop(c)
	if running, _ := p.State(); running {
		t.Fatal("still running after stop")
	}
}
func TestPortableAgentRequiresCredentials(t *testing.T) {
	if err := Run(context.Background(), Config{GameCmd: "does-not-matter"}); err == nil {
		t.Fatal("agent accepted an empty management token")
	}
}
