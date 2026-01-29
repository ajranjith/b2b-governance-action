package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper constructors
// ---------------------------------------------------------------------------

func boolP(b bool) *bool { return &b }
func intP(n int) *int    { return &n }

// ---------------------------------------------------------------------------
// Unit tests for evaluateGating
// ---------------------------------------------------------------------------

func TestGating_MaxRedExceeded(t *testing.T) {
	// max_red: 1, redCount=2 → FAIL
	cfg := &VerifyConfig{MaxRed: intP(1), FailOnRed: boolP(false), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 2, 0, 5)
	if r.Pass {
		t.Fatal("expected FAIL when redCount (2) > max_red (1)")
	}
	assertContains(t, r.Message, "exceeded max_red")
}

func TestGating_MaxRedEqual(t *testing.T) {
	// max_red: 2, redCount=2 → PASS (equal is within cap)
	cfg := &VerifyConfig{MaxRed: intP(2), FailOnRed: boolP(false), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 2, 0, 5)
	if !r.Pass {
		t.Fatalf("expected PASS when redCount (2) == max_red (2), got reasons: %v", r.Reasons)
	}
	assertContains(t, r.Message, "PASSED")
}

func TestGating_MaxAmberExceeded(t *testing.T) {
	// max_amber: 3, amberCount=4 → FAIL
	cfg := &VerifyConfig{MaxAmber: intP(3), FailOnRed: boolP(false), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 0, 4, 5)
	if r.Pass {
		t.Fatal("expected FAIL when amberCount (4) > max_amber (3)")
	}
	assertContains(t, r.Message, "exceeded max_amber")
}

func TestGating_AllowAmberFalse_CapsUnset(t *testing.T) {
	// allow_amber: false, amberCount=1, caps unset → FAIL
	cfg := &VerifyConfig{AllowAmber: boolP(false), FailOnRed: boolP(false)}
	r := evaluateGating(cfg, 0, 1, 5)
	if r.Pass {
		t.Fatal("expected FAIL when allow_amber=false and amberCount > 0")
	}
	assertContains(t, r.Message, "allow_amber disabled")
}

func TestGating_FailOnRedTrue_CapsUnset(t *testing.T) {
	// fail_on_red: true, redCount=1, caps unset → FAIL
	cfg := &VerifyConfig{FailOnRed: boolP(true), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 1, 0, 5)
	if r.Pass {
		t.Fatal("expected FAIL when fail_on_red=true and redCount > 0")
	}
	assertContains(t, r.Message, "fail_on_red enabled")
}

func TestGating_MaxRedWithFailOnRed_BothApply(t *testing.T) {
	// max_red: 5 with fail_on_red: true, redCount=3
	// Both rules trigger: caps pass (3 <= 5), but boolean fails (3 > 0)
	// Priority: caps do NOT override fail_on_red
	cfg := &VerifyConfig{MaxRed: intP(5), FailOnRed: boolP(true), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 3, 0, 5)
	if r.Pass {
		t.Fatal("expected FAIL: fail_on_red=true should still fail even when within max_red cap")
	}
	assertContains(t, r.Message, "fail_on_red enabled")
}

func TestGating_MaxRedWithFailOnRedFalse(t *testing.T) {
	// max_red: 5, fail_on_red: false, redCount=3 → PASS
	// Caps allow tolerable debt when boolean is disabled
	cfg := &VerifyConfig{MaxRed: intP(5), FailOnRed: boolP(false), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 3, 0, 5)
	if !r.Pass {
		t.Fatalf("expected PASS: fail_on_red=false and redCount (3) <= max_red (5), got reasons: %v", r.Reasons)
	}
	assertContains(t, r.Message, "PASSED")
	assertContains(t, r.Message, "max_red=5")
}

func TestGating_DefaultsNoConfig(t *testing.T) {
	// Empty config → defaults: fail_on_red=true, allow_amber=false
	cfg := &VerifyConfig{}

	// No violations → PASS
	r := evaluateGating(cfg, 0, 0, 5)
	if !r.Pass {
		t.Fatal("expected PASS with zero violations and default config")
	}

	// 1 red → FAIL (default fail_on_red=true)
	r = evaluateGating(cfg, 1, 0, 5)
	if r.Pass {
		t.Fatal("expected FAIL: default fail_on_red=true and redCount > 0")
	}

	// 0 red, 1 amber → FAIL (default allow_amber=false)
	r = evaluateGating(cfg, 0, 1, 5)
	if r.Pass {
		t.Fatal("expected FAIL: default allow_amber=false and amberCount > 0")
	}
}

func TestGating_PassMessage_WithCaps(t *testing.T) {
	cfg := &VerifyConfig{MaxRed: intP(5), MaxAmber: intP(10), FailOnRed: boolP(false), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 2, 3, 10)
	if !r.Pass {
		t.Fatalf("expected PASS, got: %v", r.Reasons)
	}
	assertContains(t, r.Message, "RED=2 (max_red=5)")
	assertContains(t, r.Message, "AMBER=3 (max_amber=10)")
}

func TestGating_PassMessage_NoCaps(t *testing.T) {
	cfg := &VerifyConfig{FailOnRed: boolP(false), AllowAmber: boolP(true)}
	r := evaluateGating(cfg, 0, 0, 8)
	if !r.Pass {
		t.Fatal("expected PASS")
	}
	assertContains(t, r.Message, "RED=0, AMBER=0, GREEN=8")
}

// ---------------------------------------------------------------------------
// YAML config parsing
// ---------------------------------------------------------------------------

func TestParseVerifyConfigYAML_Full(t *testing.T) {
	yaml := `
# Gating config
fail_on_red: true
allow_amber: false
max_red: 0
max_amber: 5
`
	cfg, err := parseVerifyConfigYAML([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOnRed == nil || *cfg.FailOnRed != true {
		t.Fatal("expected fail_on_red=true")
	}
	if cfg.AllowAmber == nil || *cfg.AllowAmber != false {
		t.Fatal("expected allow_amber=false")
	}
	if cfg.MaxRed == nil || *cfg.MaxRed != 0 {
		t.Fatal("expected max_red=0")
	}
	if cfg.MaxAmber == nil || *cfg.MaxAmber != 5 {
		t.Fatal("expected max_amber=5")
	}
}

func TestParseVerifyConfigYAML_Empty(t *testing.T) {
	cfg, err := parseVerifyConfigYAML([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOnRed != nil || cfg.AllowAmber != nil || cfg.MaxRed != nil || cfg.MaxAmber != nil {
		t.Fatal("expected all nil for empty config")
	}
}

func TestParseVerifyConfigYAML_Partial(t *testing.T) {
	yaml := `max_red: 3`
	cfg, err := parseVerifyConfigYAML([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxRed == nil || *cfg.MaxRed != 3 {
		t.Fatal("expected max_red=3")
	}
	if cfg.FailOnRed != nil {
		t.Fatal("expected fail_on_red nil when not specified")
	}
}

// ---------------------------------------------------------------------------
// Integration: verify writes correct certificate
// ---------------------------------------------------------------------------

func TestVerify_CertificateOutput(t *testing.T) {
	tmpDir := t.TempDir()
	b2bDir := filepath.Join(tmpDir, ".b2b")
	if err := os.MkdirAll(b2bDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write config.yml
	configYAML := "fail_on_red: true\nallow_amber: false\nmax_red: 1\n"
	if err := os.WriteFile(filepath.Join(b2bDir, "config.yml"), []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write results.json with 2 reds (exceeds max_red=1)
	results := ScanResults{
		Red: []Violation{
			{Rule: "SEC-001", Message: "SQL injection", File: "db.go", Line: 42},
			{Rule: "SEC-002", Message: "XSS vulnerability", File: "web.go", Line: 10},
		},
		Amber: []Violation{},
		Green: []Violation{
			{Rule: "STYLE-001", Message: "Clean code"},
		},
	}
	data, _ := json.Marshal(results)
	if err := os.WriteFile(filepath.Join(b2bDir, "results.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Load config and results
	cfg, err := loadVerifyConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	scanResults, err := loadScanResults(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Evaluate and write certificate
	result := evaluateGating(cfg, len(scanResults.Red), len(scanResults.Amber), len(scanResults.Green))
	if err := writeCertificate(tmpDir, result); err != nil {
		t.Fatal(err)
	}

	// Read and verify certificate
	certData, err := os.ReadFile(filepath.Join(b2bDir, "certificate.json"))
	if err != nil {
		t.Fatal(err)
	}

	var cert VerifyResult
	if err := json.Unmarshal(certData, &cert); err != nil {
		t.Fatal(err)
	}

	if cert.Pass {
		t.Fatal("certificate should show FAIL for redCount=2 > max_red=1")
	}
	if cert.RedCount != 2 {
		t.Fatalf("expected redCount=2, got %d", cert.RedCount)
	}
	if cert.MaxRed == nil || *cert.MaxRed != 1 {
		t.Fatal("expected max_red=1 in certificate")
	}
	if cert.FailOnRed == nil || *cert.FailOnRed != true {
		t.Fatal("expected fail_on_red=true in certificate")
	}
}

func TestVerify_SARIFOutput(t *testing.T) {
	tmpDir := t.TempDir()
	b2bDir := filepath.Join(tmpDir, ".b2b")
	os.MkdirAll(b2bDir, 0o755)

	scanResults := &ScanResults{
		Red:   []Violation{{Rule: "SEC-001", Message: "issue", File: "a.go", Line: 1}},
		Amber: []Violation{{Rule: "WARN-001", Message: "warning", File: "b.go", Line: 5}},
	}
	verifyResult := &VerifyResult{Pass: false, Message: "FAILED: test"}

	if err := writeSARIF(tmpDir, scanResults, verifyResult); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(b2bDir, "results.sarif"))
	if err != nil {
		t.Fatal(err)
	}

	var doc sarifDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(doc.Runs))
	}
	// 1 red + 1 amber + 1 gate = 3 results
	if len(doc.Runs[0].Results) != 3 {
		t.Fatalf("expected 3 SARIF results, got %d", len(doc.Runs[0].Results))
	}
}

func TestVerify_JUnitOutput(t *testing.T) {
	tmpDir := t.TempDir()
	b2bDir := filepath.Join(tmpDir, ".b2b")
	os.MkdirAll(b2bDir, 0o755)

	scanResults := &ScanResults{
		Red:   []Violation{{Rule: "SEC-001", Message: "issue"}},
		Amber: []Violation{},
		Green: []Violation{{Rule: "OK-001", Message: "good"}},
	}
	verifyResult := &VerifyResult{Pass: false, Message: "FAILED: test"}

	if err := writeJUnit(tmpDir, scanResults, verifyResult); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(b2bDir, "junit.xml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !containsStr(content, `<?xml version="1.0"`) {
		t.Fatal("missing XML header")
	}
	if !containsStr(content, `name="governance-verify"`) {
		t.Fatal("missing testsuite name")
	}
	if !containsStr(content, `name="governance-gate"`) {
		t.Fatal("missing gate testcase")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !containsStr(s, substr) {
		t.Fatalf("expected %q to contain %q", s, substr)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s, substr))
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
