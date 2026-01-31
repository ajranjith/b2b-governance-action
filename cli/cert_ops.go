package main

import (
	"crypto/ed25519"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

func generateVerifyCertificate(workspace string, result *VerifyResult) (string, bool, error) {
	cert := support.NewCertificate("verify")
	cert.Pass = result.Pass
	cert.Reason = result.Message
	cert.RedCount = result.RedCount
	cert.AmberCount = result.AmberCount
	cert.GreenCount = result.GreenCount
	cert.MaxRed = result.MaxRed
	cert.MaxAmber = result.MaxAmber
	cert.Policy = support.PolicyInfo{
		FailOnRed:  boolValue(result.FailOnRed, true),
		AllowAmber: boolValue(result.AllowAmber, false),
	}
	cert.EvidenceHashes = collectEvidenceHashes(workspace)

	priv, err := support.LoadSigningKey(workspace)
	if err != nil {
		cert.Signature = ""
		cert.SignatureMethod = ""
	} else if err := support.SignCertificate(&cert, priv); err != nil {
		return "", false, err
	}

	certPath := filepath.Join(workspace, ".b2b", "certificate.json")
	if err := support.WriteJSONAtomic(certPath, cert); err != nil {
		return "", false, err
	}

	verified := false
	if priv != nil && len(priv) > 0 {
		ok, err := support.VerifyCertificate(&cert, priv.Public().(ed25519.PublicKey))
		if err != nil {
			verified = false
		} else {
			verified = ok
		}
	}

	hash, _ := support.HashFile(certPath)
	return hash, verified, nil
}

func generateIngestCertificate(workspace string) (string, error) {
	cert := support.NewCertificate("ingest-admin")
	cert.Pass = true
	cert.Reason = "INGEST_COMPLETE"
	cert.RedCount = 0
	cert.AmberCount = 0
	cert.GreenCount = 0
	cert.Policy = support.PolicyInfo{FailOnRed: true, AllowAmber: false}
	cert.EvidenceHashes = collectEvidenceHashes(workspace)

	priv, err := support.LoadSigningKey(workspace)
	if err == nil {
		_ = support.SignCertificate(&cert, priv)
	}

	certPath := filepath.Join(workspace, ".b2b", "certificate.json")
	if err := support.WriteJSONAtomic(certPath, cert); err != nil {
		return "", err
	}
	hash, _ := support.HashFile(certPath)
	return hash, nil
}

func verifyCertificateFile(workspace, path string) (bool, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}
	var cert support.Certificate
	if err := json.Unmarshal(data, &cert); err != nil {
		return false, "", err
	}
	priv, err := support.LoadSigningKey(workspace)
	if err != nil {
		return false, "", err
	}
	pub := priv.Public().(ed25519.PublicKey)
	ok, err := support.VerifyCertificate(&cert, pub)
	hash := support.HashBytes(data)
	return ok, hash, err
}

func collectEvidenceHashes(workspace string) map[string]string {
	candidates := []string{
		".b2b/report.json",
		".b2b/report.html",
		".b2b/results.sarif",
		".b2b/junit.xml",
		".b2b/parity-report.json",
	}
	hashes := map[string]string{}
	for _, rel := range candidates {
		path := filepath.Join(workspace, rel)
		if _, err := os.Stat(path); err == nil {
			if h, err := support.HashFile(path); err == nil {
				hashes[rel] = h
			}
		}
	}
	return hashes
}

func updateReportSignature(workspace string, verified bool, certHash string) error {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	var rep report
	if data, err := os.ReadFile(reportPath); err == nil {
		if err := json.Unmarshal(data, &rep); err != nil {
			return err
		}
	}

	verifiedAt := time.Now().UTC().Format(time.RFC3339)
	rep.Rules = upsertRule(rep.Rules, ruleResult{
		RuleID:   "2.2.3",
		Severity: "high",
		Status:   boolStatus(verified),
		Evidence: map[string]interface{}{
			"signature_verified": verified,
			"certificate_hash":   certHash,
			"verified_at":        verifiedAt,
		},
		FixHint: "Regenerate and sign certificate with a valid key.",
	})

	certPath := filepath.Join(workspace, ".b2b", "certificate.json")
	cert, certErr := loadCertificate(certPath)
	certStatus := "PASS"
	certViolations := []finding{}
	if certErr != nil {
		certStatus = "FAIL"
		certViolations = append(certViolations, finding{File: certPath, Line: 1, Message: certErr.Error()})
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.1.1", certStatus, "high", map[string]interface{}{
		"certificatePath": certPath,
	}, certViolations, "Ensure verify writes a signed certificate."))

	sarifPath := filepath.Join(workspace, ".b2b", "results.sarif")
	sarifStatus, sarifViolations := validateSarif(sarifPath)
	rep.Rules = upsertRule(rep.Rules, makeRule("4.1.2", sarifStatus, "high", map[string]interface{}{
		"sarifPath": sarifPath,
	}, sarifViolations, "Ensure verify produces a valid SARIF file."))

	junitPath := filepath.Join(workspace, ".b2b", "junit.xml")
	junitStatus, junitViolations := validateJUnit(junitPath)
	rep.Rules = upsertRule(rep.Rules, makeRule("4.1.3", junitStatus, "high", map[string]interface{}{
		"junitPath": junitPath,
	}, junitViolations, "Ensure verify produces a valid JUnit file."))

	hashStatus, hashViolations, hashEvidence := validateCertificateHashes(cert, workspace)
	rep.Rules = upsertRule(rep.Rules, makeRule("4.1.5", hashStatus, "high", hashEvidence, hashViolations,
		"Ensure certificate includes hashes for SARIF and JUnit outputs."))

	rep.Phase2Status = phase2Status(rep.Rules)
	rep.Phase3Status = phase3Status(rep.Rules)
	rep.Phase4Status = phase4Status(rep.Rules)
	return support.WriteJSONAtomic(reportPath, rep)
}

func loadCertificate(path string) (*support.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cert support.Certificate
	if err := json.Unmarshal(data, &cert); err != nil {
		return nil, err
	}
	return &cert, nil
}

func validateSarif(path string) (string, []finding) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "FAIL", []finding{{File: path, Line: 1, Message: "sarif missing"}}
	}
	var doc sarifDocument
	if err := json.Unmarshal(data, &doc); err != nil || len(doc.Runs) == 0 {
		return "FAIL", []finding{{File: path, Line: 1, Message: "sarif invalid"}}
	}
	return "PASS", nil
}

func validateJUnit(path string) (string, []finding) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "FAIL", []finding{{File: path, Line: 1, Message: "junit missing"}}
	}
	var suites junitTestsuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		return "FAIL", []finding{{File: path, Line: 1, Message: "junit invalid"}}
	}
	if len(suites.Testsuites) == 0 {
		return "FAIL", []finding{{File: path, Line: 1, Message: "junit missing testsuite"}}
	}
	return "PASS", nil
}

func validateCertificateHashes(cert *support.Certificate, workspace string) (string, []finding, map[string]interface{}) {
	evidence := map[string]interface{}{}
	if cert == nil {
		return "FAIL", []finding{{File: filepath.Join(workspace, ".b2b", "certificate.json"), Line: 1, Message: "certificate missing"}}, evidence
	}
	required := []string{".b2b/results.sarif", ".b2b/junit.xml"}
	violations := []finding{}
	for _, rel := range required {
		expected, ok := cert.EvidenceHashes[rel]
		if !ok {
			violations = append(violations, finding{File: rel, Line: 1, Message: "missing hash in certificate"})
			continue
		}
		actual, err := support.HashFile(filepath.Join(workspace, rel))
		if err != nil {
			violations = append(violations, finding{File: rel, Line: 1, Message: "unable to hash evidence"})
			continue
		}
		evidence[rel] = map[string]interface{}{"expected": expected, "actual": actual}
		if actual != expected {
			violations = append(violations, finding{File: rel, Line: 1, Message: "certificate hash mismatch"})
		}
	}
	if len(violations) > 0 {
		return "FAIL", violations, evidence
	}
	return "PASS", nil, evidence
}

func boolStatus(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func boolValue(ptr *bool, def bool) bool {
	if ptr == nil {
		return def
	}
	return *ptr
}

func ensureCertExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return errors.New("certificate missing")
	}
	return nil
}

func runVerifyCert(path string) {
	workspace := config.Paths.WorkspaceRoot
	if err := ensureCertExists(path); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	ok, hash, err := verifyCertificateFile(workspace, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: verify failed: %v\n", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Printf("signature_verified=false certificate_hash=%s\n", hash)
		os.Exit(1)
	}
	fmt.Printf("signature_verified=true certificate_hash=%s\n", hash)
}
