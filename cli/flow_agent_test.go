package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajranjith/b2b-governance-action/cli/internal/flow"
)

func TestDetectAgentsCursorConfig(t *testing.T) {
	tmp := t.TempDir()
	appData := filepath.Join(tmp, "AppData", "Roaming")
	configPath := filepath.Join(appData, "Cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APPDATA", appData); err != nil {
		t.Fatal(err)
	}
	ctx := flow.Context{HomeDir: tmp}
	res := flow.DetectAgents(ctx)
	found := false
	for _, entry := range res.Entries {
		if entry.ClientName == "Cursor" {
			found = true
			if !entry.Installed {
				t.Fatalf("expected installed")
			}
			if entry.ConfigPath != configPath {
				t.Fatalf("unexpected configPath: %s", entry.ConfigPath)
			}
		}
	}
	if !found {
		t.Fatal("cursor not detected")
	}
}

func TestConnectAgentUpdatesConfig(t *testing.T) {
	tmp := t.TempDir()
	appData := filepath.Join(tmp, "AppData", "Roaming")
	configPath := filepath.Join(appData, "Cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{"other":{"command":"x"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APPDATA", appData); err != nil {
		t.Fatal(err)
	}

	ctx := flow.Context{Root: tmp, HomeDir: tmp, SkipSelftest: true}
	opts := flow.Options{Root: tmp, Clients: []string{"cursor"}, AgentBinaryPath: "C:\\bin\\gres-b2b.exe", AgentConfigPath: configPath}
	if _, err := flow.RunStep(ctx, flow.StepConnectAgents, opts, nil); err != nil {
		t.Fatalf("connect: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "gres-b2b") {
		t.Fatalf("expected gres-b2b entry")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".b2b", "agent-connect.log")); err != nil {
		t.Fatalf("agent-connect.log missing: %v", err)
	}
}

func TestClassifyGreenfieldScaffold(t *testing.T) {
	tmp := t.TempDir()
	ctx := flow.Context{Root: tmp, HomeDir: tmp}
	opts := flow.Options{Root: tmp, TargetPath: tmp, Mode: "greenfield"}
	if _, err := flow.RunStep(ctx, flow.StepSelectTarget, opts, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := flow.RunStep(ctx, flow.StepClassify, opts, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "main-index.json")); err != nil {
		t.Fatalf("registry missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "ui", "registry.json")); err != nil {
		t.Fatalf("ui registry missing: %v", err)
	}
	state, err := flow.LoadState(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if state.Mode != "greenfield" {
		b, _ := json.Marshal(state)
		t.Fatalf("expected mode greenfield: %s", string(b))
	}
}
