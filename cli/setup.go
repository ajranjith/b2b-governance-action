package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type setupState struct {
	CurrentStep int      `json:"currentStep"`
	Steps       []string `json:"steps"`
	Status      string   `json:"status"`
	UpdatedAt   string   `json:"updatedAtUtc"`
}

func runSetup() {
	workspace := config.Paths.WorkspaceRoot
	statePath := filepath.Join(workspace, ".b2b", "setup.json")
	steps := []string{"registry", "bff", "contracts", "uiRegistry", "verify"}
	state := setupState{Steps: steps, Status: "IN_PROGRESS"}

	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &state)
	}

	if state.CurrentStep >= len(steps) {
		state.Status = "COMPLETE"
	} else {
		state.CurrentStep++
		if state.CurrentStep >= len(steps) {
			state.Status = "COMPLETE"
		}
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := support.WriteJSONAtomic(statePath, state); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	updateSetupReport(workspace, state)
}

func updateSetupReport(workspace string, state setupState) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		return
	}

	status := "PASS"
	if state.Status != "COMPLETE" {
		status = "WARN"
	}

	rep.Rules = upsertRule(rep.Rules, makeRule("4.6.4", status, "low", map[string]interface{}{
		"setupStatus": state.Status,
		"currentStep": state.CurrentStep,
		"steps":       state.Steps,
	}, nil, ""))

	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(reportPath, rep)
}
