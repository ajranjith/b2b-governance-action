package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
	"github.com/ajranjith/b2b-governance-action/cli/internal/flow"
)

func TestPhase4VerifyOutputs(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "verify", "pass"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg
	configPath = ""

	runVerify()

	assertFile(t, filepath.Join(tmp, ".b2b", "certificate.json"))
	assertFile(t, filepath.Join(tmp, ".b2b", "results.sarif"))
	assertFile(t, filepath.Join(tmp, ".b2b", "junit.xml"))

	data, err := os.ReadFile(filepath.Join(tmp, ".b2b", "report.json"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("parse report: %v", err)
	}

	if ruleStatus(rep, "4.1.1") != "PASS" {
		t.Fatalf("expected 4.1.1 PASS")
	}
	if ruleStatus(rep, "4.1.2") != "PASS" {
		t.Fatalf("expected 4.1.2 PASS")
	}
	if ruleStatus(rep, "4.1.3") != "PASS" {
		t.Fatalf("expected 4.1.3 PASS")
	}
	if ruleStatus(rep, "4.1.5") != "PASS" {
		t.Fatalf("expected 4.1.5 PASS")
	}
}

func TestPhase4FailurePayloads(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "verify", "fail_payload"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	runScan()

	rep := loadReport(t, tmp)
	for _, r := range rep.Rules {
		for _, v := range r.Violations {
			if v.RuleID == "" || v.File == "" || v.Message == "" || v.FixHint == "" || v.Line <= 0 {
				t.Fatalf("invalid violation payload: %+v", v)
			}
		}
	}
}

func TestPhase4WatchBasic(t *testing.T) {
	t.Setenv("GRES_NO_EXIT", "1")
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "watch", "basic"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	b2bDir := filepath.Join(tmp, ".b2b")
	if err := os.MkdirAll(b2bDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(b2bDir, "results.json"), []byte(`{"red":[],"amber":[],"green":[]}`), 0o644)

	stop := make(chan struct{})
	go runWatchWithStop(tmp, stop)

	target := filepath.Join(tmp, "ui", "page.ts")
	_ = os.WriteFile(target, []byte("export const x = 1\n"), 0o644)

	if !waitForFile(filepath.Join(tmp, ".b2b", "report.json"), 5*time.Second) {
		close(stop)
		t.Fatal("report.json not updated")
	}
	if !waitForFile(filepath.Join(tmp, ".b2b", "report.html"), 5*time.Second) {
		close(stop)
		t.Fatal("report.html not updated")
	}
	if !waitForFile(filepath.Join(tmp, ".b2b", "hints.json"), 5*time.Second) {
		close(stop)
		t.Fatal("hints.json not updated")
	}
	if !waitForFile(filepath.Join(tmp, ".b2b", "history"), 5*time.Second) {
		close(stop)
		t.Fatal("history not created")
	}
	close(stop)
}

func TestPhase4HistoryRotation(t *testing.T) {
	tmp := t.TempDir()
	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	cfg.Scan.HistoryMaxSnapshots = 3
	cfg.Scan.HistoryKeepDays = 14
	config = &cfg

	b2bDir := filepath.Join(tmp, ".b2b")
	if err := os.MkdirAll(b2bDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(b2bDir, "report.json"), []byte(`{"rules":[]}`), 0o644)

	for i := 0; i < 5; i++ {
		writeHistorySnapshot(tmp)
		time.Sleep(1100 * time.Millisecond)
	}

	historyDir := filepath.Join(tmp, ".b2b", "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 3 {
		t.Fatalf("expected at most 3 snapshots, got %d", len(entries))
	}
}

func TestPhase4ShadowConfidenceFields(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "shadow", "mismatch"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	cfg.Scan.ShadowFailOnMismatch = false
	config = &cfg

	if err := runShadowInternal(filepath.Join(tmp, "vectors.yml"), tmp); err != nil {
		t.Fatalf("shadow failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".b2b", "parity-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rep parityReport
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.TotalVectors == 0 {
		t.Fatal("expected total vectors > 0")
	}
	if rep.Confidence == "" {
		t.Fatal("expected confidence field")
	}
}

func TestPhase4ShadowFailOnMismatch(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "shadow", "mismatch"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	cfg.Scan.ShadowFailOnMismatch = true
	config = &cfg

	if err := runShadowInternal(filepath.Join(tmp, "vectors.yml"), tmp); err == nil {
		t.Fatal("expected mismatch error when shadow_fail_on_mismatch=true")
	}
}

func TestPhase4FixModes(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "fix", "structural"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	handler := filepath.Join(tmp, "bff", "handler.ts")
	before, _ := os.ReadFile(handler)

	if err := runFixInternal(true); err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	assertFile(t, filepath.Join(tmp, ".b2b", "fix-plan.json"))
	assertFile(t, filepath.Join(tmp, ".b2b", "fix.patch"))

	afterDry, _ := os.ReadFile(handler)
	if string(before) != string(afterDry) {
		t.Fatal("dry-run should not modify source files")
	}

	if err := runFixInternal(false); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	assertFile(t, filepath.Join(tmp, ".b2b", "fix-apply.json"))

	after, _ := os.ReadFile(handler)
	if string(before) == string(after) {
		t.Fatal("apply should modify source files")
	}
}

func TestPhase4FixSemanticGuard(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "fix", "semantic_block"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	err := runFixInternal(false)
	if err == nil {
		t.Fatal("expected semantic guard error")
	}

	rep := loadReport(t, tmp)
	if ruleStatus(rep, "4.4.2") != "FAIL" {
		t.Fatal("expected 4.4.2 FAIL")
	}
}

func TestPhase4ExternalGatewayForbidden(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "external_gateway_forbidden"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	runScan()

	rep := loadReport(t, tmp)
	if ruleStatus(rep, "4.5.1") != "FAIL" {
		t.Fatal("expected 4.5.1 FAIL")
	}
	if ruleStatus(rep, "4.5.2") != "FAIL" {
		t.Fatal("expected 4.5.2 FAIL")
	}
}

func TestPhase4DoctorSchema(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase4", "doctor", "degraded"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	loadScanOverrides(tmp)
	report := buildDoctorReport()
	if report.Status != "DEGRADED" {
		t.Fatalf("expected DEGRADED, got %s", report.Status)
	}
	if report.Registry.RequiredNamespaces == nil {
		t.Fatal("expected requiredNamespaces")
	}
}

func TestPhase4SupportBundle(t *testing.T) {
	tmp := t.TempDir()
	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	b2bDir := filepath.Join(tmp, ".b2b")
	_ = os.MkdirAll(b2bDir, 0o755)
	_ = os.WriteFile(filepath.Join(b2bDir, "report.json"), []byte(`{"rules":[]}`), 0o644)

	runSupportBundle(tmp)

	matches, _ := filepath.Glob(filepath.Join(b2bDir, "support-bundle_*.zip"))
	if len(matches) == 0 {
		t.Fatal("support bundle zip not created")
	}

	zr, err := zip.OpenReader(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	found := false
	for _, f := range zr.File {
		if f.Name == ".b2b/report.json" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("report.json not found in support bundle")
	}
}

func TestPhase4SetupResume(t *testing.T) {
	tmp := t.TempDir()
	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	b2bDir := filepath.Join(tmp, ".b2b")
	_ = os.MkdirAll(b2bDir, 0o755)
	_ = os.WriteFile(filepath.Join(b2bDir, "report.json"), []byte(`{"rules":[]}`), 0o644)

	t.Setenv("GRES_SKIP_SELFTEST", "1")
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)

	ctx := flow.Context{Root: tmp, HomeDir: tmp, SkipSelftest: true}
	opts := flow.Options{Root: tmp, TargetPath: tmp, AllClients: true, ForceAction: true, Mode: "brownfield", Action: flow.Action{Name: "doctor"}}
	_, _ = flow.Run(ctx, opts, nil)
	_, _ = flow.Run(ctx, opts, nil)

	data, err := os.ReadFile(filepath.Join(b2bDir, "setup.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state flow.State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.CurrentStep == "" {
		t.Fatal("expected setup to advance")
	}
}

func loadReport(t *testing.T, root string) report {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".b2b", "report.json"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("parse report: %v", err)
	}
	return rep
}

func ruleStatus(rep report, ruleID string) string {
	for _, r := range rep.Rules {
		if r.RuleID == ruleID {
			return r.Status
		}
	}
	return ""
}

func assertFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing file: %s", path)
	}
}

func waitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
