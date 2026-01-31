package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
)

func TestPhase1Fixtures(t *testing.T) {
	cases := []struct {
		name       string
		failRules  []string
		phase1Fail bool
	}{
		{name: "pass", failRules: nil, phase1Fail: false},
		{name: "fail_namespace", failRules: []string{"1.1.3"}, phase1Fail: true},
		{name: "fail_bff", failRules: []string{"1.2.1", "1.2.4"}, phase1Fail: true},
		{name: "fail_wrapper", failRules: []string{"1.2.2", "1.2.4"}, phase1Fail: true},
		{name: "fail_permission", failRules: []string{"1.2.3", "1.2.4"}, phase1Fail: true},
		{name: "fail_structure", failRules: []string{"1.3.1"}, phase1Fail: true},
		{name: "fail_kernel", failRules: []string{"1.3.2"}, phase1Fail: true},
		{name: "fail_boot", failRules: []string{"1.3.3"}, phase1Fail: true},
		{name: "fail_noleak", failRules: []string{"1.3.4", "1.3.5"}, phase1Fail: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join("testdata", "phase1", tc.name)
			if err := copyDir(src, tmp); err != nil {
				t.Fatalf("copy fixture: %v", err)
			}

			cfg := cfgpkg.Default()
			cfg.Paths.WorkspaceRoot = tmp
			config = &cfg
			configPath = ""

			runScan()

			reportPath := filepath.Join(tmp, ".b2b", "report.json")
			if _, err := os.Stat(reportPath); err != nil {
				t.Fatalf("missing report.json: %v", err)
			}

			impactPath := filepath.Join(tmp, ".b2b", "impact-graph.json")
			if _, err := os.Stat(impactPath); err != nil {
				t.Fatalf("missing impact-graph.json: %v", err)
			}

			doctorPath := filepath.Join(tmp, ".b2b", "doctor.json")
			if _, err := os.Stat(doctorPath); err != nil {
				t.Fatalf("missing doctor.json: %v", err)
			}

			data, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatalf("read report.json: %v", err)
			}
			var rep report
			if err := json.Unmarshal(data, &rep); err != nil {
				t.Fatalf("parse report.json: %v", err)
			}

			ruleMap := map[string]string{}
			for _, r := range rep.Rules {
				ruleMap[r.RuleID] = r.Status
			}

			for _, rule := range tc.failRules {
				if ruleMap[rule] != "FAIL" {
					t.Fatalf("expected rule %s to FAIL, got %s", rule, ruleMap[rule])
				}
			}

			if tc.phase1Fail && rep.Phase1Status != "FAIL" {
				t.Fatalf("expected phase1Status FAIL, got %s", rep.Phase1Status)
			}
			if !tc.phase1Fail && rep.Phase1Status != "PASS" {
				t.Fatalf("expected phase1Status PASS, got %s", rep.Phase1Status)
			}
		})
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
