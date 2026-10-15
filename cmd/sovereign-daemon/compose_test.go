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
		Volumes []string `yaml:"volumes"`
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
}
