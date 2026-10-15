package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type ComposeConfig struct {
	Services map[string]struct {
		Volumes  []string `yaml:"volumes"`
		Ports    []string `yaml:"ports"`
		Networks []string `yaml:"networks"`
	} `yaml:"services"`
}

func findComposeFile() (string, error) {
	// Check standard locations relative to test directory
	candidates := []string{
		"../../docker-compose.yml",
		"../docker-compose.yml",
		"docker-compose.yml",
	}

	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs, nil
			}
		}
	}

	return "", os.ErrNotExist
}

func TestDockerCompose_SocketIsolationBoundary(t *testing.T) {
	composePath, err := findComposeFile()
	if err != nil {
		t.Fatalf("could not locate docker-compose.yml: %v", err)
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml at %s: %v", composePath, err)
	}

	var compose ComposeConfig
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("failed to parse docker-compose.yml: %v", err)
	}

	daemonService, daemonOk := compose.Services["sovereign-daemon"]
	if !daemonOk {
		t.Fatalf("sovereign-daemon service missing in docker-compose.yml")
	}

	// Assert /var/run/docker.sock is mounted in sovereign-daemon
	daemonHasSocket := false
	for _, v := range daemonService.Volumes {
		if strings.Contains(v, "/var/run/docker.sock") {
			daemonHasSocket = true
			break
		}
	}
	if !daemonHasSocket {
		t.Errorf("expected sovereign-daemon to mount /var/run/docker.sock, found volumes: %v", daemonService.Volumes)
	}

	webService, webOk := compose.Services["sovereign-web"]
	if !webOk {
		t.Fatalf("sovereign-web service missing in docker-compose.yml")
	}

	// Assert /var/run/docker.sock is absent from sovereign-web
	for _, v := range webService.Volumes {
		if strings.Contains(v, "docker.sock") {
			t.Errorf("security violation: sovereign-web must NEVER mount docker.sock, found volume: %s", v)
		}
	}

	// Assert caddy service exists
	caddyService, caddyOk := compose.Services["caddy"]
	if !caddyOk {
		t.Fatalf("caddy service missing in docker-compose.yml")
	}

	// Assert /var/run/docker.sock is absent from caddy
	for _, v := range caddyService.Volumes {
		if strings.Contains(v, "docker.sock") {
			t.Errorf("security violation: caddy must NEVER mount docker.sock, found volume: %s", v)
		}
	}

	// Assert Caddy port bindings include 80 and 443
	hasPort80 := false
	hasPort443 := false
	for _, p := range caddyService.Ports {
		if strings.Contains(p, "80:80") || strings.HasSuffix(p, ":80") || p == "80:80" {
			hasPort80 = true
		}
		if strings.Contains(p, "443:443") || strings.HasSuffix(p, ":443") || p == "443:443" {
			hasPort443 = true
		}
	}
	if !hasPort80 {
		t.Errorf("expected caddy to expose port 80, found ports: %v", caddyService.Ports)
	}
	if !hasPort443 {
		t.Errorf("expected caddy to expose port 443, found ports: %v", caddyService.Ports)
	}

	// Assert Caddy networks connect to sovereign-ingress and sovereign-control, but not sovereign-ai
	hasIngressNet := false
	hasControlNet := false
	for _, net := range caddyService.Networks {
		if net == "sovereign-ingress" {
			hasIngressNet = true
		}
		if net == "sovereign-control" {
			hasControlNet = true
		}
		if net == "sovereign-ai" {
			t.Errorf("security violation: caddy must NEVER connect to internal sovereign-ai network")
		}
	}
	if !hasIngressNet || !hasControlNet {
		t.Errorf("caddy must connect to sovereign-ingress and sovereign-control networks, found: %v", caddyService.Networks)
	}

	// Assert upstream container ports (:3000, :8080) are restricted to localhost/127.0.0.1
	for _, p := range webService.Ports {
		if !strings.HasPrefix(p, "127.0.0.1:") {
			t.Errorf("security violation: sovereign-web port %s must be bound to 127.0.0.1, not exposed to 0.0.0.0", p)
		}
	}
	for _, p := range daemonService.Ports {
		if !strings.HasPrefix(p, "127.0.0.1:") {
			t.Errorf("security violation: sovereign-daemon port %s must be bound to 127.0.0.1, not exposed to 0.0.0.0", p)
		}
	}
}
