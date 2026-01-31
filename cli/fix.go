package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type fixAction struct {
	File    string `json:"file"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type fixPlan struct {
	DryRun  bool        `json:"dryRun"`
	Actions []fixAction `json:"actions"`
}

type fixApply struct {
	DryRun  bool        `json:"dryRun"`
	Applied []fixAction `json:"applied"`
	Result  string      `json:"result"`
}

type semanticBlockError struct {
	File   string
	Reason string
}

func (e semanticBlockError) Error() string {
	return fmt.Sprintf("semantic change blocked: %s (%s)", e.File, e.Reason)
}

func runFix(dryRun bool) {
	if err := runFixInternal(dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runFixInternal(dryRun bool) error {
	workspace := config.Paths.WorkspaceRoot
	plan, err := buildFixPlan(workspace)
	if err != nil {
		return err
	}
	plan.DryRun = dryRun

	planPath := filepath.Join(workspace, ".b2b", "fix-plan.json")
	_ = support.WriteJSONAtomic(planPath, plan)
	_ = support.WriteFileAtomic(filepath.Join(workspace, ".b2b", "fix.patch"), []byte(renderPatch(plan)))

	if dryRun {
		updateFixReport(workspace, plan, dryRun, nil)
		_ = support.AppendAudit(workspace, support.AuditEntry{Mode: "fix", Actions: len(plan.Actions), Result: "DRY_RUN", DryRun: true})
		return nil
	}

	err = applyFixPlan(workspace, plan)
	updateFixReport(workspace, plan, dryRun, err)
	if err != nil {
		_ = support.AppendAudit(workspace, support.AuditEntry{Mode: "fix", Actions: len(plan.Actions), Result: "FAIL", DryRun: false})
		return err
	}

	_ = support.AppendAudit(workspace, support.AuditEntry{Mode: "fix", Actions: len(plan.Actions), Result: "PASS", DryRun: false})
	return nil
}

func buildFixPlan(workspace string) (*fixPlan, error) {
	actions := []fixAction{}

	missingWrap := findBFFMissingSignatures(workspace, append(config.Scan.BFFPaths, append(config.Scan.DealerBFFPaths, config.Scan.AdminBFFPaths...)...), config.Scan.WrapperSignatures)
	for _, v := range missingWrap {
		actions = append(actions, fixAction{File: v.File, Kind: "add_wrapper", Message: "insert wrapper signature"})
	}

	llidMissing := checkDealerLLID(workspace, resolveModules(workspace))
	for _, v := range llidMissing {
		actions = append(actions, fixAction{File: v.File, Kind: "add_llid", Message: "add llid field marker"})
	}

	mutationFindings, _ := detectMutations(workspace, config.Scan.AuditWrapperSignatures)
	for _, v := range mutationFindings {
		actions = append(actions, fixAction{File: v.File, Kind: "add_audit", Message: "insert audit wrapper marker"})
	}

	return &fixPlan{Actions: uniqueFixActions(actions)}, nil
}

func applyFixPlan(workspace string, plan *fixPlan) error {
	for _, action := range plan.Actions {
		if isProtectedPath(action.File) {
			return semanticBlockError{File: action.File, Reason: "protected path"}
		}
		if fileHasSemanticLock(action.File) {
			return semanticBlockError{File: action.File, Reason: "semantic lock"}
		}
		if err := applyFixAction(action); err != nil {
			return err
		}
	}
	applyPath := filepath.Join(workspace, ".b2b", "fix-apply.json")
	return support.WriteJSONAtomic(applyPath, fixApply{DryRun: false, Applied: plan.Actions, Result: "PASS"})
}

func applyFixAction(action fixAction) error {
	data, err := os.ReadFile(action.File)
	if err != nil {
		return err
	}
	original := string(data)

	var updated string
	var mode string
	var marker string

	switch action.Kind {
	case "add_wrapper":
		if len(config.Scan.WrapperSignatures) == 0 {
			return fmt.Errorf("wrapper signature not configured")
		}
		marker = config.Scan.WrapperSignatures[0]
		updated = marker + "\n" + original
		mode = "prepend"
	case "add_llid":
		marker = "\"llid\": {\"type\": \"string\"}"
		updated = original + "\n" + marker + "\n"
		mode = "append"
	case "add_audit":
		marker = "withAudit();"
		updated = marker + "\n" + original
		mode = "prepend"
	default:
		return fmt.Errorf("unknown fix action: %s", action.Kind)
	}

	if !validateStructuralChange(original, updated, marker, mode) {
		return semanticBlockError{File: action.File, Reason: "non-structural change detected"}
	}

	return support.WriteFileAtomic(action.File, []byte(updated))
}

func validateStructuralChange(original, updated, marker, mode string) bool {
	old := normalizeNewlines(original)
	newText := normalizeNewlines(updated)
	switch mode {
	case "prepend":
		expected := marker + "\n" + old
		return newText == expected
	case "append":
		expected1 := old + "\n" + marker + "\n"
		expected2 := old + "\n" + marker
		return newText == expected1 || newText == expected2
	default:
		return false
	}
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func uniqueFixActions(actions []fixAction) []fixAction {
	seen := map[string]struct{}{}
	out := []fixAction{}
	for _, a := range actions {
		key := a.File + "|" + a.Kind
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, a)
	}
	return out
}

func isProtectedPath(path string) bool {
	for _, p := range config.Scan.SemanticProtectedPaths {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

func fileHasSemanticLock(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	return strings.Contains(text, "GRES:SEMANTIC-LOCK")
}

func renderPatch(plan *fixPlan) string {
	var b strings.Builder
	for _, a := range plan.Actions {
		b.WriteString("PATCH ")
		b.WriteString(a.File)
		b.WriteString(" ")
		b.WriteString(a.Kind)
		b.WriteString("\n")
	}
	return b.String()
}

func resolveModules(workspace string) []moduleRoot {
	reg, _, ok := loadRegistry(workspace)
	if !ok || reg == nil {
		return nil
	}
	mods := []moduleRoot{}
	for _, m := range reg.Modules {
		mods = append(mods, moduleRoot{Name: m.Name, Root: m.Root, Svc: m.SvcID})
	}
	return mods
}

func updateFixReport(workspace string, plan *fixPlan, dryRun bool, applyErr error) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		return
	}

	rep.Rules = upsertRule(rep.Rules, makeRule("4.4.1", "PASS", "medium", map[string]interface{}{
		"actions": len(plan.Actions),
		"dryRun":  dryRun,
	}, nil, ""))

	status442 := "PASS"
	violations := []finding{}
	if applyErr != nil {
		status442 = "FAIL"
		if sb, ok := applyErr.(semanticBlockError); ok {
			violations = append(violations, finding{File: sb.File, Line: 1, Message: sb.Reason, RuleID: "4.4.2", FixHint: "Move changes out of protected zones or remove semantic lock."})
		} else {
			violations = append(violations, finding{File: "unknown", Line: 1, Message: applyErr.Error(), RuleID: "4.4.2", FixHint: "Review fix actions and ensure only structural changes."})
		}
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.4.2", status442, "high", nil, violations, "Ensure fix only applies structural changes."))

	planExists := fileExists(filepath.Join(workspace, ".b2b", "fix-plan.json"))
	patchExists := fileExists(filepath.Join(workspace, ".b2b", "fix.patch"))
	applyExists := fileExists(filepath.Join(workspace, ".b2b", "fix-apply.json"))

	status443 := "PASS"
	if !planExists || !patchExists || (!dryRun && !applyExists) {
		status443 = "FAIL"
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.4.3", status443, "medium", map[string]interface{}{
		"fixPlan":  planExists,
		"fixPatch": patchExists,
		"fixApply": applyExists,
	}, nil, "Ensure fix produces plan/patch and apply outputs."))

	auditExists := fileExists(filepath.Join(workspace, ".b2b", "audit.log"))
	status444 := "PASS"
	if !auditExists {
		status444 = "FAIL"
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.4.4", status444, "low", map[string]interface{}{
		"auditLog": auditExists,
	}, nil, "Ensure fix runs append to the audit log."))

	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(reportPath, rep)
}
