package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Server.ListenAddr != ":8800" {
		t.Errorf("ListenAddr = %q, want :8800", cfg.Server.ListenAddr)
	}
	if cfg.IdleReaper.RawInterval != "1m" {
		t.Errorf("IdleReaper.RawInterval = %q, want 1m", cfg.IdleReaper.RawInterval)
	}
	if cfg.IdleReaper.Interval != time.Minute {
		t.Errorf("IdleReaper.Interval = %v, want 1m", cfg.IdleReaper.Interval)
	}
	if cfg.Services == nil {
		t.Error("Services should be non-nil empty slice")
	}
	if len(cfg.Services) != 0 {
		t.Errorf("Services len = %d, want 0", len(cfg.Services))
	}
}

func TestLoad(t *testing.T) {
	t.Run("creates default when file missing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent", "config.yaml")
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if cfg.Server.ListenAddr != ":8800" {
			t.Errorf("default ListenAddr = %q, want :8800", cfg.Server.ListenAddr)
		}
	})

	t.Run("loads valid yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		yaml := `server:
  listen_addr: ":9999"
idle_reaper:
  interval: 2m
services:
  - name: web
    host: web.example.com
    containers:
      - web-1
    target_url: http://localhost:8080
    mode: on_demand
    idle_timeout: 5m
    startup_timeout: 10s
`
		if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if cfg.Server.ListenAddr != ":9999" {
			t.Errorf("ListenAddr = %q, want :9999", cfg.Server.ListenAddr)
		}
		if cfg.IdleReaper.Interval != 2*time.Minute {
			t.Errorf("IdleReaper.Interval = %v, want 2m", cfg.IdleReaper.Interval)
		}
		if len(cfg.Services) != 1 {
			t.Fatalf("Services len = %d, want 1", len(cfg.Services))
		}
		svc := cfg.Services[0]
		if svc.Name != "web" {
			t.Errorf("Name = %q, want web", svc.Name)
		}
		if svc.Host != "web.example.com" {
			t.Errorf("Host = %q, want web.example.com", svc.Host)
		}
		if len(svc.Containers) != 1 || svc.Containers[0] != "web-1" {
			t.Errorf("Containers = %v, want [web-1]", svc.Containers)
		}
		if svc.IdleTimeout != 5*time.Minute {
			t.Errorf("IdleTimeout = %v, want 5m", svc.IdleTimeout)
		}
		if svc.StartupTimeout != 10*time.Second {
			t.Errorf("StartupTimeout = %v, want 10s", svc.StartupTimeout)
		}
	})

	t.Run("invalid yaml returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte("not: yaml: [invalid"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if err == nil {
			t.Error("expected error for invalid yaml")
		}
	})

	t.Run("expands container_name to containers", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		yaml := `services:
  - name: web
    container_name: my-container
    mode: on_demand
    host: web.example.com
    target_url: http://localhost:8080
`
		if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if len(cfg.Services) != 1 {
			t.Fatalf("Services len = %d, want 1", len(cfg.Services))
		}
		if len(cfg.Services[0].Containers) != 1 || cfg.Services[0].Containers[0] != "my-container" {
			t.Errorf("Containers = %v, want [my-container]", cfg.Services[0].Containers)
		}
	})

	t.Run("defaults missing fields", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		yaml := `services:
  - name: web
    host: web.example.com
    target_url: http://localhost:8080
`
		if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		svc := cfg.Services[0]
		if svc.Mode != "on_demand" {
			t.Errorf("Mode = %q, want on_demand", svc.Mode)
		}
		if svc.IdleTimeout != 0 {
			t.Errorf("IdleTimeout = %v, want 0", svc.IdleTimeout)
		}
		if svc.StartupTimeout != 30*time.Second {
			t.Errorf("StartupTimeout = %v, want 30s", svc.StartupTimeout)
		}
	})
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := defaultConfig()
	cfg.Services = []ServiceConfig{
		{
			Name:              "web",
			Host:              "web.example.com",
			Containers:        []string{"web-1"},
			TargetURL:         "http://localhost:8080",
			Mode:              "on_demand",
			RawIdleTimeout:    "5m",
			IdleTimeout:       5 * time.Minute,
			RawStartupTimeout: "10s",
			StartupTimeout:    10 * time.Second,
		},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if len(data) == 0 {
		t.Error("saved file is empty")
	}
}

func TestCountLeadingSpaces(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 0},
		{"  hello", 2},
		{"    hello", 4},
		{"\thello", 0},
	}
	for _, tt := range tests {
		got := countLeadingSpaces(tt.input)
		if got != tt.want {
			t.Errorf("countLeadingSpaces(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := defaultConfig()
	original.Server.ListenAddr = ":7777"
	original.IdleReaper.RawInterval = "3m"
	original.IdleReaper.Interval = 3 * time.Minute
	original.Services = []ServiceConfig{
		{
			Name:              "svc1",
			Host:              "svc1.example.com",
			Containers:        []string{"c1"},
			TargetURL:         "http://localhost:8081",
			Mode:              "schedule_only",
			RawIdleTimeout:    "10m",
			IdleTimeout:       10 * time.Minute,
			RawStartupTimeout: "45s",
			StartupTimeout:    45 * time.Second,
		},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.Server.ListenAddr != original.Server.ListenAddr {
		t.Errorf("ListenAddr: got %q, want %q", loaded.Server.ListenAddr, original.Server.ListenAddr)
	}
	if loaded.IdleReaper.Interval != original.IdleReaper.Interval {
		t.Errorf("IdleReaper.Interval: got %v, want %v", loaded.IdleReaper.Interval, original.IdleReaper.Interval)
	}
	if len(loaded.Services) != 1 {
		t.Fatalf("Services len = %d, want 1", len(loaded.Services))
	}
	svc := loaded.Services[0]
	if svc.Name != "svc1" {
		t.Errorf("Name = %q, want svc1", svc.Name)
	}
	if svc.Host != "svc1.example.com" {
		t.Errorf("Host = %q, want svc1.example.com", svc.Host)
	}
	if len(svc.Containers) != 1 || svc.Containers[0] != "c1" {
		t.Errorf("Containers = %v, want [c1]", svc.Containers)
	}
	if svc.Mode != "schedule_only" {
		t.Errorf("Mode = %q, want schedule_only", svc.Mode)
	}
	if svc.IdleTimeout != 10*time.Minute {
		t.Errorf("IdleTimeout = %v, want 10m", svc.IdleTimeout)
	}
	if svc.StartupTimeout != 45*time.Second {
		t.Errorf("StartupTimeout = %v, want 45s", svc.StartupTimeout)
	}
}
