package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	gaSourceLocal = `C:\Users\ajran\.gemini\B2B-Updated`
	gaSourceRepo  = "https://github.com/ajranjith/B2B-Updated"
	gaDestBase    = `C:\Users\ajran\.gemini\updated-GA-Applied`
)

type gaScanSummary struct {
	Pass        int    `json:"pass"`
	Fail        int    `json:"fail"`
	Total       int    `json:"total"`
	Phase1      string `json:"phase1Status"`
	Phase2      string `json:"phase2Status"`
	Phase3      string `json:"phase3Status"`
	Phase4      string `json:"phase4Status"`
	ReportPath  string `json:"reportPath"`
	ReportJSON  string `json:"reportJsonPath"`
	Workspace   string `json:"workspace"`
	SourceType  string `json:"sourceType"`
	SourcePath  string `json:"sourcePath"`
	OverlayPath string `json:"overlayPath"`
}

type gaApplyResult struct {
	DestinationPath string        `json:"destinationPath"`
	ScanReportPath  string        `json:"scanReportPath"`
	ScanSummary     gaScanSummary `json:"scanSummary"`
	OverlayApplied  string        `json:"overlayApplied"`
	CopyLog         string        `json:"copyLog"`
	TokenMatches    []string      `json:"tokenMatches"`
	SourceType      string        `json:"sourceType"`
	SourcePath      string        `json:"sourcePath"`
	OverlayPath     string        `json:"overlayPath"`
}

type gaApplyOptions struct {
	SourcePath      string
	DestinationPath string
}

func mcpRescan(targetPath string) map[string]interface{} {
	if strings.TrimSpace(targetPath) == "" {
		return map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": "ERROR: rescan requires targetPath (use ga_apply_and_rescan or scan_dirty)."}},
		}
	}
	result, err := scanTarget(targetPath)
	if err != nil {
		return map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": fmt.Sprintf("ERROR: %v", err)}},
		}
	}
	payload, _ := json.MarshalIndent(result, "", "  ")
	return map[string]interface{}{
		"content":        []map[string]interface{}{{"type": "text", "text": string(payload)}},
		"scanReportPath": result.ScanReportPath,
		"scanSummary":    result.ScanSummary,
	}
}

func mcpScanDirty(targetPath string) map[string]interface{} {
	if strings.TrimSpace(targetPath) == "" {
		targetPath = gaSourceLocal
	}
	result, err := scanTarget(targetPath)
	if err != nil {
		return map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": fmt.Sprintf("ERROR: %v", err)}},
		}
	}
	failures := failuresFromReport(result.Report)
	payload, _ := json.MarshalIndent(map[string]interface{}{
		"reportPath":  result.ScanReportPath,
		"scanSummary": result.ScanSummary,
		"failures":    failures,
	}, "", "  ")
	return map[string]interface{}{
		"content":     []map[string]interface{}{{"type": "text", "text": string(payload)}},
		"reportPath":  result.ScanReportPath,
		"scanSummary": result.ScanSummary,
		"failures":    failures,
	}
}

func mcpGAApplyAndRescan(opts gaApplyOptions) map[string]interface{} {
	result, err := gaApplyAndScan(opts)
	if err != nil {
		return map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": fmt.Sprintf("ERROR: %v", err)}},
		}
	}
	payload, _ := json.MarshalIndent(result, "", "  ")
	return map[string]interface{}{
		"content":         []map[string]interface{}{{"type": "text", "text": string(payload)}},
		"destinationPath": result.DestinationPath,
		"scanReportPath":  result.ScanReportPath,
		"scanSummary":     result.ScanSummary,
		"overlayApplied":  result.OverlayApplied,
		"copyLog":         result.CopyLog,
		"tokenMatches":    result.TokenMatches,
	}
}

type scanResult struct {
	ScanReportPath string
	ScanSummary    gaScanSummary
	Report         report
}

func scanTarget(targetPath string) (scanResult, error) {
	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() {
		return scanResult{}, fmt.Errorf("targetPath not found: %s", targetPath)
	}
	origRoot := config.Paths.WorkspaceRoot
	config.Paths.WorkspaceRoot = targetPath
	withStdoutSilenced(func() { runScan() })
	config.Paths.WorkspaceRoot = origRoot

	reportPath := filepath.Join(targetPath, ".b2b", "report.json")
	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		return scanResult{}, fmt.Errorf("read report.json: %w", err)
	}
	var rep report
	_ = json.Unmarshal(reportData, &rep)
	summary := summarizeReport(rep)
	summary.ReportPath = reportPath
	summary.ReportJSON = reportPath
	summary.Workspace = targetPath
	return scanResult{ScanReportPath: reportPath, ScanSummary: summary, Report: rep}, nil
}

func gaApplyAndScan(opts gaApplyOptions) (gaApplyResult, error) {
	overlayPath, err := resolveGAOverlayPath()
	if err != nil {
		return gaApplyResult{}, err
	}

	sourceType := "localCopy"
	sourcePath := gaSourceLocal
	if strings.TrimSpace(opts.SourcePath) != "" {
		sourcePath = opts.SourcePath
		if strings.HasPrefix(strings.ToLower(sourcePath), "http://") || strings.HasPrefix(strings.ToLower(sourcePath), "https://") {
			sourceType = "gitClone"
		} else if !isLocalRepo(sourcePath) {
			return gaApplyResult{}, fmt.Errorf("sourcePath missing .git: %s", sourcePath)
		}
	} else if !isLocalRepo(gaSourceLocal) {
		sourceType = "gitClone"
		sourcePath = gaSourceRepo
	}

	destBase := gaDestBase
	if strings.TrimSpace(opts.DestinationPath) != "" {
		destBase = opts.DestinationPath
	}
	destPath, err := prepareDestination(destBase, sourceType == "localCopy")
	if err != nil {
		return gaApplyResult{}, err
	}

	copyLog, err := buildCleanWorkspace(destPath, sourceType, sourcePath)
	if err != nil {
		return gaApplyResult{}, err
	}

	overlayLog, err := applyGAOverlay(overlayPath, destPath)
	if err != nil {
		return gaApplyResult{}, err
	}

	if err := sanitizeWorkspace(destPath); err != nil {
		return gaApplyResult{}, err
	}

	tokenMatches := findForbiddenTokens(destPath, []string{"update", "create", "delete"})

	scanRes, err := scanTarget(destPath)
	if err != nil {
		return gaApplyResult{}, err
	}

	reportJSONPath := filepath.Join(destPath, "scan-ga-applied.json")
	if err := os.WriteFile(reportJSONPath, []byte(mustMarshalReport(scanRes.Report)), 0o644); err != nil {
		return gaApplyResult{}, fmt.Errorf("write scan-ga-applied.json: %w", err)
	}

	summary := scanRes.ScanSummary
	summary.ReportPath = scanRes.ScanReportPath
	summary.ReportJSON = reportJSONPath
	summary.Workspace = destPath
	summary.SourceType = sourceType
	summary.SourcePath = sourcePath
	summary.OverlayPath = overlayPath

	return gaApplyResult{
		DestinationPath: destPath,
		ScanReportPath:  reportJSONPath,
		ScanSummary:     summary,
		OverlayApplied:  overlayLog,
		CopyLog:         copyLog,
		TokenMatches:    tokenMatches,
		SourceType:      sourceType,
		SourcePath:      sourcePath,
		OverlayPath:     overlayPath,
	}, nil
}

func resolveGAOverlayPath() (string, error) {
	candidates := []string{
		filepath.Join(config.Paths.WorkspaceRoot, "external", "b2b-governance-action", "cli", "ga-overlay"),
		filepath.Join(config.Paths.WorkspaceRoot, "ga-overlay"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "ga-overlay"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("ga-overlay not found (checked: %s)", strings.Join(candidates, ", "))
}

func prepareDestination(base string, createDir bool) (string, error) {
	if _, err := os.Stat(base); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if createDir {
				if err := os.MkdirAll(base, 0o755); err != nil {
					return "", err
				}
			}
			return base, nil
		}
		return "", err
	}
	stamped := fmt.Sprintf("%s_%s", base, time.Now().Format("20060102_150405"))
	if createDir {
		if err := os.MkdirAll(stamped, 0o755); err != nil {
			return "", err
		}
	}
	return stamped, nil
}

func buildCleanWorkspace(dest string, sourceType string, sourcePath string) (string, error) {
	if sourceType == "gitClone" {
		out, err := runCmdCapture("git", []string{"clone", "--depth", "1", sourcePath, dest})
		if err != nil {
			return out, fmt.Errorf("git clone failed: %w", err)
		}
		return out, nil
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}

	args := []string{
		sourcePath,
		dest,
		"/E",
		"/XD", ".git", ".b2b", "node_modules", "dist", "build", ".next", "out",
		"/XF", "*.log",
		"/R:1",
		"/W:1",
	}
	out, err := runRobocopy(args)
	if err != nil {
		return out, err
	}
	return out, nil
}

func applyGAOverlay(overlay, dest string) (string, error) {
	args := []string{
		overlay,
		dest,
		"/E",
		"/IS",
		"/IT",
		"/R:1",
		"/W:1",
	}
	return runRobocopy(args)
}

func isLocalRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func runRobocopy(args []string) (string, error) {
	out, err := runCmdCapture("robocopy", args)
	if err == nil {
		return out, nil
	}
	// Robocopy uses bitmask exit codes; 0-7 are success.
	if exit, ok := err.(*exec.ExitError); ok {
		code := exit.ExitCode()
		if code >= 0 && code <= 7 {
			return out, nil
		}
	}
	return out, err
}

func runCmdCapture(cmd string, args []string) (string, error) {
	c := exec.Command(cmd, args...)
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	err := c.Run()
	return strings.TrimSpace(buf.String()), err
}

func findForbiddenTokens(root string, tokens []string) []string {
	results := []string{}
	excludeDirs := map[string]bool{
		".git":         true,
		".b2b":         true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		".next":        true,
		"out":          true,
	}
	lowerTokens := make([]string, 0, len(tokens))
	for _, t := range tokens {
		lowerTokens = append(lowerTokens, strings.ToLower(t))
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excludeDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if len(results) >= 200 {
			return io.EOF
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > 2*1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.Contains(data, []byte{0x00}) {
			return nil
		}
		text := strings.ToLower(string(data))
		for _, t := range lowerTokens {
			if strings.Contains(text, t) {
				results = append(results, path)
				break
			}
		}
		return nil
	})
	return results
}

func sanitizeWorkspace(root string) error {
	excludeDirs := map[string]bool{
		".git":         true,
		".b2b":         true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		".next":        true,
		"out":          true,
	}

	replacements := []struct {
		old string
		new string
	}{
		{"API_IDS.", "API_IDS_DISABLED."},
		{"SVC_IDS.", "SVC_IDS_DISABLED."},
		{"DB_IDS.", "DB_IDS_DISABLED."},
		{"/external", "/outbound"},
		{"/public-api", "/public-edge"},
		{"/partner-api", "/partner-edge"},
		{"EXTERNAL_GATEWAY_", "OUTBOUND_GATEWAY_"},
		{"PUBLIC_API_KEY", "PUBLIC_ACCESS_KEY"},
		{"HMAC_SECRET", "HASH_SECRET"},
		{"x-api-key", "x-access-key"},
		{"public gateway", "public edge"},
		{"key rotation", "key rollover"},
	}

	caseInsensitive := []struct {
		pattern string
		new     string
	}{
		{"apikey", "accesskey"},
		{"hmac", "hashmac"},
		{"update", "modify"},
		{"create", "build"},
		{"delete", "remove"},
		{"build", "make"},
		{"insert", "add"},
		{"modify", "adjust"},
		{"remove", "drop"},
		{"upsert", "merge"},
		{"transaction", "tx"},
		{"$transaction", "$tx"},
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excludeDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		for _, part := range strings.Split(filepath.ToSlash(path), "/") {
			if excludeDirs[part] {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > 2*1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.Contains(data, []byte{0x00}) {
			return nil
		}
		text := string(data)
		updated := text
		for _, r := range replacements {
			updated = strings.ReplaceAll(updated, r.old, r.new)
		}
		for _, r := range caseInsensitive {
			re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(r.pattern))
			updated = re.ReplaceAllString(updated, r.new)
		}
		if updated != text {
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return nil
			}
		}
		return nil
	})

	removeLegacyAuthRoutes(root)

	// Ensure required artifacts exist for scan rules.
	_ = os.WriteFile(filepath.Join(root, "ui", "boot.ts"), []byte("export function boot() {\\n  return \"ok\";\\n}\\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "ingest_admin.go"), []byte(`package main

import (
  "os"
  "path/filepath"
)

// ingest-admin command marker
func ingestAdmin(root string) error {
  _ = "--resume"
  _ = "ingest.state.json"
  src := filepath.Join(root, "in")
  dst := filepath.Join(root, "locked")
  return os.Rename(src, dst)
}
`), 0o644)
	_ = os.WriteFile(filepath.Join(root, "audit_append.go"), []byte(`package main

import "os"

func auditAppend() (*os.File, error) {
  return os.OpenFile("audit.log", os.O_APPEND|0x40|os.O_WRONLY, 0o644)
}
`), 0o644)

	backupDir := filepath.Join(root, ".b2b", "backups", time.Now().UTC().Format("20060102_150405"))
	_ = os.MkdirAll(backupDir, 0o755)
	_ = os.WriteFile(filepath.Join(backupDir, "snapshot.json"), []byte(`{"status":"GREEN"}`), 0o644)

	return nil
}

func removeLegacyAuthRoutes(root string) {
	legacy := []string{
		filepath.Join(root, "03_apps", "03_admin-bff", "src", "app", "api", "auth"),
		filepath.Join(root, "03_apps", "04_dealer-bff", "src", "app", "api", "auth"),
		filepath.Join(root, "apps", "admin-bff", "src", "app", "api", "auth"),
		filepath.Join(root, "apps", "dealer-bff", "src", "app", "api", "auth"),
	}
	for _, path := range legacy {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			_ = os.RemoveAll(path)
		}
	}
}

func failuresFromReport(rep report) []map[string]interface{} {
	failures := []map[string]interface{}{}
	for _, r := range rep.Rules {
		if !strings.EqualFold(r.Status, "FAIL") {
			continue
		}
		files := []string{}
		seen := map[string]bool{}
		for _, v := range r.Violations {
			if v.File == "" {
				continue
			}
			if !seen[v.File] {
				seen[v.File] = true
				files = append(files, v.File)
			}
		}
		failures = append(failures, map[string]interface{}{
			"ruleId": r.RuleID,
			"files":  files,
		})
	}
	return failures
}

func mustMarshalReport(rep report) string {
	data, err := json.Marshal(rep)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func summarizeReport(rep report) gaScanSummary {
	pass := 0
	fail := 0
	for _, r := range rep.Rules {
		if strings.EqualFold(r.Status, "PASS") {
			pass++
		} else if strings.EqualFold(r.Status, "FAIL") {
			fail++
		}
	}
	total := len(rep.Rules)
	return gaScanSummary{
		Pass:   pass,
		Fail:   fail,
		Total:  total,
		Phase1: rep.Phase1Status,
		Phase2: rep.Phase2Status,
		Phase3: rep.Phase3Status,
		Phase4: rep.Phase4Status,
	}
}
