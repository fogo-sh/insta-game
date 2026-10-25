package fleet

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// CPU is measured in millicores. Disk includes retained server data.
type Resources struct {
	CPU       int `json:"cpu"`
	MemoryMiB int `json:"memoryMiB"`
	DiskMiB   int `json:"diskMiB"`
}

type Game struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Resources Resources `json:"resources"`
}

type Config struct {
	Release        string    `json:"release"`
	Workload       string    `json:"workload"`
	Games          []Game    `json:"games"`
	Capacity       Resources `json:"capacity"`
	MaxHosts       int       `json:"maxHosts"`
	MaxServers     int       `json:"maxServers"`
	LaunchTemplate string    `json:"launchTemplate"`
	LaunchVersion  string    `json:"launchVersion"`
	IdleMinutes    int       `json:"idleMinutes"`
	MaxMinutes     int       `json:"maxMinutes"`
	Deadline       int64     `json:"deadline"`
}

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var launchVersion = regexp.MustCompile(`^[1-9][0-9]*$`)

func (c Config) Validate() error {
	if c.MaxHosts < 1 || c.MaxHosts > 16 || c.MaxServers < 1 || c.MaxServers > 128 {
		return fmt.Errorf("fleet limits require 1–16 hosts and 1–128 saved servers")
	}
	if c.Capacity.CPU <= 0 || c.Capacity.MemoryMiB <= 0 || c.Capacity.DiskMiB <= 0 {
		return fmt.Errorf("host CPU, memory, and disk capacity must be positive")
	}
	if c.LaunchTemplate == "" || !launchVersion.MatchString(c.LaunchVersion) {
		return fmt.Errorf("fleet requires a launch template and a pinned numeric version")
	}
	if c.IdleMinutes < 0 || c.IdleMinutes > 10080 || c.MaxMinutes < 0 || c.MaxMinutes > 10080 || c.Deadline < 0 {
		return fmt.Errorf("invalid fleet runtime limits")
	}
	if len(c.Games) == 0 {
		return fmt.Errorf("fleet catalog is empty")
	}
	seen := make(map[string]bool, len(c.Games))
	for _, g := range c.Games {
		if !validID.MatchString(g.ID) || seen[g.ID] || strings.TrimSpace(g.Name) == "" || len(g.Name) > 80 {
			return fmt.Errorf("fleet catalog requires unique IDs and game names")
		}
		seen[g.ID] = true
		r := g.Resources
		if r.CPU <= 0 || r.MemoryMiB <= 0 || r.DiskMiB <= 0 || r.CPU > c.Capacity.CPU || r.MemoryMiB > c.Capacity.MemoryMiB || r.DiskMiB > c.Capacity.DiskMiB {
			return fmt.Errorf("game %s must have positive resource requirements that fit one host", g.ID)
		}
	}
	return nil
}

type State struct {
	Revision int64    `json:"revision"`
	Hosts    []Host   `json:"hosts"`
	Servers  []Server `json:"servers"`
	Deleting bool     `json:"deleting"`
}

type Host struct {
	Release        string    `json:"release"`
	Workload       string    `json:"workload"`
	Capacity       Resources `json:"capacity"`
	ID             string    `json:"id"`
	InstanceID     string    `json:"instanceId"`
	LaunchTemplate string    `json:"launchTemplate"`
	LaunchVersion  string    `json:"launchVersion"`
	LaunchToken    string    `json:"launchToken"`
	CreatedAt      int64     `json:"createdAt"`
	Retiring       bool      `json:"retiring"`
	Error          string    `json:"error,omitempty"`
}

type Server struct {
	ReadyAt     int64     `json:"readyAt,omitempty"`
	Error       string    `json:"error,omitempty"`
	Resources   Resources `json:"resources"`
	ID          string    `json:"id"`
	Game        string    `json:"game"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	Desired     string    `json:"desired"`
	RequestID   string    `json:"requestId"`
	RequestedAt int64     `json:"requestedAt"`
}

type Report struct {
	ID         string    `json:"id"`
	Host       string    `json:"host"`
	RequestID  string    `json:"requestId"`
	Phase      string    `json:"phase"`
	Desired    string    `json:"desired"`
	Address    string    `json:"address,omitempty"`
	Players    *int      `json:"players,omitempty"`
	Error      string    `json:"error,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

var ErrConflict = errors.New("fleet state changed; retry with current state")
var ErrReportOwner = errors.New("report belongs to another host")

type Store interface {
	Load(context.Context) (State, error)
	Save(context.Context, State, int64) error
	Reports(context.Context) ([]Report, error)
	Publish(context.Context, Report) error
	Tombstoned(context.Context, string) (bool, error)
	Forget(context.Context, State, int64, string) error
}
