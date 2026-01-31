package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
	"gopkg.in/yaml.v3"
)

type shadowVector struct {
	Name       string `yaml:"name"`
	ExpectPass bool   `yaml:"expectPass"`
}

type parityReport struct {
	TotalVectors  int          `json:"totalVectors"`
	PassedVectors int          `json:"passedVectors"`
	FailedVectors int          `json:"failedVectors"`
	PassRatePct   float64      `json:"passRatePct"`
	DiffCount     int          `json:"diffCount"`
	Confidence    string       `json:"confidence"`
	Mismatches    []parityDiff `json:"mismatches"`
}

type parityDiff struct {
	Vector     string `json:"vector"`
	ExpectPass bool   `json:"expectPass"`
	ActualPass bool   `json:"actualPass"`
	Reason     string `json:"reason,omitempty"`
}

func runShadow(vectorsPath, repoRoot string) {
	if err := runShadowInternal(vectorsPath, repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runShadowInternal(vectorsPath, repoRoot string) error {
	if repoRoot == "" {
		repoRoot = config.Paths.WorkspaceRoot
	}
	loadScanOverrides(repoRoot)
	vectors, err := loadVectors(vectorsPath)
	if err != nil {
		return err
	}

	total := len(vectors)
	passed := 0
	failed := 0
	mismatches := []parityDiff{}

	for _, v := range vectors {
		actualPass, reason := evalRepoPass(repoRoot)
		if actualPass == v.ExpectPass {
			passed++
		} else {
			failed++
			mismatches = append(mismatches, parityDiff{
				Vector:     v.Name,
				ExpectPass: v.ExpectPass,
				ActualPass: actualPass,
				Reason:     reason,
			})
		}
	}

	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total) * 100
	}
	confidence := "LOW"
	if passRate == 100 && len(mismatches) == 0 {
		confidence = "HIGH"
	} else if passRate >= 95 && len(mismatches) <= 1 {
		confidence = "MEDIUM"
	}

	report := parityReport{
		TotalVectors:  total,
		PassedVectors: passed,
		FailedVectors: failed,
		PassRatePct:   passRate,
		DiffCount:     len(mismatches),
		Confidence:    confidence,
		Mismatches:    mismatches,
	}

	if err := support.WriteJSONAtomic(filepath.Join(repoRoot, ".b2b", "parity-report.json"), report); err != nil {
		return err
	}

	updateShadowReport(repoRoot, report)

	if config.Scan.ShadowFailOnMismatch && len(mismatches) > 0 {
		return fmt.Errorf("shadow parity mismatch")
	}
	return nil
}

func loadVectors(path string) ([]shadowVector, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var vectors []shadowVector
	if err := yaml.Unmarshal(support.StripBOM(data), &vectors); err != nil {
		return nil, err
	}
	return vectors, nil
}

func evalRepoPass(repoRoot string) (bool, string) {
	cfg, err := loadVerifyConfig(repoRoot)
	if err != nil {
		return false, "config error"
	}
	results, err := loadScanResults(repoRoot)
	if err != nil {
		return false, "results missing"
	}
	res := evaluateGating(cfg, len(results.Red), len(results.Amber), len(results.Green))
	return res.Pass, res.Message
}

func updateShadowReport(workspace string, parity parityReport) {
	path := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		return
	}

	rep.Rules = upsertRule(rep.Rules, makeRule("4.3.1", "PASS", "medium", map[string]interface{}{
		"vectorsLoaded": parity.TotalVectors,
	}, nil, ""))

	rep.Rules = upsertRule(rep.Rules, makeRule("4.3.2", "PASS", "medium", map[string]interface{}{
		"parityReportPath": filepath.Join(workspace, ".b2b", "parity-report.json"),
	}, nil, ""))

	rep.Rules = upsertRule(rep.Rules, makeRule("4.3.3", "PASS", "medium",
		map[string]interface{}{
			"totalVectors":  parity.TotalVectors,
			"passedVectors": parity.PassedVectors,
			"failedVectors": parity.FailedVectors,
			"passRatePct":   parity.PassRatePct,
			"diffCount":     parity.DiffCount,
			"confidence":    parity.Confidence,
		}, nil, ""))

	status := "PASS"
	severity := "high"
	if parity.DiffCount > 0 {
		if config.Scan.ShadowFailOnMismatch {
			status = "FAIL"
		} else {
			status = "WARN"
			severity = "medium"
		}
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.3.4", status, severity,
		map[string]interface{}{"shadow_fail_on_mismatch": config.Scan.ShadowFailOnMismatch}, nil, ""))

	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(path, rep)
}
