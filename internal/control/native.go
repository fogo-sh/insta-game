//go:build linux || darwin

package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fogo-sh/insta-game/internal/agent"
)

// NativeGame describes an operator-installed engine and its asset layout.
// Assets are copied into each instance's working directory.
type NativeGame struct {
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	Assets     string   `json:"assets"`
	ConfigPath string   `json:"configPath"`
}

type Native struct {
	Root, Binary, PublicHost string
	Games                    map[string]NativeGame
}

type nativeState struct {
	BootID    string
	Token     string
	Port      int
	Endpoints []Endpoint
	Started   bool
}

func (n *Native) directory(i Instance) string { return filepath.Join(n.Root, i.ID) }
func (n *Native) state(i Instance) (nativeState, error) {
	var state nativeState
	data, err := os.ReadFile(filepath.Join(n.directory(i), "state.json"))
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	if state.Token != i.AgentToken {
		return state, fmt.Errorf("native instance ownership mismatch")
	}
	return state, nil
}

func writePrivateJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func nativeAlive(dir string) (bool, error) {
	lock, err := os.OpenFile(filepath.Join(dir, "run.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, err
	}
	defer lock.Close()
	err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
}

func (n *Native) request(ctx context.Context, i Instance, state nativeState, method, path string, body io.Reader, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:"+strconv.Itoa(state.Port)+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+i.AgentToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Duration(i.Game.GameQuitTimeout+5) * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("native agent HTTP %d", res.StatusCode)
	}
	if result == nil {
		_, err := io.Copy(io.Discard, res.Body)
		return err
	}
	return json.NewDecoder(res.Body).Decode(result)
}

func (n *Native) Inspect(ctx context.Context, i Instance) (Observation, error) {
	state, err := n.state(i)
	if errors.Is(err, os.ErrNotExist) {
		return Observation{}, nil
	}
	if err != nil {
		return Observation{}, err
	}
	endpoints := append([]Endpoint(nil), state.Endpoints...)
	for index := range endpoints {
		_, port, err := net.SplitHostPort(endpoints[index].Address)
		if err != nil {
			return Observation{}, err
		}
		endpoints[index].Address = net.JoinHostPort(n.PublicHost, port)
	}
	observation := Observation{Exists: true, Started: state.Started, Endpoints: endpoints}
	alive, err := nativeAlive(n.directory(i))
	if err != nil || !alive {
		bootID := nativeBootID()
		if err == nil && bootID != "" && state.BootID != "" && state.BootID != bootID {
			observation.Started = false
		}
		return observation, err
	}
	var status AgentStatus
	if err := n.request(ctx, i, state, "GET", "/status", nil, &status); err != nil {
		return observation, err
	}
	observation.Running = status.Running
	observation.Failure = status.Error
	observation.AgentURL = "http://127.0.0.1:" + strconv.Itoa(state.Port)
	return observation, nil
}

func reservePort() (int, func(), error) {
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	port := tcp.Addr().(*net.TCPAddr).Port
	udp, err := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		tcp.Close()
		return 0, nil, err
	}
	return port, func() { tcp.Close(); udp.Close() }, nil
}

func (n *Native) Ensure(ctx context.Context, i Instance) (Observation, error) {
	if _, err := n.state(i); err == nil {
		return n.Inspect(ctx, i)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Observation{}, err
	}
	game, ok := n.Games[i.Game.ID]
	if !ok {
		return Observation{}, fmt.Errorf("native engine %q is not configured", i.Game.ID)
	}
	if !filepath.IsAbs(game.Command) || !filepath.IsAbs(game.Assets) || !filepath.IsLocal(game.ConfigPath) {
		return Observation{}, fmt.Errorf("native command/assets must be absolute and configPath must be relative")
	}
	if _, err := os.Stat(game.Command); err != nil {
		return Observation{}, err
	}
	dir := n.directory(i)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return Observation{}, err
	}
	// Reserve ownership before copying. A retry finishes preparation for this instance.
	owner := filepath.Join(dir, "owner")
	f, err := os.OpenFile(owner, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		_, err = f.WriteString(i.AgentToken)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	} else if errors.Is(err, os.ErrExist) {
		data, readErr := os.ReadFile(owner)
		err = readErr
		if err == nil && string(data) != i.AgentToken {
			err = fmt.Errorf("native directory is owned by another instance")
		}
	}
	if err != nil {
		return Observation{}, err
	}
	work := filepath.Join(dir, "work")
	if err := copyAssets(ctx, game.Assets, work); err != nil {
		return Observation{}, err
	}
	config := filepath.Join(work, game.ConfigPath)
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		return Observation{}, err
	}
	if err := os.WriteFile(config, []byte(i.Config), 0600); err != nil {
		return Observation{}, err
	}
	state := nativeState{Token: i.AgentToken}
	port, release, err := reservePort()
	if err != nil {
		return Observation{}, err
	}
	defer release()
	state.Port = port
	replacements := []string{"{config}", config, "{data}", work}
	reserved := map[int]int{}
	for key := range i.Game.Ports {
		original, err := strconv.Atoi(strings.Split(key, "/")[0])
		if original == i.Game.AgentPort {
			continue
		}
		if err != nil {
			return Observation{}, err
		}
		assigned := reserved[original]
		if assigned == 0 {
			var closePort func()
			assigned, closePort, err = reservePort()
			if err != nil {
				return Observation{}, err
			}
			defer closePort()
			reserved[original] = assigned
			replacements = append(replacements, "{port:"+strconv.Itoa(original)+"}", strconv.Itoa(assigned))
		}
		if original == i.Game.GamePort {
			replacements = append(replacements, "{port}", strconv.Itoa(assigned))
		}
		state.Endpoints = append(state.Endpoints, Endpoint{Port: original, Protocol: strings.Split(key, "/")[1], Address: net.JoinHostPort(n.PublicHost, strconv.Itoa(assigned))})
	}
	replacer := strings.NewReplacer(replacements...)
	args := make([]string, len(game.Args))
	for index, value := range game.Args {
		args[index] = replacer.Replace(value)
	}
	if err := os.WriteFile(filepath.Join(dir, "default.cfg"), []byte(i.Config), 0600); err != nil {
		return Observation{}, err
	}
	c := agent.Config{Protocol: i.Game.Protocol, GameDir: work, GameCmd: game.Command, GameArgs: args, GamePort: reserved[i.Game.GamePort], GameQuitCmd: i.Game.GameQuitCmd, GameQuitTimeout: time.Duration(i.Game.GameQuitTimeout) * time.Second, ConfigPath: config, DefaultConfig: filepath.Join(dir, "default.cfg"), RconPassword: i.RconPassword, Token: i.AgentToken, Host: "127.0.0.1", Port: state.Port}
	if err := writePrivateJSON(filepath.Join(dir, "agent.json"), c); err != nil {
		return Observation{}, err
	}
	if err := writePrivateJSON(filepath.Join(dir, "state.json"), state); err != nil {
		return Observation{}, err
	}
	return Observation{Exists: true, Endpoints: state.Endpoints}, nil
}

func copyAssets(ctx context.Context, source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("native assets must be regular files: %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		info, err := entry.Info()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, err = io.Copy(output, input)
		closeErr := output.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}

func (n *Native) Start(ctx context.Context, i Instance) error {
	state, err := n.state(i)
	if err != nil {
		return err
	}
	alive, err := nativeAlive(n.directory(i))
	if err != nil {
		return err
	}
	if alive {
		body, _ := json.Marshal(map[string]string{"config_text": i.Config})
		return n.request(ctx, i, state, "POST", "/restart", bytes.NewReader(body), nil)
	}
	state.Started = true
	state.BootID = nativeBootID()
	if err := writePrivateJSON(filepath.Join(n.directory(i), "state.json"), state); err != nil {
		return err
	}
	cmd := exec.Command(n.Binary, "agent", "--config", filepath.Join(n.directory(i), "agent.json"))
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + filepath.Join(n.directory(i), "work"), "LANG=C.UTF-8"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	output, err := os.OpenFile(filepath.Join(n.directory(i), "agent.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return fmt.Errorf("native agent exited during startup (%v); see agent.log", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			alive, err := nativeAlive(n.directory(i))
			if err != nil {
				return err
			}
			if alive {
				return nil
			}
		}
	}
}

func (n *Native) Stop(ctx context.Context, i Instance) error {
	state, err := n.state(i)
	if err != nil {
		return err
	}
	alive, err := nativeAlive(n.directory(i))
	if err != nil || !alive {
		return err
	}
	return n.request(ctx, i, state, "POST", "/stop", nil, nil)
}

func (n *Native) Delete(ctx context.Context, i Instance) error {
	state, err := n.state(i)
	if errors.Is(err, os.ErrNotExist) {
		owner, readErr := os.ReadFile(filepath.Join(n.directory(i), "owner"))
		if errors.Is(readErr, os.ErrNotExist) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		if string(owner) != i.AgentToken {
			return fmt.Errorf("native directory ownership mismatch")
		}
	} else if err != nil {
		return err
	} else {
		alive, err := nativeAlive(n.directory(i))
		if err != nil {
			return err
		}
		if alive {
			if err := n.request(ctx, i, state, "POST", "/shutdown", nil, nil); err != nil {
				return err
			}
			ticker := time.NewTicker(25 * time.Millisecond)
			defer ticker.Stop()
			for alive {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
				}
				alive, err = nativeAlive(n.directory(i))
				if err != nil {
					return err
				}
			}
		}
	}
	return os.RemoveAll(n.directory(i))
}

func nativeBootID() string {
	data, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(data))
}
