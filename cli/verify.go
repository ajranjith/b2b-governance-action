package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
	"gopkg.in/yaml.v3"
)

// VerifyConfig represents .b2b/config.yml gating configuration.
// Pointer fields distinguish "unset" from zero values.
type VerifyConfig struct {
	FailOnRed  *bool `json:"fail_on_red,omitempty" yaml:"fail_on_red"`
	AllowAmber *bool `json:"allow_amber,omitempty" yaml:"allow_amber"`
	MaxRed     *int  `json:"max_red,omitempty" yaml:"max_red"`
	MaxAmber   *int  `json:"max_amber,omitempty" yaml:"max_amber"`
}

// Violation represents a single governance finding.
type Violation struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// ScanResults represents the output of a governance scan.
type ScanResults struct {
	Red   []Violation `json:"red"`
	Amber []Violation `json:"amber"`
	Green []Violation `json:"green"`
}

// VerifyResult is the output of the gating decision.
type VerifyResult struct {
	Pass       bool     `json:"pass"`
	Message    string   `json:"message"`
	RedCount   int      `json:"redCount"`
	AmberCount int      `json:"amberCount"`
	GreenCount int      `json:"greenCount"`
	FailOnRed  *bool    `json:"fail_on_red,omitempty"`
	AllowAmber *bool    `json:"allow_amber,omitempty"`
	MaxRed     *int     `json:"max_red,omitempty"`
	MaxAmber   *int     `json:"max_amber,omitempty"`
	Reasons    []string `json:"reasons,omitempty"`
	Timestamp  string   `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Config loading (.b2b/config.yml) - YAML parser
// ---------------------------------------------------------------------------

func loadVerifyConfig(workspaceRoot string) (*VerifyConfig, error) {
	configPath := filepath.Join(workspaceRoot, ".b2b", "config.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &VerifyConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read verify config %s: %w", configPath, err)
	}
	return parseVerifyConfigYAML(support.StripBOM(data))
}

// parseVerifyConfigYAML parses .b2b/config.yml using YAML v3.
func parseVerifyConfigYAML(data []byte) (*VerifyConfig, error) {
	cfg := &VerifyConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse verify config: %w", err)
	}
	return cfg, nil
}

// ---------------------------------------------------------------------------
// Scan results loading
// ---------------------------------------------------------------------------

func loadScanResults(workspaceRoot string) (*ScanResults, error) {
	resultsPath := filepath.Join(workspaceRoot, ".b2b", "results.json")
	data, err := os.ReadFile(resultsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read scan results %s: %w", resultsPath, err)
	}
	var results ScanResults
	if err := json.Unmarshal(support.StripBOM(data), &results); err != nil {
		return nil, fmt.Errorf("failed to parse scan results: %w", err)
	}
	return &results, nil
}

// ---------------------------------------------------------------------------
// Gating logic
// ---------------------------------------------------------------------------

// evaluateGating applies numeric caps then boolean rules and returns the verdict.
func evaluateGating(cfg *VerifyConfig, redCount, amberCount, greenCount int) *VerifyResult {
	result := &VerifyResult{
		Pass:       true,
		RedCount:   redCount,
		AmberCount: amberCount,
		GreenCount: greenCount,
		FailOnRed:  cfg.FailOnRed,
		AllowAmber: cfg.AllowAmber,
		MaxRed:     cfg.MaxRed,
		MaxAmber:   cfg.MaxAmber,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	var reasons []string
	var primary string

	// Effective boolean defaults
	failOnRed := true
	if cfg.FailOnRed != nil {
		failOnRed = *cfg.FailOnRed
	}
	allowAmber := false
	if cfg.AllowAmber != nil {
		allowAmber = *cfg.AllowAmber
	}

	// 1. Numeric caps (checked first per spec)
	if cfg.MaxRed != nil && redCount > *cfg.MaxRed {
		result.Pass = false
		msg := fmt.Sprintf("FAILED: RED violations (%d) exceeded max_red (%d)", redCount, *cfg.MaxRed)
		reasons = append(reasons, msg)
		if primary == "" {
			primary = msg
		}
	}
	if cfg.MaxAmber != nil && amberCount > *cfg.MaxAmber {
		result.Pass = false
		msg := fmt.Sprintf("FAILED: AMBER violations (%d) exceeded max_amber (%d)", amberCount, *cfg.MaxAmber)
		reasons = append(reasons, msg)
		if primary == "" {
			primary = msg
		}
	}

	// 2. Boolean rules
	if failOnRed && redCount > 0 {
		result.Pass = false
		msg := fmt.Sprintf("FAILED: %d RED violations detected (fail_on_red enabled)", redCount)
		reasons = append(reasons, msg)
		if primary == "" {
			primary = msg
		}
	}
	if !allowAmber && amberCount > 0 {
		result.Pass = false
		msg := fmt.Sprintf("FAILED: %d AMBER violations detected (allow_amber disabled)", amberCount)
		reasons = append(reasons, msg)
		if primary == "" {
			primary = msg
		}
	}

	result.Reasons = reasons

	// Build message
	if result.Pass {
		if cfg.MaxRed != nil || cfg.MaxAmber != nil {
			var parts []string
			if cfg.MaxRed != nil {
				parts = append(parts, fmt.Sprintf("RED=%d (max_red=%d)", redCount, *cfg.MaxRed))
			} else {
				parts = append(parts, fmt.Sprintf("RED=%d", redCount))
			}
			if cfg.MaxAmber != nil {
				parts = append(parts, fmt.Sprintf("AMBER=%d (max_amber=%d)", amberCount, *cfg.MaxAmber))
			} else {
				parts = append(parts, fmt.Sprintf("AMBER=%d", amberCount))
			}
			result.Message = fmt.Sprintf("PASSED: %s", strings.Join(parts, ", "))
		} else {
			result.Message = fmt.Sprintf("PASSED: RED=%d, AMBER=%d, GREEN=%d", redCount, amberCount, greenCount)
		}
	} else {
		if primary != "" {
			result.Message = primary
		} else {
			result.Message = strings.Join(reasons, "; ")
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// Output writers
// ---------------------------------------------------------------------------

// printHUD prints a human-readable summary to stdout.
func printHUD(result *VerifyResult) {
	fmt.Println("+--------------------------------------------------+")
	fmt.Println("|           GRES B2B Governance Verify            |")
	fmt.Println("+--------------------------------------------------+")

	status := "PASS"
	if !result.Pass {
		status = "FAIL"
	}
	fmt.Printf("|  Status: %-40s|\n", status)
	fmt.Printf("|  RED:    %-40s|\n", formatCount(result.RedCount, result.MaxRed, "max_red"))
	fmt.Printf("|  AMBER:  %-40s|\n", formatCount(result.AmberCount, result.MaxAmber, "max_amber"))
	fmt.Printf("|  GREEN:  %-40d|\n", result.GreenCount)

	if result.FailOnRed != nil {
		fmt.Printf("|  fail_on_red:  %-34v|\n", *result.FailOnRed)
	}
	if result.AllowAmber != nil {
		fmt.Printf("|  allow_amber:  %-34v|\n", *result.AllowAmber)
	}

	fmt.Println("+--------------------------------------------------+")
	if len(result.Reasons) > 0 {
		for _, r := range result.Reasons {
			// Wrap long reasons
			if len(r) > 48 {
				fmt.Printf("|  %-48s|\n", r[:48])
				fmt.Printf("|  %-48s|\n", r[48:])
			} else {
				fmt.Printf("|  %-48s|\n", r)
			}
		}
	} else {
		fmt.Printf("|  %-48s|\n", result.Message)
	}
	fmt.Println("+--------------------------------------------------+")
}
func formatCount(count int, cap *int, label string) string {
	if cap != nil {
		return fmt.Sprintf("%d (%s=%d)", count, label, *cap)
	}
	return fmt.Sprintf("%d", count)
}

// ---------------------------------------------------------------------------
// SARIF output
// ---------------------------------------------------------------------------

type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}
type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}
type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}
type sarifDriver struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type sarifResult struct {
	RuleID  string          `json:"ruleId"`
	Level   string          `json:"level"`
	Message sarifMessage    `json:"message"`
	Locs    []sarifLocation `json:"locations,omitempty"`
}
type sarifMessage struct {
	Text string `json:"text"`
}
type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}
type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           *sarifRegion  `json:"region,omitempty"`
}
type sarifArtifact struct {
	URI string `json:"uri"`
}
type sarifRegion struct {
	StartLine int `json:"startLine"`
}

func writeSARIF(workspaceRoot string, scanResults *ScanResults, verifyResult *VerifyResult) error {
	var results []sarifResult

	for _, v := range scanResults.Red {
		r := sarifResult{
			RuleID:  v.Rule,
			Level:   "error",
			Message: sarifMessage{Text: v.Message},
		}
		if v.File != "" {
			loc := sarifLocation{PhysicalLocation: sarifPhysical{ArtifactLocation: sarifArtifact{URI: v.File}}}
			if v.Line > 0 {
				loc.PhysicalLocation.Region = &sarifRegion{StartLine: v.Line}
			}
			r.Locs = append(r.Locs, loc)
		}
		results = append(results, r)
	}
	for _, v := range scanResults.Amber {
		r := sarifResult{
			RuleID:  v.Rule,
			Level:   "warning",
			Message: sarifMessage{Text: v.Message},
		}
		if v.File != "" {
			loc := sarifLocation{PhysicalLocation: sarifPhysical{ArtifactLocation: sarifArtifact{URI: v.File}}}
			if v.Line > 0 {
				loc.PhysicalLocation.Region = &sarifRegion{StartLine: v.Line}
			}
			r.Locs = append(r.Locs, loc)
		}
		results = append(results, r)
	}

	// Add gating result as a summary entry
	gateLevel := "none"
	if !verifyResult.Pass {
		gateLevel = "error"
	}
	if gateLevel == "error" {
		results = append(results, sarifResult{
			RuleID:  "governance-gate",
			Level:   "error",
			Message: sarifMessage{Text: verifyResult.Message},
		})
	}

	doc := sarifDocument{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: "gres-b2b", Version: Version}},
			Results: results,
		}},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return support.WriteFileAtomic(filepath.Join(workspaceRoot, ".b2b", "results.sarif"), data)
}

// ---------------------------------------------------------------------------
// JUnit XML output
// ---------------------------------------------------------------------------

type junitTestsuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	Testsuites []junitTestsuite `xml:"testsuite"`
}
type junitTestsuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []junitTestcase `xml:"testcase"`
}
type junitTestcase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
}
type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}
type junitSkipped struct {
	Message string `xml:"message,attr,omitempty"`
}

func writeJUnit(workspaceRoot string, scanResults *ScanResults, verifyResult *VerifyResult) error {
	var cases []junitTestcase
	failures := 0

	allowAmber := false
	if verifyResult.AllowAmber != nil {
		allowAmber = *verifyResult.AllowAmber
	}

	amberViolated := (!allowAmber && len(scanResults.Amber) > 0)
	if verifyResult.MaxAmber != nil && len(scanResults.Amber) > *verifyResult.MaxAmber {
		amberViolated = true
	}

	for _, v := range scanResults.Red {
		tc := junitTestcase{
			Name:      v.Rule,
			Classname: "governance.red",
			Time:      "0",
			Failure: &junitFailure{
				Message: v.Message,
				Type:    "RED",
				Body:    fmt.Sprintf("%s: %s", v.File, v.Message),
			},
		}
		cases = append(cases, tc)
		failures++
	}
	for _, v := range scanResults.Amber {
		tc := junitTestcase{
			Name:      v.Rule,
			Classname: "governance.amber",
			Time:      "0",
		}
		// Amber is a failure only if amber gating is violated
		if amberViolated {
			tc.Failure = &junitFailure{
				Message: v.Message,
				Type:    "AMBER",
				Body:    fmt.Sprintf("%s: %s", v.File, v.Message),
			}
			failures++
		} else {
			tc.Skipped = &junitSkipped{Message: "amber tolerated by gating"}
		}
		cases = append(cases, tc)
	}
	for _, v := range scanResults.Green {
		cases = append(cases, junitTestcase{
			Name:      v.Rule,
			Classname: "governance.green",
			Time:      "0",
		})
	}

	// Add the gating verdict itself
	gateCase := junitTestcase{
		Name:      "governance-gate",
		Classname: "governance.verify",
		Time:      "0",
	}
	if !verifyResult.Pass {
		gateCase.Failure = &junitFailure{
			Message: verifyResult.Message,
			Type:    "GATE",
			Body:    verifyResult.Message,
		}
		failures++
	}
	cases = append(cases, gateCase)

	doc := junitTestsuites{
		Testsuites: []junitTestsuite{{
			Name:     "governance-verify",
			Tests:    len(cases),
			Failures: failures,
			Time:     "0",
			Cases:    cases,
		}},
	}

	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	header := []byte(xml.Header)
	return support.WriteFileAtomic(filepath.Join(workspaceRoot, ".b2b", "junit.xml"), append(header, data...))
}

// ---------------------------------------------------------------------------
// runVerify orchestrates the full verify pipeline
// ---------------------------------------------------------------------------

func shouldExit() bool {
	return os.Getenv("GRES_NO_EXIT") != "1"
}

func runVerify() {
	workspace := config.Paths.WorkspaceRoot

	// Load project-level verify config
	verifyCfg, err := loadVerifyConfig(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		if shouldExit() {
			os.Exit(1)
		}
		return
	}

	// Load scan results
	scanResults, err := loadScanResults(workspace)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "ERROR: Scan results missing: expected %s. Run `gres-b2b scan` or ensure pipeline produces it.\n", filepath.Join(workspace, ".b2b", "results.json"))
			if shouldExit() {
				os.Exit(1)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		if shouldExit() {
			os.Exit(1)
		}
		return
	}

	// Evaluate gating
	result := evaluateGating(verifyCfg, len(scanResults.Red), len(scanResults.Amber), len(scanResults.Green))

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Join(workspace, ".b2b"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot create output directory: %v\n", err)
		os.Exit(1)
	}

	// Write outputs
	if err := writeSARIF(workspace, scanResults, result); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to write SARIF: %v\n", err)
	}
	if err := writeJUnit(workspace, scanResults, result); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to write JUnit: %v\n", err)
	}

	certHash, verified, err := generateVerifyCertificate(workspace, result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to write certificate: %v\n", err)
	}
	if err := updateReportSignature(workspace, verified, certHash); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to update report signature: %v\n", err)
	}
	if result.Pass {
		if _, err := createBackupSnapshot(workspace); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to create backup snapshot: %v\n", err)
		}
	}

	auditEntry := support.AuditEntry{
		Mode:           "verify",
		RedCount:       result.RedCount,
		AmberCount:     result.AmberCount,
		GreenCount:     result.GreenCount,
		CertificateSHA: certHash,
	}
	if data, err := os.ReadFile(filepath.Join(workspace, ".b2b", "report.json")); err == nil {
		var rep report
		if err := json.Unmarshal(data, &rep); err == nil {
			auditEntry.Phase1Status = rep.Phase1Status
			auditEntry.Phase2Status = rep.Phase2Status
			auditEntry.Phase3Status = rep.Phase3Status
			auditEntry.Phase4Status = rep.Phase4Status
		}
	}
	_ = support.AppendAudit(workspace, auditEntry)

	// Print HUD
	printHUD(result)

	if !result.Pass {
		if shouldExit() {
			os.Exit(1)
		}
	}
}
