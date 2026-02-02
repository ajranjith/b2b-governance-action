package main

import (
	"os"
	"path/filepath"
	"testing"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
	"github.com/ajranjith/b2b-governance-action/cli/internal/flow"
	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type testRunner struct {
	calls []flow.Action
}

func (r *testRunner) Run(action flow.Action, targetRoot string) error {
	r.calls = append(r.calls, action)
	_ = os.MkdirAll(filepath.Join(targetRoot, ".b2b"), 0o755)
	_ = support.WriteJSONAtomic(filepath.Join(targetRoot, ".b2b", "report.json"), map[string]interface{}{"rules": []interface{}{}})
	return nil
}

func TestFlowWizardCliParity(t *testing.T) {
	tmp1 := t.TempDir()
	tmp2 := t.TempDir()

	t.Setenv("GRES_SKIP_SELFTEST", "1")
	t.Setenv("USERPROFILE", tmp1)
	t.Setenv("HOME", tmp1)

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp1
	config = &cfg

	target1 := filepath.Join(tmp1, "repo")
	target2 := filepath.Join(tmp2, "repo")
	_ = os.MkdirAll(target1, 0o755)
	_ = os.MkdirAll(target2, 0o755)

	opts1 := flow.Options{
		Root:        tmp1,
		TargetPath:  target1,
		Action:      flow.Action{Name: "doctor"},
		Mode:        "brownfield",
		AllClients:  true,
		ForceAction: true,
	}

	ctx1 := flow.Context{Root: tmp1, HomeDir: tmp1, SkipSelftest: true}
	runner1 := &testRunner{}
	for _, step := range []flow.StepID{
		flow.StepSelectTarget,
		flow.StepDetectAgents,
		flow.StepConnectAgents,
		flow.StepValidateAgents,
		flow.StepClassify,
		flow.StepScan,
		flow.StepFixLoop,
		flow.StepFinalVerify,
	} {
		if _, err := flow.RunStep(ctx1, step, opts1, runner1); err != nil {
			t.Fatalf("run step %s: %v", step, err)
		}
	}

	state1, err := flow.LoadState(tmp1)
	if err != nil {
		t.Fatalf("load state1: %v", err)
	}

	t.Setenv("USERPROFILE", tmp2)
	t.Setenv("HOME", tmp2)

	cfg2 := cfgpkg.Default()
	cfg2.Paths.WorkspaceRoot = tmp2
	config = &cfg2

	opts2 := flow.Options{
		Root:        tmp2,
		TargetPath:  target2,
		Action:      flow.Action{Name: "doctor"},
		Mode:        "brownfield",
		AllClients:  true,
		ForceAction: true,
	}
	ctx2 := flow.Context{Root: tmp2, HomeDir: tmp2, SkipSelftest: true}
	runner2 := &testRunner{}
	if _, err := flow.Run(ctx2, opts2, runner2); err != nil {
		t.Fatalf("run flow: %v", err)
	}

	state2, err := flow.LoadState(tmp2)
	if err != nil {
		t.Fatalf("load state2: %v", err)
	}

	if state1.CurrentStep != state2.CurrentStep {
		t.Fatalf("expected same step; got %s vs %s", state1.CurrentStep, state2.CurrentStep)
	}
	if state1.Action.Name != state2.Action.Name {
		t.Fatalf("expected same action; got %s vs %s", state1.Action.Name, state2.Action.Name)
	}

	if _, err := os.Stat(filepath.Join(tmp1, ".b2b", "agent-detect.json")); err != nil {
		t.Fatalf("agent-detect.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp2, ".b2b", "agent-detect.json")); err != nil {
		t.Fatalf("agent-detect.json missing: %v", err)
	}
}
