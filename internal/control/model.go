package control

import (
	"context"
	"encoding/json"
	"time"
)

type Game struct {
	ID                string                     `json:"id"`
	DisplayName       string                     `json:"displayName"`
	Protocol          string                     `json:"protocol"`
	Image             string                     `json:"image"`
	SidecarPort       int                        `json:"sidecarPort"`
	GamePort          int                        `json:"gamePort"`
	Ports             map[string]json.RawMessage `json:"ports"`
	GameCmd           string                     `json:"gameCmd"`
	GameArgs          string                     `json:"gameArgs"`
	GameQuitCmd       string                     `json:"gameQuitCmd"`
	GameQuitTimeout   int                        `json:"gameQuitTimeout"`
	ConfigPath        string                     `json:"configPath"`
	DataURLEnv        string                     `json:"dataUrlEnv"`
	DefaultConfigFile string                     `json:"defaultConfigFile,omitempty"`
	DefaultConfig     string                     `json:"defaultConfig"`
	ConfigEditor      json.RawMessage            `json:"configEditor,omitempty"`
	ClientDownloadURL string                     `json:"clientDownloadUrl,omitempty"`
}

type CreateRequest struct {
	ID          string  `json:"id"`
	Game        string  `json:"game"`
	Name        string  `json:"name"`
	Config      *string `json:"config,omitempty"`
	IdleMinutes int     `json:"idleMinutes"`
	MaxMinutes  int     `json:"maxMinutes"`
}

type Endpoint struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
}

type Instance struct {
	Generation   int64      `json:"generation"`
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Game         Game       `json:"game"`
	Provider     string     `json:"provider"`
	Desired      string     `json:"desired"`
	Phase        string     `json:"phase"`
	Config       string     `json:"config"`
	IdleMinutes  int        `json:"idleMinutes"`
	MaxMinutes   int        `json:"maxMinutes"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    time.Time  `json:"startedAt"`
	EmptySince   time.Time  `json:"emptySince"`
	Endpoints    []Endpoint `json:"endpoints"`
	Players      *int       `json:"players"`
	Map          string     `json:"map"`
	Error        string     `json:"error,omitempty"`
	StopReason   string     `json:"stopReason,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	AgentToken   string     `json:"-"`
	RconPassword string     `json:"-"`
}

// Credentials stay in the private database and never enter API responses.
type record struct {
	Instance
	AgentToken   string
	RconPassword string
}

func (i Instance) Connect() string {
	for _, e := range i.Endpoints {
		if e.Port == i.Game.GamePort {
			return e.Address
		}
	}
	return ""
}

type Observation struct {
	Started   bool
	Exists    bool
	Running   bool
	Endpoints []Endpoint
	AgentURL  string
	Failure   string
}

// Provider manages infrastructure only. The agent owns game-level operations.
type Provider interface {
	Ensure(context.Context, Instance) (Observation, error)
	Inspect(context.Context, Instance) (Observation, error)
	Start(context.Context, Instance) error
	Stop(context.Context, Instance) error
	Delete(context.Context, Instance) error
}
