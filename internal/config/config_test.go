package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/config"
)

func TestLoadValidatesLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
transport:
  url: serial:/dev/ttyUSB0
mqtt:
  broker_url: mqtt://localhost:1883
  username: user
  password: secret
  topic_prefix: rm/n/abcd1234
labels:
  relaymesh.bad: x
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected reserved prefix error")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
transport:
  url: serial:/dev/ttyUSB0
mqtt:
  broker_url: mqtt://localhost:1883
  username: user
  password: secret
  topic_prefix: rm/n/abcd1234
labels:
  site: oslo-east
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentID != "local" {
		t.Fatalf("agent_id = %q", cfg.AgentID)
	}
	if !cfg.Synthesise.NodeInfo.Enabled {
		t.Fatal("expected nodeinfo synthesis enabled by default")
	}
}
