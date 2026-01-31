package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type ingestState struct {
	Status       string   `json:"status"`
	StartedAtUtc string   `json:"startedAtUtc"`
	LastFile     string   `json:"lastFile,omitempty"`
	MovedCount   int      `json:"movedCount"`
	PendingFiles []string `json:"pendingFiles,omitempty"`
}

func runIngestAdmin(incoming, locked string, resume bool) {
	if _, err := ingestAdmin(incoming, locked, resume); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func ingestAdmin(incoming, locked string, resume bool) (string, error) {
	workspace := config.Paths.WorkspaceRoot
	if incoming == "" {
		incoming = filepath.Join(workspace, "incoming")
	}
	if locked == "" {
		locked = filepath.Join(workspace, "locked")
	}

	if err := os.MkdirAll(incoming, 0o755); err != nil {
		return "", fmt.Errorf("cannot access incoming: %w", err)
	}
	if err := os.MkdirAll(locked, 0o755); err != nil {
		return "", fmt.Errorf("cannot access locked: %w", err)
	}

	statePath := filepath.Join(workspace, ".b2b", "ingest.state.json")
	state := ingestState{
		Status:       "IN_PROGRESS",
		StartedAtUtc: time.Now().UTC().Format(time.RFC3339),
	}

	if resume {
		if data, err := os.ReadFile(statePath); err == nil {
			_ = json.Unmarshal(data, &state)
		}
	}

	pending, err := listPendingFiles(incoming, state.PendingFiles, resume)
	if err != nil {
		return "", fmt.Errorf("cannot list incoming: %w", err)
	}
	state.PendingFiles = pending
	if err := support.WriteJSONAtomic(statePath, state); err != nil {
		return "", fmt.Errorf("cannot write ingest state: %w", err)
	}

	failAfter := -1
	if v := os.Getenv("GRES_INGEST_FAIL_AFTER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			failAfter = n
		}
	}

	for _, name := range pending {
		src := filepath.Join(incoming, name)
		dst := filepath.Join(locked, name)

		if _, err := os.Stat(src); err != nil {
			continue
		}

		if err := os.Rename(src, dst); err != nil {
			state.Status = "FAILED"
			state.LastFile = name
			_ = support.WriteJSONAtomic(statePath, state)
			return "", fmt.Errorf("ingest failed on %s: %w", name, err)
		}

		state.MovedCount++
		state.LastFile = name
		state.PendingFiles = remainingAfter(pending, name)
		if err := support.WriteJSONAtomic(statePath, state); err != nil {
			return "", fmt.Errorf("cannot update ingest state: %w", err)
		}

		if failAfter >= 0 && state.MovedCount > failAfter {
			state.Status = "FAILED"
			_ = support.WriteJSONAtomic(statePath, state)
			return "", fmt.Errorf("simulated ingest failure after %d files", failAfter)
		}
	}

	state.Status = "COMPLETE"
	state.PendingFiles = nil
	if err := support.WriteJSONAtomic(statePath, state); err != nil {
		return "", fmt.Errorf("cannot finalize ingest state: %w", err)
	}

	certHash, err := generateIngestCertificate(workspace)
	if err != nil {
		return "", fmt.Errorf("cannot generate ingest certificate: %w", err)
	}

	if err := support.AppendAudit(workspace, support.AuditEntry{
		Mode:           "ingest-admin",
		CertificateSHA: certHash,
	}); err != nil {
		return certHash, fmt.Errorf("cannot append audit log: %w", err)
	}
	return certHash, nil
}

func listPendingFiles(incoming string, statePending []string, resume bool) ([]string, error) {
	if resume && len(statePending) > 0 {
		return statePending, nil
	}
	entries, err := os.ReadDir(incoming)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	return files, nil
}

func remainingAfter(all []string, current string) []string {
	out := []string{}
	for _, v := range all {
		if v == current {
			continue
		}
		out = append(out, v)
	}
	return out
}
