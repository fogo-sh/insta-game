package control

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Docker struct {
	client     *http.Client
	PublicHost string
	BindIP     string
	bootedAt   time.Time
}

func NewDocker(socket, publicHost, bindIP string) *Docker {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	return &Docker{client: &http.Client{Transport: transport}, PublicHost: publicHost, BindIP: bindIP, bootedAt: hostBootTime()}
}

type dockerError struct {
	status  int
	message string
}

func (e *dockerError) Error() string { return fmt.Sprintf("Docker: %s (HTTP %d)", e.message, e.status) }
func (d *Docker) request(ctx context.Context, method, path string, body, result any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
		return &dockerError{res.StatusCode, string(b)}
	}
	if result == nil {
		_, err = io.Copy(io.Discard, res.Body)
		return err
	}
	return json.NewDecoder(res.Body).Decode(result)
}
func containerName(i Instance) string { return "insta-game-" + i.ID }
func containerPath(i Instance) string { return "/containers/" + url.PathEscape(containerName(i)) }

type dockerInspect struct {
	ID    string
	State struct {
		Status    string
		Running   bool
		Error     string
		ExitCode  int
		StartedAt time.Time
	}
	Config          struct{ Labels map[string]string }
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string
		}
	}
}

func owner(i Instance) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(i.AgentToken))) }
func (d *Docker) inspect(ctx context.Context, i Instance) (dockerInspect, error) {
	var state dockerInspect
	err := d.request(ctx, "GET", containerPath(i)+"/json", nil, &state)
	if e, ok := err.(*dockerError); ok && e.status == 404 {
		return dockerInspect{}, nil
	}
	if err != nil {
		return state, err
	}
	if state.Config.Labels["gg.insta-game.instance"] != i.ID || state.Config.Labels["gg.insta-game.owner"] != owner(i) {
		return state, fmt.Errorf("container name is occupied by an unmanaged container")
	}
	if state.ID == "" {
		return state, fmt.Errorf("Docker returned an empty container ID")
	}
	return state, nil
}
func (d *Docker) Inspect(ctx context.Context, i Instance) (Observation, error) {
	state, err := d.inspect(ctx, i)
	if err != nil {
		return Observation{}, err
	}
	if state.ID == "" {
		return Observation{}, nil
	}

	observation := Observation{Exists: true, Started: state.State.Status != "created", Running: state.State.Running, Failure: state.State.Error}
	if !state.State.Running && state.State.ExitCode != 0 {
		observation.Failure = fmt.Sprintf("container exited with code %d: %s", state.State.ExitCode, state.State.Error)
	}
	if !state.State.Running && !state.State.StartedAt.IsZero() && state.State.StartedAt.Before(d.bootedAt) {
		observation.Started = false
		observation.Failure = ""
	}
	keys := make([]string, 0, len(state.NetworkSettings.Ports))
	for key := range state.NetworkSettings.Ports {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		bindings := state.NetworkSettings.Ports[key]
		if len(bindings) == 0 {
			continue
		}
		parts := strings.Split(key, "/")
		port, _ := strconv.Atoi(parts[0])
		if port == i.Game.AgentPort && parts[1] == "tcp" {
			observation.AgentURL = "http://" + net.JoinHostPort("127.0.0.1", bindings[0].HostPort)
			continue
		}
		if parts[0] == state.Config.Labels["gg.insta-game.game-port"] {
			port = i.Game.GamePort
		}
		observation.Endpoints = append(observation.Endpoints, Endpoint{Port: port, Protocol: parts[1], Address: net.JoinHostPort(d.PublicHost, bindings[0].HostPort)})
	}
	return observation, nil
}
func (d *Docker) Ensure(ctx context.Context, i Instance) (Observation, error) {
	observation, err := d.Inspect(ctx, i)
	if err != nil || observation.Exists {
		return observation, err
	}
	ports := map[string]any{}
	exposed := map[string]any{}

	// Engines using TCP and UDP on one port need the same external port for both.
	// Reserve both sockets while choosing it; Docker remains authoritative at start.
	shared := map[string]string{}
	for key := range i.Game.Ports {
		port, protocol, _ := strings.Cut(key, "/")
		if protocol != "tcp" {
			continue
		}
		if _, ok := i.Game.Ports[port+"/udp"]; !ok {
			continue
		}
		tcp, err := net.Listen("tcp", net.JoinHostPort(d.BindIP, "0"))
		if err != nil {
			return Observation{}, err
		}
		defer tcp.Close()
		number := strconv.Itoa(tcp.Addr().(*net.TCPAddr).Port)
		udp, err := net.ListenPacket("udp", net.JoinHostPort(d.BindIP, number))
		if err != nil {
			return Observation{}, err
		}
		defer udp.Close()
		shared[port] = number
	}
	gamePort := strconv.Itoa(i.Game.GamePort)
	runtimePort := gamePort
	if strings.Contains(i.Game.GameArgs, "{port}") {
		if _, ok := i.Game.Ports[gamePort+"/udp"]; !ok {
			return Observation{}, fmt.Errorf("game port templates require a UDP game port")
		}
		if shared[gamePort] == "" {
			listener, err := net.ListenPacket("udp", net.JoinHostPort(d.BindIP, "0"))
			if err != nil {
				return Observation{}, err
			}
			defer listener.Close()
			shared[gamePort] = strconv.Itoa(listener.LocalAddr().(*net.UDPAddr).Port)
		}
		runtimePort = shared[gamePort]
	}
	for port := range i.Game.Ports {
		if port == fmt.Sprintf("%d/tcp", i.Game.AgentPort) {
			continue
		}
		number, _, _ := strings.Cut(port, "/")
		hostPort := shared[number]
		if number == gamePort && runtimePort != gamePort {
			_, transport, _ := strings.Cut(port, "/")
			port = runtimePort + "/" + transport
			hostPort = runtimePort
		}
		ports[port] = []map[string]string{{"HostIp": d.BindIP, "HostPort": hostPort}}
		exposed[port] = struct{}{}
	}
	management := fmt.Sprintf("%d/tcp", i.Game.AgentPort)
	ports[management] = []map[string]string{{"HostIp": "127.0.0.1", "HostPort": ""}}
	exposed[management] = struct{}{}
	env := []string{"GAME_CMD=" + i.Game.GameCmd, "GAME_ARGS=" + strings.ReplaceAll(i.Game.GameArgs, "{port}", runtimePort), "GAME_PORT=" + runtimePort, "PROTOCOL=" + i.Game.Protocol, "PORT=" + strconv.Itoa(i.Game.AgentPort), "CONFIG_PATH=" + i.Game.ConfigPath, "CONFIG_TEXT=" + i.Config, "TOKEN=" + i.AgentToken, "RCON_PASSWORD=" + i.RconPassword, "GAME_QUIT_CMD=" + i.Game.GameQuitCmd, "GAME_QUIT_TIMEOUT=" + strconv.Itoa(i.Game.GameQuitTimeout)}
	if i.Game.DataURLEnv != "" {
		if value := os.Getenv(i.Game.DataURLEnv); value != "" {
			env = append(env, "DATA_URL="+value)
		}
	}
	body := map[string]any{"Image": i.Game.Image, "Env": env, "ExposedPorts": exposed, "Labels": map[string]string{"gg.insta-game.instance": i.ID, "gg.insta-game.owner": owner(i), "gg.insta-game.game-port": runtimePort}, "HostConfig": map[string]any{"PortBindings": ports, "Init": true, "RestartPolicy": map[string]string{"Name": "no"}, "LogConfig": map[string]any{"Type": "json-file", "Config": map[string]string{"max-size": "10m", "max-file": "2"}}}}
	err = d.request(ctx, "POST", "/containers/create?name="+url.QueryEscape(containerName(i)), body, nil)
	if e, ok := err.(*dockerError); ok && e.status == 404 {
		return Observation{}, fmt.Errorf("image %s is not installed; build or pull it before creating a server", i.Game.Image)
	}
	if err != nil {
		return Observation{}, err
	}
	return d.Inspect(ctx, i)
}
func (d *Docker) mutate(ctx context.Context, i Instance, method, suffix string) error {
	state, err := d.inspect(ctx, i)
	if err != nil {
		return err
	}
	if state.ID == "" {
		if method == "DELETE" {
			return nil
		}
		return fmt.Errorf("container does not exist")
	}
	err = d.request(ctx, method, "/containers/"+url.PathEscape(state.ID)+suffix, nil, nil)
	if e, ok := err.(*dockerError); ok && e.status == 404 && method == "DELETE" {
		return nil
	}
	return err
}
func (d *Docker) Start(ctx context.Context, i Instance) error {
	return d.mutate(ctx, i, "POST", "/start")
}
func (d *Docker) Stop(ctx context.Context, i Instance) error {
	timeout := i.Game.GameQuitTimeout + 5
	if timeout < 20 {
		timeout = 20
	}
	return d.mutate(ctx, i, "POST", fmt.Sprintf("/stop?t=%d", timeout))
}
func (d *Docker) Delete(ctx context.Context, i Instance) error {
	return d.mutate(ctx, i, "DELETE", "?force=true&v=true")
}

func hostBootTime() time.Time {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "btime "); ok {
			seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err == nil {
				return time.Unix(seconds, 0)
			}
		}
	}
	return time.Time{}
}
