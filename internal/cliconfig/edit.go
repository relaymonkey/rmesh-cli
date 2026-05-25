package cliconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const minimalAgentConfigTemplate = `agent_id: local

transport:
  url: serial:/dev/ttyUSB0

mqtt:
  broker_url: mqtt://mqtt.relaymonkey.com:1883
  username: ""
  password: ""
  topic_prefix: ""

labels: {}

synthesise:
  nodedb_poll: 5m
  nodeinfo:
    enabled: true
    interval: 6h
    on_first_seen: true
  position:
    enabled: true
    interval: 30m
    on_first_seen: true
  mapreport:
    enabled: true
    interval: 6h
`

// EnsureAgentConfigFile creates path and parent dirs with a minimal template when missing.
func EnsureAgentConfigFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(minimalAgentConfigTemplate), 0o600); err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	return nil
}

// EditAgentConfig opens path in $EDITOR / $VISUAL (fallback: nano).
func EditAgentConfig(path string) error {
	if err := EnsureAgentConfigFile(path); err != nil {
		return err
	}
	editor := Editor()
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", editor, err)
	}
	return nil
}
