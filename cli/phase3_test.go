package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
)

func TestPhase3Fixtures(t *testing.T) {
	cases := []struct {
		name        string
		expectRule  string
		expectState string
	}{
		{name: "pass", expectRule: "3.1.1", expectState: "PASS"},
		{name: "fail_contract_version", expectRule: "3.1.4", expectState: "FAIL"},
		{name: "fail_dealer_bff_missing", expectRule: "3.2.1", expectState: "FAIL"},
		{name: "fail_admin_bff_missing", expectRule: "3.2.2", expectState: "FAIL"},
		{name: "fail_contract_spec_missing", expectRule: "3.2.3", expectState: "FAIL"},
		{name: "fail_ui_prisma", expectRule: "3.2.4", expectState: "FAIL"},
		{name: "fail_ui_no_envelope", expectRule: "3.2.5", expectState: "FAIL"},
		{name: "fail_ui_registry_coverage", expectRule: "3.3.3", expectState: "FAIL"},
		{name: "fail_ui_registry_coverage_warn", expectRule: "3.3.3", expectState: "WARN"},
		{name: "fail_dealer_llid", expectRule: "3.4.1", expectState: "FAIL"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join("testdata", "phase3", tc.name)
			if err := copyDir(src, tmp); err != nil {
				t.Fatalf("copy fixture: %v", err)
			}

			cfg := cfgpkg.Default()
			cfg.Paths.WorkspaceRoot = tmp
			config = &cfg

			runScan()

			reportPath := filepath.Join(tmp, ".b2b", "report.json")
			data, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatalf("read report: %v", err)
			}
			var rep report
			if err := json.Unmarshal(data, &rep); err != nil {
				t.Fatalf("parse report: %v", err)
			}

			status := ""
			var coverageFound bool
			for _, r := range rep.Rules {
				if r.RuleID == tc.expectRule {
					status = r.Status
				}
				if r.RuleID == "3.3.4" {
					if r.Evidence != nil {
						if _, ok := r.Evidence["uiRegistryCoveragePct"]; ok {
							coverageFound = true
						}
					}
				}
			}
			if status != tc.expectState {
				t.Fatalf("expected %s to be %s, got %s", tc.expectRule, tc.expectState, status)
			}
			if !coverageFound {
				t.Fatal("expected 3.3.4 coverage evidence")
			}
		})
	}
}
