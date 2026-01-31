package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

const oldWording = "HUD will turn RED if the template attempts to bypass your security wrappers to fetch data directly from /locked files."
const newWording = "HUD will turn RED if the template performs a DB mutation without the mandatory audit wrapper/marker (Naked Mutation)."

func TestCopyEnforcementWording(t *testing.T) {
	repoRoot := filepath.Join("..")
	files := []string{
		filepath.Join(repoRoot, "docs", "ARCHITECTURE.md"),
		filepath.Join(repoRoot, "docs", "AGENT.md"),
		filepath.Join(repoRoot, "RELEASE_NOTES.md"),
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "docs", "index.html"),
		filepath.Join(repoRoot, "docs", "cli", "index.html"),
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing required doc: %s", path)
		}
		content := string(data)
		if containsStr(content, oldWording) {
			t.Fatalf("old wording still present in %s", path)
		}
		if !containsStr(content, newWording) {
			t.Fatalf("new wording missing in %s", path)
		}
	}

	// HUD template content
	tmp := t.TempDir()
	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	if err := os.MkdirAll(filepath.Join(tmp, ".b2b"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tmp, ".b2b", "report.json"), []byte(`{"rules":[]}`), 0o644)
	writeReportHTML(tmp)

	data, err := os.ReadFile(filepath.Join(tmp, ".b2b", "report.html"))
	if err != nil {
		t.Fatalf("missing report.html: %v", err)
	}
	content := string(data)
	if containsStr(content, oldWording) {
		t.Fatal("old wording still present in HUD template")
	}
	if !containsStr(content, newWording) {
		t.Fatal("new wording missing from HUD template")
	}
}

func TestPhaseHeadlessGhostRoutes(t *testing.T) {
	cases := []struct {
		name        string
		ruleID      string
		expectState string
	}{
		{name: "pass", ruleID: "API_ROUTE_ID_REQUIRED", expectState: "PASS"},
		{name: "fail_missing_api_id", ruleID: "API_ROUTE_ID_REQUIRED", expectState: "FAIL"},
		{name: "fail_unknown_api_id", ruleID: "API_ROUTE_ID_UNKNOWN", expectState: "FAIL"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join("testdata", "headless", "ghost_routes", tc.name)
			if err := copyDir(src, tmp); err != nil {
				t.Fatalf("copy fixture: %v", err)
			}
			cfg := cfgpkg.Default()
			cfg.Paths.WorkspaceRoot = tmp
			config = &cfg

			runScan()

			assertFile(t, filepath.Join(tmp, ".b2b", "api-routes.json"))
			rep := loadReport(t, tmp)
			if ruleStatus(rep, tc.ruleID) != tc.expectState {
				t.Fatalf("expected %s to be %s", tc.ruleID, tc.expectState)
			}

			// Ensure violations include required fields when failing
			for _, r := range rep.Rules {
				if r.RuleID == tc.ruleID && r.Status == "FAIL" {
					for _, v := range r.Violations {
						if v.RuleID == "" || v.File == "" || v.Message == "" || v.FixHint == "" || v.Line <= 0 {
							t.Fatalf("invalid violation payload: %+v", v)
						}
					}
				}
			}
		})
	}
}

func TestPhaseHeadlessRollback(t *testing.T) {
	tmp := t.TempDir()
	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	if err := os.MkdirAll(filepath.Join(tmp, ".b2b"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tmp, ".b2b", "results.json"), []byte(`{"red":[],"amber":[],"green":[]}`), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, ".b2b", "report.json"), []byte(`{"rules":[]}`), 0o644)
	_ = os.MkdirAll(filepath.Join(tmp, "ui"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, "ui", "registry.json"), []byte(`{"dealer:/orders":{"svcId":"SVC-1"}}`), 0o644)

	runVerify()

	backupIDs := listBackupSnapshots(tmp)
	if len(backupIDs) == 0 {
		t.Fatal("expected backup snapshot")
	}
	latest := backupIDs[len(backupIDs)-1]
	snapshotDir := filepath.Join(tmp, ".b2b", "backups", latest)

	// Corrupt files
	_ = os.WriteFile(filepath.Join(tmp, ".b2b", "report.json"), []byte(`{"rules":[{"ruleId":"BAD"}]}`), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "ui", "registry.json"), []byte(`{"bad":true}`), 0o644)

	runRollbackLatest()

	// Verify restored UI registry
	origUI := filepath.Join(snapshotDir, "ui", "registry.json")
	restUI := filepath.Join(tmp, "ui", "registry.json")
	origUIHash, _ := support.HashFile(origUI)
	restUIHash, _ := support.HashFile(restUI)
	if origUIHash != restUIHash {
		t.Fatal("ui/registry.json did not restore from snapshot")
	}

	assertFile(t, filepath.Join(tmp, ".b2b", "rollback.log"))
	assertFile(t, filepath.Join(tmp, ".b2b", "audit.log"))

	rep := loadReport(t, tmp)
	if ruleStatus(rep, "ROLLBACK_READY") == "" {
		t.Fatal("expected rollback evidence in report")
	}
}

func TestPhaseHeadlessDealerUiLlidDisplay(t *testing.T) {
	cases := []struct {
		name        string
		expectState string
	}{
		{name: "pass", expectState: "PASS"},
		{name: "fail_missing_llid_display", expectState: "FAIL"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join("testdata", "headless", "ui_llid_display", tc.name)
			if err := copyDir(src, tmp); err != nil {
				t.Fatalf("copy fixture: %v", err)
			}
			cfg := cfgpkg.Default()
			cfg.Paths.WorkspaceRoot = tmp
			config = &cfg

			runScan()
			rep := loadReport(t, tmp)
			if ruleStatus(rep, "DEALER_UI_LLID_DISPLAY_REQUIRED") != tc.expectState {
				t.Fatalf("expected dealer LLID display to be %s", tc.expectState)
			}
		})
	}
}

func TestRollbackReadyRule(t *testing.T) {
	tmp := t.TempDir()
	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	if err := os.MkdirAll(filepath.Join(tmp, ".b2b"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tmp, ".b2b", "report.json"), []byte(`{"rules":[]}`), 0o644)
	runScan()
	if ruleStatus(loadReport(t, tmp), "ROLLBACK_READY") != "FAIL" {
		t.Fatal("expected rollback ready FAIL without backups")
	}

	// Create a backup snapshot by writing report/cert
	_ = os.WriteFile(filepath.Join(tmp, ".b2b", "results.json"), []byte(`{"red":[],"amber":[],"green":[]}`), 0o644)
	runVerify()
	// Allow snapshot creation
	time.Sleep(50 * time.Millisecond)

	runScan()
	if ruleStatus(loadReport(t, tmp), "ROLLBACK_READY") != "PASS" {
		t.Fatal("expected rollback ready PASS with backups")
	}
}
