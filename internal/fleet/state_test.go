package fleet

import "testing"

func TestConfigRejectsUnboundedOrUnschedulableFleet(t *testing.T) {
	valid := Config{Games: []Game{{ID: "quake", Name: "Quake", Resources: Resources{CPU: 100, MemoryMiB: 256, DiskMiB: 1024}}}, Capacity: Resources{CPU: 1000, MemoryMiB: 1536, DiskMiB: 16000}, MaxHosts: 2, MaxServers: 32, LaunchTemplate: "lt-example", LaunchVersion: "1", IdleMinutes: 10, MaxMinutes: 360}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"unbounded hosts", func(c *Config) { c.MaxHosts = 0 }},
		{"too many servers", func(c *Config) { c.MaxServers = 129 }},
		{"mutable launch version", func(c *Config) { c.LaunchVersion = "$Latest" }},
		{"insufficient disk", func(c *Config) { c.Capacity.DiskMiB = 512 }},
		{"duplicate game", func(c *Config) { c.Games = append(c.Games, c.Games[0]) }},
		{"no games", func(c *Config) { c.Games = nil }},
		{"negative idle", func(c *Config) { c.IdleMinutes = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			c.Games = append([]Game(nil), valid.Games...)
			tc.change(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
