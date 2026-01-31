package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
)

func TestPhase2_IngestPass(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase2", "ingest_pass"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	if _, err := ingestAdmin("", "", false); err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	incoming := filepath.Join(tmp, "incoming")
	locked := filepath.Join(tmp, "locked")
	if entries, _ := os.ReadDir(incoming); len(entries) != 0 {
		t.Fatal("incoming should be empty after ingest")
	}
	if entries, _ := os.ReadDir(locked); len(entries) == 0 {
		t.Fatal("locked should have files after ingest")
	}

	statePath := filepath.Join(tmp, ".b2b", "ingest.state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("missing ingest.state.json: %v", err)
	}

	auditPath := filepath.Join(tmp, ".b2b", "audit.log")
	if _, err := os.Stat(auditPath); err != nil {
		t.Fatalf("missing audit.log: %v", err)
	}
}

func TestPhase2_IngestCrashResume(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase2", "ingest_crash_resume"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	os.Setenv("GRES_INGEST_FAIL_AFTER", "0")
	_, err := ingestAdmin("", "", false)
	os.Unsetenv("GRES_INGEST_FAIL_AFTER")
	if err == nil {
		t.Fatal("expected ingest to fail for crash simulation")
	}

	if _, err := ingestAdmin("", "", true); err != nil {
		t.Fatalf("resume ingest failed: %v", err)
	}
}

func TestPhase2_CertTamper(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase2", "cert_tamper"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	os.Setenv("GRES_SIGNING_PRIVATE_KEY", base64.StdEncoding.EncodeToString(priv))
	defer os.Unsetenv("GRES_SIGNING_PRIVATE_KEY")

	result := &VerifyResult{Pass: true, Message: "PASSED", RedCount: 0, AmberCount: 0, GreenCount: 0}
	if _, _, err := generateVerifyCertificate(tmp, result); err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	certPath := filepath.Join(tmp, ".b2b", "certificate.json")
	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	data[10] ^= 0xFF
	if err := os.WriteFile(certPath, data, 0o644); err != nil {
		t.Fatalf("tamper cert: %v", err)
	}

	ok, _, err := verifyCertificateFile(tmp, certPath)
	if err != nil {
		t.Fatalf("verify cert: %v", err)
	}
	if ok {
		t.Fatal("expected tampered cert to fail verification")
	}
}

func TestPhase2_Caps(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase2", "caps"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg, err := loadVerifyConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	results, err := loadScanResults(tmp)
	if err != nil {
		t.Fatal(err)
	}

	res := evaluateGating(cfg, len(results.Red), len(results.Amber), len(results.Green))
	if res.Pass {
		t.Fatal("expected caps to fail verify")
	}
	if !strings.Contains(res.Message, "max_amber") {
		t.Fatalf("expected failure due to max_amber, got %s", res.Message)
	}
}

func TestPhase2_PolicyPrecedence(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase2", "policy_precedence"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg, err := loadVerifyConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	results, err := loadScanResults(tmp)
	if err != nil {
		t.Fatal(err)
	}

	res := evaluateGating(cfg, len(results.Red), len(results.Amber), len(results.Green))
	if res.Pass {
		t.Fatal("expected fail due to caps precedence")
	}
	if !strings.Contains(res.Message, "max_red") {
		t.Fatalf("expected primary reason max_red, got %s", res.Message)
	}
}

func TestPhase2_NakedMutation(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase2", "naked_mutation"), tmp); err != nil {
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

	status := map[string]string{}
	for _, r := range rep.Rules {
		status[r.RuleID] = r.Status
	}
	if status["2.4.2"] != "FAIL" {
		t.Fatalf("expected 2.4.2 FAIL, got %s", status["2.4.2"])
	}
	if status["2.4.1"] != "PASS" {
		t.Fatalf("expected 2.4.1 PASS, got %s", status["2.4.1"])
	}
}

func TestPhase2_AuditAppend(t *testing.T) {
	tmp := t.TempDir()
	if err := copyDir(filepath.Join("testdata", "phase2", "naked_mutation"), tmp); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cfg := cfgpkg.Default()
	cfg.Paths.WorkspaceRoot = tmp
	config = &cfg

	runScan()
	runScan()

	auditPath := filepath.Join(tmp, ".b2b", "audit.log")
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected audit log to append entries, got %d", len(lines))
	}
}
