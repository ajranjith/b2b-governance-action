package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
	"github.com/ajranjith/b2b-governance-action/cli/internal/flow"
)

func TestWizardFlowStateMachine(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join("testdata", "wizard_flow", "project")
	if err := copyDir(src, tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg
	configPath = ""

	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("CODEX_HOME", filepath.Join(tmp, ".codex"))

	if err := os.MkdirAll(filepath.Join(tmp, ".cursor"), 0o755); err != nil {
		t.Fatalf("mkdir .cursor: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".cursor", "mcp.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatalf("write cursor config: %v", err)
	}

	ctx := flow.Context{Root: tmp, HomeDir: tmp, ExePath: os.Args[0], SkipSelftest: true}
	opts := flow.Options{Root: tmp, AllClients: true, Mode: "brownfield", Action: flow.Action{Name: "scan"}}

	if _, err := flow.RunStep(ctx, flow.StepDetectAgents, opts, nil); err != nil {
		t.Fatalf("detect agents: %v", err)
	}

	setupPath := filepath.Join(tmp, ".b2b", "setup.json")
	if _, err := os.Stat(setupPath); err != nil {
		t.Fatalf("missing setup.json: %v", err)
	}

	if _, err := flow.RunStep(ctx, flow.StepConnectAgents, opts, nil); err != nil {
		t.Fatalf("connect agents: %v", err)
	}

	state := readSetupState(t, setupPath)
	if state.Step != "mode_select" {
		t.Fatalf("expected step=mode_select, got %s", state.Step)
	}

	if _, err := flow.RunStep(ctx, flow.StepClassify, opts, nil); err != nil {
		t.Fatalf("classify: %v", err)
	}

	state = readSetupState(t, setupPath)
	if state.Step != "target_select" {
		t.Fatalf("expected step=target_select, got %s", state.Step)
	}

	if _, err := flow.RunStep(ctx, flow.StepScan, opts, flowRunner{}); err == nil {
		t.Fatal("expected scan to fail without target")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".b2b", "report.json")); err == nil {
		t.Fatal("report.json should not exist without target")
	}

	opts.TargetPath = tmp
	if _, err := flow.RunStep(ctx, flow.StepSelectTarget, opts, flowRunner{}); err != nil {
		t.Fatalf("select target: %v", err)
	}
	if _, err := flow.RunStep(ctx, flow.StepScan, opts, flowRunner{}); err != nil {
		t.Fatalf("scan: %v", err)
	}

	assertFile(t, filepath.Join(tmp, ".b2b", "report.json"))
	assertFile(t, filepath.Join(tmp, ".b2b", "report.html"))
}

func readSetupState(t *testing.T, path string) flow.State {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read setup.json: %v", err)
	}
	var state flow.State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("parse setup.json: %v", err)
	}
	return state
}
