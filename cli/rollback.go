package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type rollbackEntry struct {
	TimestampUtc string `json:"timestampUtc"`
	SnapshotID   string `json:"snapshotId"`
	Result       string `json:"result"`
	Message      string `json:"message,omitempty"`
}

func createBackupSnapshot(workspace string) (string, error) {
	backupRoot := filepath.Join(workspace, ".b2b", "backups")
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		return "", err
	}
	id := time.Now().UTC().Format("20060102_150405")
	snapshotDir := filepath.Join(backupRoot, id)
	if _, err := os.Stat(snapshotDir); err == nil {
		time.Sleep(1100 * time.Millisecond)
		id = time.Now().UTC().Format("20060102_150405")
		snapshotDir = filepath.Join(backupRoot, id)
	}
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return "", err
	}

	copies := []string{
		".b2b/report.json",
		".b2b/report.html",
		".b2b/certificate.json",
		".b2b/config.yml",
		"ui/registry.json",
		".b2b/api-routes.json",
		".b2b/hints.json",
	}
	for _, rel := range copies {
		src := filepath.Join(workspace, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(snapshotDir, rel)
		if err := support.CopyFileAtomic(src, dst); err != nil {
			return "", err
		}
	}

	updateRollbackReport(workspace, id, "", true)
	return id, nil
}

func runRollbackLatest() {
	if err := rollbackToSnapshot(""); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runRollbackTo(id string) {
	if err := rollbackToSnapshot(id); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func rollbackToSnapshot(id string) error {
	workspace := config.Paths.WorkspaceRoot
	backupRoot := filepath.Join(workspace, ".b2b", "backups")
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return fmt.Errorf("no backups available")
	}
	ids := []string{}
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("no backups available")
	}
	sort.Strings(ids)
	if id == "" {
		id = ids[len(ids)-1]
	}
	snapshotDir := filepath.Join(backupRoot, id)
	if _, err := os.Stat(snapshotDir); err != nil {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	restore := []string{
		".b2b/report.json",
		".b2b/report.html",
		".b2b/certificate.json",
		".b2b/config.yml",
		"ui/registry.json",
		".b2b/api-routes.json",
		".b2b/hints.json",
	}
	for _, rel := range restore {
		src := filepath.Join(snapshotDir, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(workspace, rel)
		if err := support.CopyFileAtomic(src, dst); err != nil {
			return err
		}
	}

	_ = appendRollbackLog(workspace, rollbackEntry{SnapshotID: id, Result: "PASS"})
	updateRollbackReport(workspace, id, time.Now().UTC().Format(time.RFC3339), false)

	audit := support.AuditEntry{Mode: "rollback", Result: "PASS"}
	if data, err := os.ReadFile(filepath.Join(workspace, ".b2b", "report.json")); err == nil {
		var rep report
		if err := json.Unmarshal(data, &rep); err == nil {
			audit.Phase1Status = rep.Phase1Status
			audit.Phase2Status = rep.Phase2Status
			audit.Phase3Status = rep.Phase3Status
			audit.Phase4Status = rep.Phase4Status
		}
	}
	_ = support.AppendAudit(workspace, audit)
	return nil
}

func appendRollbackLog(workspace string, entry rollbackEntry) error {
	entry.TimestampUtc = time.Now().UTC().Format(time.RFC3339)
	path := filepath.Join(workspace, ".b2b", "rollback.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func updateRollbackReport(workspace, snapshotID, rollbackAt string, refreshOnly bool) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		return
	}

	backupIDs := listBackupSnapshots(workspace)
	latest := ""
	if len(backupIDs) > 0 {
		latest = backupIDs[len(backupIDs)-1]
	}
	status := "FAIL"
	if len(backupIDs) > 0 {
		status = "PASS"
	}
	evidence := map[string]interface{}{
		"backupCount":  len(backupIDs),
		"latestBackup": latest,
	}
	if snapshotID != "" {
		evidence["rollbackSnapshotId"] = snapshotID
	}
	if rollbackAt != "" {
		evidence["lastRollbackAtUtc"] = rollbackAt
	}
	if refreshOnly {
		evidence["snapshotCreated"] = snapshotID
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("ROLLBACK_READY", status, "high", evidence, nil, "Create a GREEN/PASS snapshot before attempting rollback."))
	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(reportPath, rep)
}
