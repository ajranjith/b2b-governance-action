package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

func classifyProject(ctx Context, opts Options, state *State, root string) error {
	if opts.Mode != "" {
		state.Mode = opts.Mode
	}
	if state.Mode == "" {
		return fmt.Errorf("mode is required: greenfield or brownfield")
	}
	if state.Mode != "greenfield" && state.Mode != "brownfield" {
		return fmt.Errorf("invalid mode: %s", state.Mode)
	}

	workspace := state.Target.WorkspaceRoot
	if workspace == "" {
		workspace = state.Target.Path
	}
	if workspace == "" {
		workspace = rootFor(ctx, opts)
	}

	if state.Mode == "greenfield" {
		return scaffoldGreenfield(workspace)
	}
	return nil
}

func scaffoldGreenfield(workspace string) error {
	if err := os.MkdirAll(filepath.Join(workspace, ".b2b"), 0o755); err != nil {
		return err
	}
	if err := ensureRegistry(workspace); err != nil {
		return err
	}
	if err := ensureUIRegistry(workspace); err != nil {
		return err
	}
	return nil
}

func ensureRegistry(workspace string) error {
	candidates := []string{"main-index.json", filepath.Join(".b2b", "main-index.json"), filepath.Join("registry", "main-index.json")}
	for _, rel := range candidates {
		path := filepath.Join(workspace, rel)
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	path := filepath.Join(workspace, "main-index.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := map[string]interface{}{
		"version": "1.0",
		"modules": []interface{}{},
		"ids": map[string]interface{}{
			"API": map[string]interface{}{},
			"SVC": map[string]interface{}{},
			"DB":  map[string]interface{}{},
		},
	}
	return support.WriteJSONAtomic(path, content)
}

func ensureUIRegistry(workspace string) error {
	path := filepath.Join(workspace, "ui", "registry.json")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return support.WriteJSONAtomic(path, map[string]interface{}{})
}

func runScanStep(ctx Context, opts Options, state *State, runner Runner) error {
	if state.Action.Name == "" {
		state.Action = opts.Action
	}
	if state.Action.Name == "" {
		return fmt.Errorf("action is required")
	}
	if runner == nil {
		return nil
	}
	if err := writeChecklist(state, opts); err != nil {
		return err
	}
	return runner.Run(state.Action, resolveTarget(ctx, opts, state))
}

func runFixLoop(ctx Context, opts Options, state *State, runner Runner) error {
	if runner == nil {
		return nil
	}
	if state.Action.Name != "fix-loop" {
		return nil
	}
	attempts := opts.MaxFixAttempts
	if attempts <= 0 {
		attempts = 3
	}
	root := resolveTarget(ctx, opts, state)
	for i := 0; i < attempts; i++ {
		_ = runner.Run(Action{Name: "fix", FixDryRun: true}, root)
		_ = runner.Run(Action{Name: "fix", FixDryRun: false}, root)
		_ = runner.Run(Action{Name: "scan"}, root)
		_ = runner.Run(Action{Name: "verify"}, root)
		if pass, _ := reportPass(root); pass {
			return nil
		}
	}
	return fmt.Errorf("fix loop exceeded max attempts")
}

func runFinalVerify(ctx Context, opts Options, state *State, runner Runner) error {
	if runner == nil {
		return nil
	}
	if state.Action.Name == "verify" || state.Action.Name == "fix" || state.Action.Name == "fix-loop" {
		if err := runner.Run(Action{Name: "verify"}, resolveTarget(ctx, opts, state)); err != nil {
			return err
		}
	}
	return createShortcut(state, opts)
}

func reportPass(root string) (bool, error) {
	path := filepath.Join(root, ".b2b", "report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var rep struct {
		Phase1Status string `json:"phase1Status"`
		Phase2Status string `json:"phase2Status"`
		Phase3Status string `json:"phase3Status"`
		Phase4Status string `json:"phase4Status"`
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		return false, err
	}
	return rep.Phase1Status == "PASS" && rep.Phase2Status == "PASS" && rep.Phase3Status == "PASS" && rep.Phase4Status == "PASS", nil
}

func resolveTarget(ctx Context, opts Options, state *State) string {
	if state.Target.WorkspaceRoot != "" {
		return state.Target.WorkspaceRoot
	}
	if state.Target.Path != "" {
		return state.Target.Path
	}
	return rootFor(ctx, opts)
}

func writeChecklist(state *State, opts Options) error {
	root := resolveTarget(Context{}, opts, state)
	if root == "" {
		return nil
	}
	checklist := []string{
		"Boundary rules (contracts-only / no-leak)",
		"BFF mandatory + wrapper enforcement + policy call",
		"Registry correctness + ID namespaces",
		"Atomic ingestion checks",
		"Evidence signing checks",
		"UI registry coverage",
		"Dealer LLID contract + UI LLID display enforcement",
		"Phase 1-4 rule groups",
	}
	path := filepath.Join(root, ".b2b", "checklist.json")
	return support.WriteJSONAtomic(path, map[string]interface{}{
		"generatedAtUtc": time.Now().UTC().Format(time.RFC3339),
		"items":          checklist,
	})
}

func createShortcut(state *State, opts Options) error {
	root := resolveTarget(Context{}, opts, state)
	if root == "" {
		return nil
	}
	if runtime.GOOS != "windows" {
		return nil
	}
	hud := filepath.Join(root, ".b2b", "report.html")
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	if desktop == "\\" || desktop == "" {
		return nil
	}
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		return err
	}
	shortcutPath := filepath.Join(desktop, "GRES Governance HUD (v4.0.0).url")
	content := fmt.Sprintf("[InternetShortcut]\nURL=file:///%s\n", filepath.ToSlash(hud))
	if err := support.WriteFileAtomic(shortcutPath, []byte(content)); err != nil {
		return err
	}
	return support.WriteJSONAtomic(filepath.Join(root, ".b2b", "shortcut.json"), map[string]interface{}{
		"createdAtUtc": time.Now().UTC().Format(time.RFC3339),
		"path":         shortcutPath,
		"target":       hud,
	})
}
