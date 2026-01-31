package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
	"gopkg.in/yaml.v3"
)

type registryModule struct {
	Name  string `json:"name"`
	Root  string `json:"root"`
	SvcID string `json:"svcId,omitempty"`
}

type registry struct {
	Version string                 `json:"version"`
	Modules []registryModule       `json:"modules"`
	IDs     map[string]interface{} `json:"ids"`
}

type impactGraph struct {
	GeneratedAtUtc string         `json:"generatedAtUtc"`
	Modules        []impactModule `json:"modules"`
	Edges          []impactEdge   `json:"edges"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type impactModule struct {
	Name  string `json:"name"`
	Root  string `json:"root"`
	SvcID string `json:"svcId,omitempty"`
}

type impactEdge struct {
	FromModule string `json:"fromModule"`
	ToModule   string `json:"toModule"`
	Reason     string `json:"reason"`
	ImportPath string `json:"importPath"`
	Violates   bool   `json:"violates,omitempty"`
	RuleID     string `json:"ruleId,omitempty"`
}

type apiRoute struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Method string `json:"method"`
	Path   string `json:"path,omitempty"`
	APIID  string `json:"apiId,omitempty"`
}

type report struct {
	Phase1Status string       `json:"phase1Status"`
	Phase2Status string       `json:"phase2Status,omitempty"`
	Phase3Status string       `json:"phase3Status,omitempty"`
	Phase4Status string       `json:"phase4Status,omitempty"`
	Rules        []ruleResult `json:"rules"`
}

type ruleResult struct {
	RuleID     string                 `json:"ruleId"`
	Severity   string                 `json:"severity,omitempty"`
	Status     string                 `json:"status"`
	Evidence   map[string]interface{} `json:"evidence,omitempty"`
	Violations []finding              `json:"violations,omitempty"`
	FixHint    string                 `json:"fixHint,omitempty"`
	Message    string                 `json:"message,omitempty"`
}

type finding struct {
	RuleID  string `json:"ruleId"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
	FixHint string `json:"fixHint"`
	ID      string `json:"id,omitempty"`
}

type doctor struct {
	RegistryFound        bool              `json:"registryFound"`
	RegistryPath         string            `json:"registryPath,omitempty"`
	RegistryValid        bool              `json:"registryValid"`
	IDNamespaces         map[string]bool   `json:"idNamespaces,omitempty"`
	KernelPresent        bool              `json:"kernelPresent"`
	FoundKernelFiles     []string          `json:"foundKernelFiles,omitempty"`
	ModuleStructureReady bool              `json:"moduleStructureReady"`
	ModuleCount          int               `json:"moduleCount"`
	Notes                map[string]string `json:"notes,omitempty"`
}

type moduleRoot struct {
	Name string
	Root string
	Svc  string
}

type importRef struct {
	Path string
	Line int
}

type moduleFile struct {
	Path   string
	Module string
}

func runScan() {
	workspace := config.Paths.WorkspaceRoot
	loadScanOverrides(workspace)
	outputDir := filepath.Join(workspace, ".b2b")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot create output directory: %v\n", err)
		os.Exit(1)
	}

	rep := &report{}

	reg, regPath, regValid := loadRegistry(workspace)

	// Rule 1.1.1
	rep.Rules = append(rep.Rules, makeRule("1.1.1", statusFromBool(regValid), "high",
		map[string]interface{}{"registryPath": regPath, "registryValid": regValid},
		nil, "Ensure registry exists and is valid JSON with required fields."))

	// Rule 1.1.3 (namespaces)
	ns := map[string]bool{
		"API": false,
		"SVC": false,
		"DB":  false,
	}
	if regValid {
		ns["API"] = hasNamespace(reg.IDs, "API")
		ns["SVC"] = hasNamespace(reg.IDs, "SVC")
		ns["DB"] = hasNamespace(reg.IDs, "DB")
	}
	if !ns["API"] || !ns["SVC"] || !ns["DB"] {
		rep.Rules = append(rep.Rules, makeRule("1.1.3", "FAIL", "high",
			map[string]interface{}{"idNamespaces": ns}, nil,
			"Add ids.API, ids.SVC, and ids.DB to registry."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.1.3", "PASS", "high",
			map[string]interface{}{"idNamespaces": ns}, nil, ""))
	}

	modules := []moduleRoot{}
	if regValid {
		for _, m := range reg.Modules {
			if m.Name == "" || m.Root == "" {
				continue
			}
			modules = append(modules, moduleRoot{Name: m.Name, Root: filepath.Clean(m.Root), Svc: m.SvcID})
		}
	}

	graph := &impactGraph{
		GeneratedAtUtc: time.Now().UTC().Format(time.RFC3339),
	}
	for _, m := range modules {
		graph.Modules = append(graph.Modules, impactModule{Name: m.Name, Root: m.Root, SvcID: m.Svc})
	}

	var edges []impactEdge
	crawlErr := false
	if regValid {
		var err error
		edges, err = crawlImpactGraph(workspace, modules)
		if err != nil {
			graph.Warnings = append(graph.Warnings, err.Error())
			crawlErr = true
		}
	}
	graph.Edges = edges

	if !regValid || crawlErr {
		rep.Rules = append(rep.Rules, makeRule("1.1.2", "FAIL", "high",
			map[string]interface{}{"impactGraph": "missing"}, nil,
			"Ensure registry modules can be crawled and impact graph generated."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.1.2", "PASS", "high",
			map[string]interface{}{"impactGraph": "generated"}, nil, ""))
	}

	// Rule 1.1.4 (ID references)
	if regValid {
		unknown := findUnknownIDs(workspace, reg.IDs)
		if len(unknown) > 0 {
			rep.Rules = append(rep.Rules, makeRule("1.1.4", "FAIL", "high",
				nil, unknown, "Register referenced IDs in registry ids.API/SVC/DB."))
		} else {
			rep.Rules = append(rep.Rules, makeRule("1.1.4", "PASS", "high", nil, nil, ""))
		}
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.1.4", "FAIL", "high",
			map[string]interface{}{"reason": "registry invalid"}, nil, "Fix registry and rerun scan."))
	}

	// Rule 1.2.1 - UI must route through BFF
	uiFindings := findUIBypass(workspace, config.Scan.UIPaths, config.Scan.BFFPaths)
	if len(uiFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.2.1", "FAIL", "high",
			nil, uiFindings, "Route UI calls through BFF entrypoints."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.2.1", "PASS", "high", nil, nil, ""))
	}

	// Rule 1.2.2 - BFF wrapper enforcement
	missingWrappers := findBFFMissingSignatures(workspace, config.Scan.BFFPaths, config.Scan.WrapperSignatures)
	if len(missingWrappers) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.2.2", "FAIL", "high",
			nil, missingWrappers, "Apply envelope wrapper to all BFF handlers."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.2.2", "PASS", "high", nil, nil, ""))
	}

	// Rule 1.2.3 - BFF permission lookup
	missingPerms := findBFFMissingSignatures(workspace, config.Scan.BFFPaths, config.Scan.PermissionSignatures)
	if len(missingPerms) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.2.3", "FAIL", "high",
			nil, missingPerms, "Add permission checks to BFF handlers."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.2.3", "PASS", "high", nil, nil, ""))
	}

	// Rule 1.2.4 - rollup
	if len(uiFindings) > 0 || len(missingWrappers) > 0 || len(missingPerms) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.2.4", "FAIL", "high",
			nil, nil, "Fix 1.2.1–1.2.3 failures."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.2.4", "PASS", "high", nil, nil, ""))
	}

	// Rule 1.3.1 - module layout
	structFindings, _ := checkModuleStructure(workspace, modules)
	if len(structFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.3.1", "FAIL", "high",
			nil, structFindings, "Add contracts/, internal/, and manifest file for each module."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.3.1", "PASS", "high", nil, nil, ""))
	}

	// Rule 1.3.2 - kernel/bootstrap
	kernelFiles := findKernelFiles(workspace)
	if len(kernelFiles) == 0 {
		rep.Rules = append(rep.Rules, makeRule("1.3.2", "FAIL", "high",
			map[string]interface{}{"kernelPresent": false}, nil, "Add a kernel/bootstrap entry file."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.3.2", "PASS", "high",
			map[string]interface{}{"kernelPresent": true}, nil, ""))
	}

	// Rule 1.3.3 - boot entrypoint + export marker
	bootFindings := checkModuleBoot(workspace, modules)
	if len(bootFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.3.3", "FAIL", "high",
			nil, bootFindings, "Add module boot entrypoint with Boot/Start export."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.3.3", "PASS", "high", nil, nil, ""))
	}

	// Rule 1.3.4 - contracts-only cross-module
	leakFindings := []finding{}
	for i := range graph.Edges {
		edge := &graph.Edges[i]
		if edge.FromModule != edge.ToModule {
			if strings.Contains(edge.ImportPath, "/internal/") || strings.Contains(edge.ImportPath, "\\internal\\") {
				edge.Violates = true
				edge.RuleID = "1.3.4"
				leakFindings = append(leakFindings, finding{
					File:    edge.Reason,
					Message: fmt.Sprintf("cross-module internal import: %s -> %s (%s)", edge.FromModule, edge.ToModule, edge.ImportPath),
				})
			} else if !strings.Contains(edge.ImportPath, "/contracts/") && !strings.Contains(edge.ImportPath, "\\contracts\\") {
				edge.Violates = true
				edge.RuleID = "1.3.4"
				leakFindings = append(leakFindings, finding{
					File:    edge.Reason,
					Message: fmt.Sprintf("cross-module import missing contracts path: %s -> %s (%s)", edge.FromModule, edge.ToModule, edge.ImportPath),
				})
			}
		}
	}
	leakFindings = append(leakFindings, detectInternalImportLeaks(workspace, modules)...)
	if len(leakFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.3.4", "FAIL", "high",
			nil, leakFindings, "Restrict cross-module imports to contracts/."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.3.4", "PASS", "high", nil, nil, ""))
	}

	// Rule 1.3.5 - rollup
	if len(leakFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("1.3.5", "FAIL", "high",
			nil, nil, "Resolve cross-module leaks."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("1.3.5", "PASS", "high", nil, nil, ""))
	}

	// Rule 2.1.1 - ingest admin uses rename + command exists
	ingestFindings, ingestOk := checkIngestImplementation(workspace)
	if !ingestOk {
		rep.Rules = append(rep.Rules, makeRule("2.1.1", "FAIL", "high",
			nil, ingestFindings, "Implement ingest-admin with atomic rename."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("2.1.1", "PASS", "high", nil, nil, ""))
	}

	// Rule 2.1.2 - processing reads only from locked
	incomingFindings := findIncomingUsage(workspace)
	if len(incomingFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("2.1.2", "FAIL", "high",
			nil, incomingFindings, "Ensure processing reads only from locked/."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("2.1.2", "PASS", "high", nil, nil, ""))
	}

	// Rule 2.1.3 - resumable ingest state
	resumeFindings, resumeOk := checkResumeSupport(workspace)
	if !resumeOk {
		rep.Rules = append(rep.Rules, makeRule("2.1.3", "FAIL", "high",
			nil, resumeFindings, "Add --resume and ingest.state.json support."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("2.1.3", "PASS", "high", nil, nil, ""))
	}

	// Rule 2.1.4 - atomic .b2b writes only
	atomicFindings := findNonAtomicB2BWrites(workspace)
	if len(atomicFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("2.1.4", "FAIL", "high",
			nil, atomicFindings, "Use support.WriteFileAtomic for all .b2b writes."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("2.1.4", "PASS", "high", nil, nil, ""))
	}

	// Rule 4.6.1 - atomic safety for .b2b outputs
	if len(atomicFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("4.6.1", "FAIL", "high",
			map[string]interface{}{"sourceRule": "2.1.4"}, atomicFindings, "Ensure all .b2b writes are atomic."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("4.6.1", "PASS", "high", map[string]interface{}{"sourceRule": "2.1.4"}, nil, ""))
	}

	// Rule 2.4.1 - mutation detection
	mutationFindings, mutationCount := detectMutations(workspace, config.Scan.AuditWrapperSignatures)
	rep.Rules = append(rep.Rules, makeRule("2.4.1", "PASS", "medium",
		map[string]interface{}{"mutationCount": mutationCount}, nil, ""))

	// Rule 2.4.2 - naked mutation without audit wrapper
	if len(mutationFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("2.4.2", "FAIL", "high",
			nil, mutationFindings, "Wrap mutations with audit signatures."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("2.4.2", "PASS", "high", nil, nil, ""))
	}

	// Rule 2.4.3 - audit log append-only
	auditFindings, auditOk := checkAuditAppend(workspace)
	if !auditOk {
		rep.Rules = append(rep.Rules, makeRule("2.4.3", "FAIL", "high",
			nil, auditFindings, "Ensure audit log is append-only."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("2.4.3", "PASS", "high", nil, nil, ""))
	}

	// -------------------------
	// Phase 3 rules
	// -------------------------

	// 3.1.1 SVC-ID enforcement per module
	svcViolations := checkModuleSvcIDs(workspace, reg, modules)
	if len(svcViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.1.1", "FAIL", "high",
			nil, svcViolations, "Ensure each module declares a valid SVC-ID."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.1.1", "PASS", "high", nil, nil, ""))
	}

	// 3.1.2 Internal privacy (reuse internal leak findings)
	internalLeaks := filterViolations(leakFindings, "internal")
	if len(internalLeaks) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.1.2", "FAIL", "high",
			nil, internalLeaks, "Remove cross-module imports of internal/."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.1.2", "PASS", "high", nil, nil, ""))
	}

	// 3.1.3 Contracts-only exports (reuse non-contract leaks)
	contractLeaks := filterViolations(leakFindings, "contracts")
	if len(contractLeaks) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.1.3", "FAIL", "high",
			nil, contractLeaks, "Restrict cross-module imports to contracts/."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.1.3", "PASS", "high", nil, nil, ""))
	}

	// 3.1.4 Contract versioning + compatibility
	versionViolations := checkContractVersioning(workspace, modules)
	if len(versionViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.1.4", "FAIL", "high",
			nil, versionViolations, "Add contracts/version.json with valid semver + compatibility."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.1.4", "PASS", "high", nil, nil, ""))
	}

	// 3.2.1 Dealer BFF exists + exclusivity
	dealerBFFHandlers := listHandlerFiles(workspace, config.Scan.DealerBFFPaths)
	dealerBypass := findUIBypassWithRoots(workspace, config.Scan.DealerUIPaths, config.Scan.DealerBFFPaths)
	if len(dealerBFFHandlers) == 0 || len(dealerBypass) > 0 {
		violations := dealerBypass
		if len(dealerBFFHandlers) == 0 {
			violations = append(violations, finding{Message: "dealer BFF handlers missing"})
		}
		rep.Rules = append(rep.Rules, makeRule("3.2.1", "FAIL", "high",
			map[string]interface{}{"dealerBffHandlers": len(dealerBFFHandlers)}, violations,
			"Add dealer BFF handlers and route dealer UI through them."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.2.1", "PASS", "high",
			map[string]interface{}{"dealerBffHandlers": len(dealerBFFHandlers)}, nil, ""))
	}

	// 3.2.2 Admin BFF exists + exclusivity
	adminBFFHandlers := listHandlerFiles(workspace, config.Scan.AdminBFFPaths)
	adminBypass := findUIBypassWithRoots(workspace, config.Scan.AdminUIPaths, config.Scan.AdminBFFPaths)
	if len(adminBFFHandlers) == 0 || len(adminBypass) > 0 {
		violations := adminBypass
		if len(adminBFFHandlers) == 0 {
			violations = append(violations, finding{Message: "admin BFF handlers missing"})
		}
		rep.Rules = append(rep.Rules, makeRule("3.2.2", "FAIL", "high",
			map[string]interface{}{"adminBffHandlers": len(adminBFFHandlers)}, violations,
			"Add admin BFF handlers and route admin UI through them."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.2.2", "PASS", "high",
			map[string]interface{}{"adminBffHandlers": len(adminBFFHandlers)}, nil, ""))
	}

	// 3.2.3 Contract specs exist + parse-valid
	specViolations := checkContractSpecs(workspace, modules)
	if len(specViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.2.3", "FAIL", "high",
			nil, specViolations, "Add and validate OpenAPI/Zod/schema contracts per module."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.2.3", "PASS", "high", nil, nil, ""))
	}

	// 3.2.4 UI cannot call DB/Repo/Prisma
	uiForbidden := findUIForbiddenImports(workspace, append(config.Scan.UIPaths, append(config.Scan.DealerUIPaths, config.Scan.AdminUIPaths...)...))
	if len(uiForbidden) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.2.4", "FAIL", "high",
			nil, uiForbidden, "Remove direct DB/Repo/Prisma imports from UI."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.2.4", "PASS", "high", nil, nil, ""))
	}

	// 3.2.5 Envelope usage at BFF
	dealerMissingWrap := findBFFMissingSignatures(workspace, config.Scan.DealerBFFPaths, config.Scan.WrapperSignatures)
	adminMissingWrap := findBFFMissingSignatures(workspace, config.Scan.AdminBFFPaths, config.Scan.WrapperSignatures)
	envViolations := append([]finding{}, dealerMissingWrap...)
	envViolations = append(envViolations, adminMissingWrap...)
	if len(dealerBypass) > 0 || len(adminBypass) > 0 {
		envViolations = append(envViolations, finding{Message: "ui bypasses BFF"})
	}
	if len(envViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.2.5", "FAIL", "high",
			nil, envViolations, "Ensure BFF handlers use envelope wrappers and UI routes through BFF."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.2.5", "PASS", "high", nil, nil, ""))
	}

	// API route ID enforcement (Ghost routes)
	apiRoutes := extractAPIRoutes(workspace, append(config.Scan.DealerBFFPaths, config.Scan.AdminBFFPaths...))
	apiIDSet := apiIDSet(reg)
	ghostViolations := []finding{}
	unknownViolations := []finding{}
	ghostCount := 0
	unknownCount := 0
	for _, route := range apiRoutes {
		resolved := resolveAPIID(route.APIID, apiIDSet)
		if resolved == "" {
			ghostCount++
			ghostViolations = append(ghostViolations, finding{
				RuleID:  "API_ROUTE_ID_REQUIRED",
				File:    route.File,
				Line:    route.Line,
				Message: "route missing API-ID declaration",
			})
			continue
		}
		if _, ok := apiIDSet[resolved]; !ok {
			unknownCount++
			unknownViolations = append(unknownViolations, finding{
				RuleID:  "API_ROUTE_ID_UNKNOWN",
				File:    route.File,
				Line:    route.Line,
				Message: fmt.Sprintf("API-ID not found in registry: %s", resolved),
			})
		}
	}
	rep.Rules = append(rep.Rules, makeRule("API_ROUTE_ID_REQUIRED", statusFromBool(ghostCount == 0), "high",
		map[string]interface{}{"ghostRouteCount": ghostCount}, ghostViolations, "Declare a valid API-ID for each BFF route."))
	rep.Rules = append(rep.Rules, makeRule("API_ROUTE_ID_UNKNOWN", statusFromBool(unknownCount == 0), "high",
		map[string]interface{}{"unknownApiIdCount": unknownCount}, unknownViolations, "Register the API-ID under ids.API in the registry."))

	// 3.3.1 UI registry validation
	registry, regViolations := loadUIRegistry(workspace)
	if len(regViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.3.1", "FAIL", "high",
			nil, regViolations, "Create ui/registry.json with svcId entries."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.3.1", "PASS", "high", nil, nil, ""))
	}

	// 3.3.2 Inventory mapping coverage
	inventory := buildUIInventory(workspace)
	mapped, unmapped, coverage := mapRegistryCoverage(registry, inventory)
	status332 := "PASS"
	if registry == nil {
		status332 = "FAIL"
	}
	rep.Rules = append(rep.Rules, makeRule("3.3.2", status332, "medium",
		map[string]interface{}{
			"uiRegistryMappedCount": mapped,
			"uiRegistryTotalCount":  len(inventory),
			"uiRegistryCoveragePct": coverage,
		}, nil, "Map UI components in ui/registry.json."))

	// 3.3.3 Critical unmapped enforcement
	criticalUnmapped := filterCriticalUnmapped(unmapped, config.Scan.UIRegistryCriticalPatterns)
	if len(criticalUnmapped) > 0 {
		status := "FAIL"
		severity := "high"
		if strings.ToLower(config.Scan.UIRegistryEnforcement) == "warn" {
			status = "WARN"
			severity = "medium"
		}
		rep.Rules = append(rep.Rules, makeRule("3.3.3", status, severity,
			map[string]interface{}{"enforcement": config.Scan.UIRegistryEnforcement}, criticalUnmapped,
			"Map critical UI components in ui/registry.json."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.3.3", "PASS", "medium",
			map[string]interface{}{"enforcement": config.Scan.UIRegistryEnforcement}, nil, ""))
	}

	// 3.3.4 HUD coverage row
	status334 := "PASS"
	if registry == nil {
		status334 = "FAIL"
	}
	rep.Rules = append(rep.Rules, makeRule("3.3.4", status334, "low",
		map[string]interface{}{
			"uiRegistryMappedCount": mapped,
			"uiRegistryTotalCount":  len(inventory),
			"uiRegistryCoveragePct": coverage,
		}, nil, ""))

	// 3.4.1 Dealer contracts include LLID fields
	llidViolations := checkDealerLLID(workspace, modules)
	if len(llidViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.4.1", "FAIL", "high",
			nil, llidViolations, "Add LLID fields to dealer contracts."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.4.1", "PASS", "high", nil, nil, ""))
	}

	// 3.4.2 Dealer responses include LLID
	if len(llidViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.4.2", "FAIL", "high",
			nil, llidViolations, "Include LLID fields in dealer-facing DTOs."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.4.2", "PASS", "high", nil, nil, ""))
	}

	// 3.4.3 Mutation LLID enforcement continues
	if len(mutationFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("3.4.3", "FAIL", "high",
			map[string]interface{}{"sourceRule": "2.4.2"}, nil, "Resolve mutation audit failures."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("3.4.3", "PASS", "high",
			map[string]interface{}{"sourceRule": "2.4.2"}, nil, ""))
	}

	// Dealer UI LLID display enforcement
	dataPages, llidMissing, llidCoverage := checkDealerUiLLIDDisplay(workspace)
	statusLLID := "PASS"
	if len(llidMissing) > 0 && strings.ToLower(config.Scan.UILLIDDisplayEnforcement) == "fail" {
		statusLLID = "FAIL"
	}
	rep.Rules = append(rep.Rules, makeRule("DEALER_UI_LLID_DISPLAY_REQUIRED", statusLLID, "high",
		map[string]interface{}{
			"llidDisplayCoveragePct": llidCoverage,
			"dealerDataPages":        dataPages,
			"missingLlidDisplay":     len(llidMissing),
		}, llidMissing, "Render LLID/trace markers in dealer UI pages that show dealer data."))

	// 4.5.1/4.5.2 external gateway forbidden
	externalFindings := scanExternalGateway(workspace)
	if len(externalFindings) > 0 {
		rep.Rules = append(rep.Rules, makeRule("4.5.1", "FAIL", "high",
			nil, externalFindings, "Remove external gateway routes/keys; internal gateway only."))
		rep.Rules = append(rep.Rules, makeRule("4.5.2", "FAIL", "high",
			nil, externalFindings, "Remove external keys/HMAC usage from code/config."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("4.5.1", "PASS", "high", nil, nil, ""))
		rep.Rules = append(rep.Rules, makeRule("4.5.2", "PASS", "high", nil, nil, ""))
	}

	// 4.5.3/4.5.4 internal routing references (pass-through)
	rep.Rules = append(rep.Rules, makeRule("4.5.3", "PASS", "medium",
		map[string]interface{}{"sourceRules": []string{"1.2.1", "3.2.5", "1.3.4"}}, nil, ""))
	rep.Rules = append(rep.Rules, makeRule("4.5.4", "PASS", "medium",
		map[string]interface{}{"sourceRules": []string{"1.3.4", "3.1.2", "3.1.3"}}, nil, ""))

	// 4.1.4 validation of violation payloads
	payloadViolations := validateViolationPayloads(rep)
	if len(payloadViolations) > 0 {
		rep.Rules = append(rep.Rules, makeRule("4.1.4", "FAIL", "high",
			nil, payloadViolations, "Ensure violations include ruleId, file, line, message, fixHint."))
	} else {
		rep.Rules = append(rep.Rules, makeRule("4.1.4", "PASS", "high", nil, nil, ""))
	}

	doctorReport := buildDoctorReport()

	// 4.6.2 doctor diagnostics + readiness
	status462 := "PASS"
	if doctorReport.Status != "OK" {
		status462 = "FAIL"
	}
	rep.Rules = append(rep.Rules, makeRule("4.6.2", status462, "high", map[string]interface{}{
		"doctorStatus": doctorReport.Status,
		"reasons":      doctorReport.Reasons,
	}, nil, "Ensure doctor readiness checks pass."))

	// 4.6.3 support bundle available
	rep.Rules = append(rep.Rules, makeRule("4.6.3", "PASS", "low", map[string]interface{}{
		"supportBundleAvailable": true,
	}, nil, ""))

	// 4.6.4 setup resume available
	rep.Rules = append(rep.Rules, makeRule("4.6.4", "PASS", "low", map[string]interface{}{
		"setupAvailable": true,
	}, nil, ""))

	// Rollback readiness
	backupIDs := listBackupSnapshots(workspace)
	rollbackStatus := "FAIL"
	latestBackup := ""
	if len(backupIDs) > 0 {
		rollbackStatus = "PASS"
		latestBackup = backupIDs[len(backupIDs)-1]
	}
	rep.Rules = append(rep.Rules, makeRule("ROLLBACK_READY", rollbackStatus, "high", map[string]interface{}{
		"backupCount":  len(backupIDs),
		"latestBackup": latestBackup,
	}, nil, "Create a GREEN/PASS snapshot before attempting rollback."))

	rep.Phase1Status = phaseStatus(rep.Rules)

	rep.Phase2Status = phase2Status(rep.Rules)
	rep.Phase3Status = phase3Status(rep.Rules)
	rep.Phase4Status = phase4Status(rep.Rules)

	writeJSON(filepath.Join(outputDir, "report.json"), rep)
	writeReportHTML(workspace)
	writeJSON(filepath.Join(outputDir, "results.json"), buildScanResults(rep))
	writeJSON(filepath.Join(outputDir, "impact-graph.json"), graph)
	writeJSON(filepath.Join(outputDir, "api-routes.json"), apiRoutes)
	writeJSON(filepath.Join(outputDir, "doctor.json"), doctorReport)

	_ = support.AppendAudit(workspace, support.AuditEntry{
		Mode:         "scan",
		Phase1Status: rep.Phase1Status,
		Phase2Status: rep.Phase2Status,
		Phase3Status: rep.Phase3Status,
		Phase4Status: rep.Phase4Status,
	})
}

func loadRegistry(workspace string) (*registry, string, bool) {
	paths := []string{
		filepath.Join(workspace, "main-index.json"),
		filepath.Join(workspace, ".b2b", "main-index.json"),
		filepath.Join(workspace, "registry", "main-index.json"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, p, false
			}
			var reg registry
			if err := json.Unmarshal(support.StripBOM(data), &reg); err != nil {
				return nil, p, false
			}
			if reg.Version == "" || reg.Modules == nil || reg.IDs == nil {
				return &reg, p, false
			}
			return &reg, p, true
		}
	}
	return nil, "", false
}

func hasNamespace(ids map[string]interface{}, key string) bool {
	if ids == nil {
		return false
	}
	_, ok := ids[key]
	return ok
}

func findUnknownIDs(workspace string, ids map[string]interface{}) []finding {
	reg := flattenIDs(ids)
	results := []finding{}
	re := regexp.MustCompile(`\b(API|SVC|DB)_IDS\.(\w+)`)
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if isDotB2B(path) {
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			matches := re.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				ns := m[1]
				id := m[2]
				if !idExists(reg, ns, id) {
					results = append(results, finding{
						File: path,
						Line: i + 1,
						ID:   fmt.Sprintf("%s.%s", ns, id),
					})
				}
			}
		}
		return nil
	})
	return results
}

func flattenIDs(ids map[string]interface{}) map[string]map[string]struct{} {
	out := map[string]map[string]struct{}{}
	for ns, raw := range ids {
		out[ns] = map[string]struct{}{}
		switch v := raw.(type) {
		case map[string]interface{}:
			for k := range v {
				out[ns][k] = struct{}{}
			}
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					out[ns][s] = struct{}{}
				}
			}
		}
	}
	return out
}

func idExists(reg map[string]map[string]struct{}, ns, id string) bool {
	if _, ok := reg[ns]; !ok {
		return false
	}
	_, ok := reg[ns][id]
	return ok
}

func crawlImpactGraph(workspace string, modules []moduleRoot) ([]impactEdge, error) {
	edges := []impactEdge{}
	files := []moduleFile{}

	for _, m := range modules {
		root := filepath.Join(workspace, m.Root)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !isSourceFile(path) {
				return nil
			}
			files = append(files, moduleFile{Path: path, Module: m.Name})
			return nil
		})
	}

	for _, f := range files {
		imports := extractImports(f.Path)
		for _, imp := range imports {
			to := resolveImportModule(workspace, modules, f.Path, imp.Path)
			if to == "" {
				continue
			}
			edges = append(edges, impactEdge{
				FromModule: f.Module,
				ToModule:   to,
				Reason:     f.Path,
				ImportPath: imp.Path,
			})
		}
	}

	return edges, nil
}

func extractImports(path string) []importRef {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	imports := []importRef{}
	reImport := regexp.MustCompile(`(?m)^\s*import\s+.*from\s+["']([^"']+)["']`)
	reSideEffect := regexp.MustCompile(`(?m)^\s*import\s+["']([^"']+)["']`)
	reRequire := regexp.MustCompile(`require\(["']([^"']+)["']\)`)
	for i, line := range lines {
		if m := reImport.FindStringSubmatch(line); len(m) == 2 {
			imports = append(imports, importRef{Path: m[1], Line: i + 1})
		}
		if m := reSideEffect.FindStringSubmatch(line); len(m) == 2 {
			imports = append(imports, importRef{Path: m[1], Line: i + 1})
		}
		if m := reRequire.FindStringSubmatch(line); len(m) == 2 {
			imports = append(imports, importRef{Path: m[1], Line: i + 1})
		}
	}
	return imports
}

func resolveImportModule(workspace string, modules []moduleRoot, fromFile, importPath string) string {
	if strings.HasPrefix(importPath, ".") {
		base := filepath.Dir(fromFile)
		target := filepath.Clean(filepath.Join(base, importPath))
		for _, m := range modules {
			root := filepath.Join(workspace, m.Root)
			if isPathWithin(target, root) {
				return m.Name
			}
		}
		return ""
	}
	for _, m := range modules {
		if strings.HasPrefix(importPath, m.Name+"/") || strings.HasPrefix(importPath, m.Root+"/") {
			return m.Name
		}
	}
	return ""
}

func isPathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func findUIBypass(workspace string, uiPaths, bffPaths []string) []finding {
	results := []finding{}
	for _, rel := range uiPaths {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !isSourceFile(path) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				imports := extractImportsFromLine(line)
				for _, imp := range imports {
					if isAllowedBFFImport(imp, bffPaths) {
						continue
					}
					if isForbiddenUIImport(imp) {
						results = append(results, finding{
							File:    path,
							Line:    i + 1,
							Message: fmt.Sprintf("ui import bypasses bff: %s", imp),
						})
					}
				}
			}
			return nil
		})
	}
	return results
}

func isAllowedBFFImport(imp string, bffPaths []string) bool {
	for _, p := range bffPaths {
		if strings.Contains(imp, p) {
			return true
		}
	}
	return false
}

func isForbiddenUIImport(imp string) bool {
	lower := strings.ToLower(imp)
	if strings.Contains(lower, "repo") || strings.Contains(lower, "prisma") || strings.Contains(lower, "db") || strings.Contains(lower, "server") {
		return true
	}
	if strings.Contains(lower, "/internal/") || strings.Contains(lower, "\\internal\\") {
		return true
	}
	return false
}

func extractImportsFromLine(line string) []string {
	out := []string{}
	reImport := regexp.MustCompile(`import\s+.*from\s+["']([^"']+)["']`)
	reSideEffect := regexp.MustCompile(`import\s+["']([^"']+)["']`)
	reRequire := regexp.MustCompile(`require\(["']([^"']+)["']\)`)
	if m := reImport.FindStringSubmatch(line); len(m) == 2 {
		out = append(out, m[1])
	}
	if m := reSideEffect.FindStringSubmatch(line); len(m) == 2 {
		out = append(out, m[1])
	}
	if m := reRequire.FindStringSubmatch(line); len(m) == 2 {
		out = append(out, m[1])
	}
	return out
}

func findBFFMissingSignatures(workspace string, bffPaths, signatures []string) []finding {
	results := []finding{}
	for _, rel := range bffPaths {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !isSourceFile(path) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			text := string(data)
			has := false
			for _, sig := range signatures {
				if strings.Contains(text, sig) {
					has = true
					break
				}
			}
			if !has {
				results = append(results, finding{
					File:    path,
					Message: "missing required signature",
				})
			}
			return nil
		})
	}
	return results
}

func checkModuleStructure(workspace string, modules []moduleRoot) ([]finding, bool) {
	findings := []finding{}
	for _, m := range modules {
		root := filepath.Join(workspace, m.Root)
		contracts := filepath.Join(root, "contracts")
		internal := filepath.Join(root, "internal")
		manifestOK := false
		for _, mf := range []string{"module.json", "service.json", "package.json"} {
			if _, err := os.Stat(filepath.Join(root, mf)); err == nil {
				manifestOK = true
				break
			}
		}
		if _, err := os.Stat(contracts); err != nil {
			findings = append(findings, finding{File: root, Message: "missing contracts/ directory"})
		}
		if _, err := os.Stat(internal); err != nil {
			findings = append(findings, finding{File: root, Message: "missing internal/ directory"})
		}
		if !manifestOK {
			findings = append(findings, finding{File: root, Message: "missing manifest (module.json/service.json/package.json)"})
		}
	}
	return findings, len(findings) == 0
}

func findKernelFiles(workspace string) []string {
	candidates := []string{
		"kernel.go",
		"kernel.ts",
		"bootstrap.go",
		"bootstrap.ts",
	}
	found := []string{}
	for _, c := range candidates {
		path := filepath.Join(workspace, c)
		if _, err := os.Stat(path); err == nil {
			found = append(found, c)
		}
	}
	_ = filepath.Walk(filepath.Join(workspace, "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "kernel.") || strings.HasPrefix(base, "bootstrap.") {
			rel, _ := filepath.Rel(workspace, path)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	return uniqueStrings(found)
}

func checkModuleBoot(workspace string, modules []moduleRoot) []finding {
	findings := []finding{}
	entrypoints := []string{"main.go", "main.ts", "index.ts", "boot.ts"}
	markers := []string{"func Boot", "func Start", "export function boot", "export const start"}
	for _, m := range modules {
		root := filepath.Join(workspace, m.Root)
		entry := ""
		for _, ep := range entrypoints {
			p := filepath.Join(root, ep)
			if _, err := os.Stat(p); err == nil {
				entry = p
				break
			}
		}
		if entry == "" {
			findings = append(findings, finding{File: root, Message: "missing boot entrypoint"})
			continue
		}
		data, err := os.ReadFile(entry)
		if err != nil {
			findings = append(findings, finding{File: entry, Message: "failed to read entrypoint"})
			continue
		}
		text := string(data)
		has := false
		for _, m := range markers {
			if strings.Contains(text, m) {
				has = true
				break
			}
		}
		if !has {
			findings = append(findings, finding{File: entry, Message: "missing boot export marker"})
		}
	}
	return findings
}

func phaseStatus(rules []ruleResult) string {
	for _, r := range rules {
		if strings.HasPrefix(r.RuleID, "1.") && r.Status == "FAIL" {
			return "FAIL"
		}
	}
	return "PASS"
}

func phase2Status(rules []ruleResult) string {
	for _, r := range rules {
		if strings.HasPrefix(r.RuleID, "2.") && r.Status == "FAIL" {
			return "FAIL"
		}
	}
	return "PASS"
}

func phase3Status(rules []ruleResult) string {
	for _, r := range rules {
		if strings.HasPrefix(r.RuleID, "3.") && r.Status == "FAIL" {
			return "FAIL"
		}
	}
	return "PASS"
}

func phase4Status(rules []ruleResult) string {
	for _, r := range rules {
		if strings.HasPrefix(r.RuleID, "4.") && r.Status == "FAIL" {
			return "FAIL"
		}
	}
	return "PASS"
}
func writeJSON(path string, v interface{}) {
	if err := support.WriteJSONAtomic(path, v); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot write %s: %v\n", path, err)
	}
}

func buildScanResults(rep *report) *ScanResults {
	results := &ScanResults{
		Red:   []Violation{},
		Amber: []Violation{},
		Green: []Violation{},
	}
	for _, r := range rep.Rules {
		status := strings.ToUpper(r.Status)
		var target *[]Violation
		switch status {
		case "FAIL":
			target = &results.Red
		case "WARN":
			target = &results.Amber
		case "PASS":
			target = &results.Green
		default:
			continue
		}
		if len(r.Violations) > 0 {
			for _, v := range r.Violations {
				*target = append(*target, Violation{
					Rule:    r.RuleID,
					Message: v.Message,
					File:    v.File,
					Line:    v.Line,
				})
			}
			continue
		}
		if status == "FAIL" || status == "WARN" {
			msg := r.Message
			if msg == "" {
				msg = r.FixHint
			}
			if msg == "" {
				msg = "Rule reported without violation details"
			}
			*target = append(*target, Violation{
				Rule:    r.RuleID,
				Message: msg,
			})
		}
	}
	return results
}

func makeRule(ruleID, status, severity string, evidence map[string]interface{}, violations []finding, fixHint string) ruleResult {
	if fixHint == "" {
		fixHint = "Review rule documentation and apply the required fixes."
	}
	for i := range violations {
		if violations[i].RuleID == "" {
			violations[i].RuleID = ruleID
		}
		if violations[i].File == "" {
			violations[i].File = "unknown"
		}
		if violations[i].Line <= 0 {
			violations[i].Line = 1
		}
		if violations[i].Message == "" {
			violations[i].Message = "violation"
		}
		violations[i].FixHint = fixHint
	}
	return ruleResult{
		RuleID:     ruleID,
		Status:     status,
		Severity:   severity,
		Evidence:   evidence,
		Violations: violations,
		FixHint:    fixHint,
	}
}

func upsertRule(rules []ruleResult, next ruleResult) []ruleResult {
	for i, r := range rules {
		if r.RuleID == next.RuleID {
			rules[i] = next
			return rules
		}
	}
	return append(rules, next)
}

func statusFromBool(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx":
		return true
	default:
		return false
	}
}

func isDotB2B(path string) bool {
	return strings.Contains(path, string(filepath.Separator)+".b2b"+string(filepath.Separator))
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func checkIngestImplementation(workspace string) ([]finding, bool) {
	findings := []finding{}
	hasCommand := false
	hasRename := false
	hasCopy := false
	var ingestFiles []string

	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)
		if strings.Contains(text, "ingest-admin") {
			hasCommand = true
			ingestFiles = append(ingestFiles, path)
		}
		if strings.Contains(text, "runIngestAdmin") {
			ingestFiles = append(ingestFiles, path)
		}
		return nil
	})

	for _, path := range uniqueStrings(ingestFiles) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if strings.Contains(text, "os.Rename(") {
			hasRename = true
		}
		if strings.Contains(text, "io.Copy(") || strings.Contains(text, "os.WriteFile(") {
			hasCopy = true
		}
	}

	if !hasCommand {
		findings = append(findings, finding{Message: "ingest-admin command missing"})
	}
	if !hasRename {
		findings = append(findings, finding{Message: "ingest does not use os.Rename"})
	}
	if hasCopy {
		findings = append(findings, finding{Message: "ingest uses copy/write instead of rename"})
	}
	return findings, len(findings) == 0
}

func checkResumeSupport(workspace string) ([]finding, bool) {
	findings := []finding{}
	hasResume := false
	hasState := false
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)
		if strings.Contains(text, "--resume") {
			hasResume = true
		}
		if strings.Contains(text, "ingest.state.json") {
			hasState = true
		}
		return nil
	})
	if !hasResume {
		findings = append(findings, finding{Message: "missing --resume support"})
	}
	if !hasState {
		findings = append(findings, finding{Message: "missing ingest.state.json usage"})
	}
	return findings, len(findings) == 0
}

func findIncomingUsage(workspace string) []finding {
	results := []finding{}
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		if strings.Contains(path, "ingest") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, "/incoming") || strings.Contains(line, "incoming/") {
				results = append(results, finding{
					File:    path,
					Line:    i + 1,
					Message: "incoming path usage detected",
				})
			}
		}
		return nil
	})
	return results
}

func findNonAtomicB2BWrites(workspace string) []finding {
	results := []finding{}
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"internal"+string(filepath.Separator)+"support") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, "os.WriteFile(") && strings.Contains(line, ".b2b") {
				results = append(results, finding{
					File:    path,
					Line:    i + 1,
					Message: "non-atomic .b2b write",
				})
			}
		}
		return nil
	})
	return results
}

func detectMutations(workspace string, auditSignatures []string) ([]finding, int) {
	findings := []finding{}
	count := 0
	reMut := regexp.MustCompile(`\.(create|update|delete)\b|\$transaction\b|(?i)\b(INSERT|UPDATE|DELETE)\b`)
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if reMut.MatchString(line) {
				count++
				if !hasAuditMarker(lines, i, auditSignatures) {
					findings = append(findings, finding{
						File:    path,
						Line:    i + 1,
						Message: strings.TrimSpace(line),
					})
				}
			}
		}
		return nil
	})
	return findings, count
}

func hasAuditMarker(lines []string, index int, signatures []string) bool {
	start := index - 3
	if start < 0 {
		start = 0
	}
	end := index + 3
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for i := start; i <= end; i++ {
		for _, sig := range signatures {
			if strings.Contains(lines[i], sig) {
				return true
			}
		}
	}
	return false
}

func checkAuditAppend(workspace string) ([]finding, bool) {
	findings := []finding{}
	hasAudit := false
	hasAppend := false
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)
		if strings.Contains(text, "audit.log") {
			hasAudit = true
		}
		if strings.Contains(text, "O_APPEND") {
			hasAppend = true
		}
		return nil
	})
	if !hasAudit {
		findings = append(findings, finding{Message: "audit.log not referenced"})
	}
	if !hasAppend {
		findings = append(findings, finding{Message: "append-only mode not detected"})
	}
	return findings, len(findings) == 0
}

func detectInternalImportLeaks(workspace string, modules []moduleRoot) []finding {
	findings := []finding{}
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		fromModule := moduleForPath(workspace, modules, path)
		if fromModule == "" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			imps := extractImportsFromLine(line)
			for _, imp := range imps {
				if !strings.Contains(imp, "/internal/") && !strings.Contains(imp, "\\internal\\") {
					continue
				}
				toModule := resolveImportModule(workspace, modules, path, imp)
				if toModule != "" && toModule != fromModule {
					findings = append(findings, finding{
						File:    path,
						Line:    i + 1,
						Message: fmt.Sprintf("cross-module internal import: %s -> %s (%s)", fromModule, toModule, imp),
					})
				}
			}
		}
		return nil
	})
	return findings
}

func moduleForPath(workspace string, modules []moduleRoot, path string) string {
	for _, m := range modules {
		root := filepath.Join(workspace, m.Root)
		if isPathWithin(path, root) {
			return m.Name
		}
	}
	return ""
}

func checkModuleSvcIDs(workspace string, reg *registry, modules []moduleRoot) []finding {
	violations := []finding{}
	svcIDs := map[string]struct{}{}
	if reg != nil {
		if raw, ok := reg.IDs["SVC"]; ok {
			switch v := raw.(type) {
			case map[string]interface{}:
				for k := range v {
					svcIDs[k] = struct{}{}
				}
			case []interface{}:
				for _, item := range v {
					if s, ok := item.(string); ok {
						svcIDs[s] = struct{}{}
					}
				}
			}
		}
	}
	for _, m := range modules {
		svcID := m.Svc
		if svcID == "" {
			svcID = readSvcFromManifest(filepath.Join(workspace, m.Root))
		}
		if svcID == "" {
			violations = append(violations, finding{File: m.Root, Message: "missing svcId"})
			continue
		}
		if _, ok := svcIDs[svcID]; !ok {
			violations = append(violations, finding{File: m.Root, Message: fmt.Sprintf("unknown svcId: %s", svcID)})
		}
	}
	return violations
}

func readSvcFromManifest(moduleRoot string) string {
	for _, mf := range []string{"module.json", "service.json", "package.json"} {
		path := filepath.Join(moduleRoot, mf)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(support.StripBOM(data), &raw); err != nil {
			continue
		}
		if v, ok := raw["svcId"].(string); ok {
			return v
		}
	}
	return ""
}

func filterViolations(all []finding, kind string) []finding {
	out := []finding{}
	for _, v := range all {
		if kind == "internal" && strings.Contains(v.Message, "internal") {
			out = append(out, v)
		}
		if kind == "contracts" && strings.Contains(v.Message, "contracts") {
			out = append(out, v)
		}
	}
	return out
}

func checkContractVersioning(workspace string, modules []moduleRoot) []finding {
	violations := []finding{}
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	allowed := map[string]struct{}{}
	for _, v := range config.Scan.CompatibilityAllowedValues {
		allowed[v] = struct{}{}
	}
	for _, m := range modules {
		path := filepath.Join(workspace, m.Root, config.Scan.ContractVersionFile)
		data, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, finding{File: m.Root, Message: "missing contracts/version.json"})
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(support.StripBOM(data), &raw); err != nil {
			violations = append(violations, finding{File: path, Message: "invalid version.json"})
			continue
		}
		moduleVal, _ := raw["module"].(string)
		versionVal, _ := raw["version"].(string)
		compatVal, _ := raw["compatibility"].(string)
		if moduleVal == "" || versionVal == "" || compatVal == "" {
			violations = append(violations, finding{File: path, Message: "missing required fields"})
			continue
		}
		if !semver.MatchString(versionVal) {
			violations = append(violations, finding{File: path, Message: "invalid semver"})
		}
		if _, ok := allowed[compatVal]; !ok {
			violations = append(violations, finding{File: path, Message: "invalid compatibility value"})
		}
	}
	return violations
}

func listHandlerFiles(workspace string, roots []string) []string {
	files := []string{}
	for _, rel := range roots {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !isSourceFile(path) {
				return nil
			}
			files = append(files, path)
			return nil
		})
	}
	return files
}

func findUIBypassWithRoots(workspace string, roots, allowedBffPaths []string) []finding {
	results := []finding{}
	for _, rel := range roots {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !isSourceFile(path) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				imports := extractImportsFromLine(line)
				for _, imp := range imports {
					if isAllowedBFFImport(imp, allowedBffPaths) {
						continue
					}
					if isForbiddenUIImport(imp) || strings.Contains(imp, "/internal/") || strings.Contains(imp, "\\internal\\") {
						results = append(results, finding{
							File:    path,
							Line:    i + 1,
							Message: fmt.Sprintf("ui import bypasses bff: %s", imp),
						})
					}
				}
			}
			return nil
		})
	}
	return results
}

func checkContractSpecs(workspace string, modules []moduleRoot) []finding {
	violations := []finding{}
	for _, m := range modules {
		root := filepath.Join(workspace, m.Root)
		versionPath := filepath.Join(root, config.Scan.ContractVersionFile)
		if _, err := os.Stat(versionPath); err != nil {
			violations = append(violations, finding{File: m.Root, Message: "missing contract version.json"})
		}
		specs := findSpecsInModule(root, config.Scan.ContractSpecGlobs)
		if len(specs) == 0 {
			violations = append(violations, finding{File: m.Root, Message: "no contract spec found"})
			continue
		}
		valid := false
		for _, spec := range specs {
			if validateContractSpec(spec) {
				valid = true
				break
			}
		}
		if !valid {
			violations = append(violations, finding{File: m.Root, Message: "contract spec invalid"})
		}
	}
	return violations
}

func findSpecsInModule(root string, patterns []string) []string {
	found := []string{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		for _, p := range patterns {
			if matchGlob(rel, p) {
				found = append(found, path)
				break
			}
		}
		return nil
	})
	return found
}

func matchGlob(path, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, "\\*\\*", ".*")
	escaped = strings.ReplaceAll(escaped, "\\*", "[^/]*")
	re := regexp.MustCompile("^" + escaped + "$")
	if re.MatchString(path) {
		return true
	}
	ok, _ := filepath.Match(pattern, path)
	return ok
}

func validateContractSpec(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	switch ext {
	case ".yaml", ".yml":
		var raw map[string]interface{}
		if err := yaml.Unmarshal(support.StripBOM(data), &raw); err != nil {
			return false
		}
		_, okOpen := raw["openapi"]
		_, okSwagger := raw["swagger"]
		return okOpen || okSwagger
	case ".json":
		var raw map[string]interface{}
		if err := json.Unmarshal(support.StripBOM(data), &raw); err != nil {
			return false
		}
		_, okOpen := raw["openapi"]
		_, okSwagger := raw["swagger"]
		return okOpen || okSwagger || len(raw) > 0
	default:
		text := string(data)
		return strings.Contains(text, "export") || strings.Contains(text, "zod") || strings.Contains(text, "z.")
	}
}

func findUIForbiddenImports(workspace string, roots []string) []finding {
	results := []finding{}
	for _, rel := range roots {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !isSourceFile(path) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if strings.Contains(line, "@prisma/client") || strings.Contains(strings.ToLower(line), "prisma") ||
					strings.Contains(strings.ToLower(line), "/repo/") || strings.Contains(strings.ToLower(line), "repository") ||
					strings.Contains(strings.ToLower(line), "/db/") || strings.Contains(strings.ToLower(line), "database") ||
					strings.Contains(strings.ToLower(line), "select ") || strings.Contains(strings.ToLower(line), "insert ") ||
					strings.Contains(strings.ToLower(line), "update ") || strings.Contains(strings.ToLower(line), "delete ") {
					results = append(results, finding{
						File:    path,
						Line:    i + 1,
						Message: strings.TrimSpace(line),
					})
				}
			}
			return nil
		})
	}
	return results
}

func loadUIRegistry(workspace string) (map[string]map[string]interface{}, []finding) {
	path := filepath.Join(workspace, config.Scan.UIRegistryPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []finding{{File: path, Message: "ui registry missing"}}
	}
	var raw map[string]map[string]interface{}
	if err := json.Unmarshal(support.StripBOM(data), &raw); err != nil {
		return nil, []finding{{File: path, Message: "ui registry invalid JSON"}}
	}
	violations := []finding{}
	for k, v := range raw {
		if _, ok := v["svcId"].(string); !ok {
			violations = append(violations, finding{File: path, Message: fmt.Sprintf("missing svcId for %s", k)})
		}
	}
	if len(violations) > 0 {
		return raw, violations
	}
	return raw, nil
}

func buildUIInventory(workspace string) []string {
	roots := append([]string{}, config.Scan.UIPaths...)
	roots = append(roots, config.Scan.DealerUIPaths...)
	roots = append(roots, config.Scan.AdminUIPaths...)

	globs := config.Scan.UIInventoryGlobs
	if len(globs) == 0 {
		switch strings.ToLower(config.Scan.UIInventoryFramework) {
		case "react-router":
			globs = []string{"**/routes/**/*.*", "**/components/**/*.*"}
		case "generic":
			globs = []string{"**/*.*"}
		default:
			globs = []string{"**/app/**/page.*", "**/pages/**/*.*", "**/components/**/*.*"}
		}
	}

	items := []string{}
	for _, rel := range roots {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.Contains(path, "node_modules") || strings.Contains(path, "dist") || strings.Contains(path, "build") {
				return nil
			}
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)
			for _, g := range globs {
				if matchGlob(relPath, g) {
					items = append(items, filepath.ToSlash(filepath.Join(rel, relPath)))
					break
				}
			}
			return nil
		})
	}
	return uniqueStrings(items)
}

func mapRegistryCoverage(registry map[string]map[string]interface{}, inventory []string) (int, []finding, float64) {
	mapped := 0
	unmapped := []finding{}
	for _, item := range inventory {
		keys := inventoryKeys(item)
		found := false
		for _, k := range keys {
			if _, ok := registry[k]; ok {
				found = true
				break
			}
		}
		if found {
			mapped++
		} else {
			unmapped = append(unmapped, finding{File: item, Message: "unmapped"})
		}
	}
	total := len(inventory)
	coverage := 0.0
	if total > 0 {
		coverage = float64(mapped) / float64(total) * 100
	}
	return mapped, unmapped, coverage
}

func inventoryKeys(path string) []string {
	rel := filepath.ToSlash(path)
	scope := "ui"
	for _, p := range config.Scan.DealerUIPaths {
		if strings.HasPrefix(rel, filepath.ToSlash(p)) {
			scope = "dealer"
			rel = strings.TrimPrefix(rel, filepath.ToSlash(p))
		}
	}
	for _, p := range config.Scan.AdminUIPaths {
		if strings.HasPrefix(rel, filepath.ToSlash(p)) {
			scope = "admin"
			rel = strings.TrimPrefix(rel, filepath.ToSlash(p))
		}
	}
	rel = strings.TrimPrefix(rel, "/")
	noExt := strings.TrimSuffix(rel, filepath.Ext(rel))
	keys := []string{
		fmt.Sprintf("%s:%s", scope, noExt),
		noExt,
	}
	if strings.Contains(rel, "/app/") && strings.Contains(rel, "/page") {
		route := rel[strings.Index(rel, "/app/")+5:]
		route = strings.TrimSuffix(route, "/page")
		keys = append(keys, fmt.Sprintf("%s:/%s", scope, strings.Trim(route, "/")))
	}
	if strings.HasPrefix(rel, "app/") && strings.Contains(rel, "/page") {
		route := strings.TrimPrefix(rel, "app/")
		route = strings.TrimSuffix(route, "/page")
		keys = append(keys, fmt.Sprintf("%s:/%s", scope, strings.Trim(route, "/")))
	}
	if strings.Contains(rel, "/pages/") {
		route := rel[strings.Index(rel, "/pages/")+7:]
		route = strings.TrimSuffix(route, "/index")
		keys = append(keys, fmt.Sprintf("%s:/%s", scope, strings.Trim(route, "/")))
	}
	if strings.HasPrefix(rel, "pages/") {
		route := strings.TrimPrefix(rel, "pages/")
		route = strings.TrimSuffix(route, "/index")
		keys = append(keys, fmt.Sprintf("%s:/%s", scope, strings.Trim(route, "/")))
	}
	return uniqueStrings(keys)
}

func filterCriticalUnmapped(unmapped []finding, patterns []string) []finding {
	out := []finding{}
	for _, u := range unmapped {
		lower := strings.ToLower(u.File)
		for _, p := range patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				out = append(out, u)
				break
			}
		}
	}
	return out
}

func checkDealerLLID(workspace string, modules []moduleRoot) []finding {
	violations := []finding{}
	dealerFiles := findDealerContractFiles(workspace, modules)
	for _, path := range dealerFiles {
		if !fileHasLLID(path, config.Scan.LLIDFieldNames) {
			violations = append(violations, finding{File: path, Message: "missing LLID field"})
		}
	}
	return violations
}

func scanExternalGateway(workspace string) []finding {
	results := []finding{}
	patterns := config.Scan.ExternalGatewayForbiddenPatterns
	paths := config.Scan.ExternalGatewayForbiddenPaths
	_ = filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isSourceFile(path) && filepath.Ext(path) != ".env" && filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" && filepath.Ext(path) != ".json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)
		for _, p := range patterns {
			if strings.Contains(text, p) {
				results = append(results, finding{
					File:    path,
					Line:    1,
					Message: fmt.Sprintf("forbidden pattern: %s", p),
				})
			}
		}
		for _, p := range paths {
			if strings.Contains(text, p) {
				results = append(results, finding{
					File:    path,
					Line:    1,
					Message: fmt.Sprintf("forbidden path: %s", p),
				})
			}
		}
		return nil
	})
	return results
}

func validateViolationPayloads(rep *report) []finding {
	violations := []finding{}
	for _, r := range rep.Rules {
		for _, v := range r.Violations {
			if v.RuleID == "" || v.File == "" || v.Message == "" || v.FixHint == "" || v.Line <= 0 {
				violations = append(violations, finding{
					RuleID:  "4.1.4",
					File:    v.File,
					Line:    v.Line,
					Message: fmt.Sprintf("missing violation fields for rule %s", r.RuleID),
					FixHint: "Ensure violation payloads include ruleId, file, line, message, fixHint.",
				})
			}
		}
	}
	return violations
}

func findDealerContractFiles(workspace string, modules []moduleRoot) []string {
	files := []string{}
	for _, m := range modules {
		root := filepath.Join(workspace, m.Root)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			for _, p := range config.Scan.DealerContractPaths {
				if strings.HasPrefix(rel, filepath.ToSlash(p)) {
					files = append(files, path)
					return nil
				}
			}
			if isOpenAPISpec(path) && hasDealerTag(path, config.Scan.DealerContractTags) {
				files = append(files, path)
			}
			return nil
		})
	}
	return uniqueStrings(files)
}

func isOpenAPISpec(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}

func hasDealerTag(path string, tags []string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := strings.ToLower(string(data))
	for _, t := range tags {
		if strings.Contains(text, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

func fileHasLLID(path string, fields []string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	for _, f := range fields {
		if strings.Contains(text, f) {
			return true
		}
	}
	return false
}

func extractAPIRoutes(workspace string, bffPaths []string) []apiRoute {
	routes := []apiRoute{}
	for _, rel := range bffPaths {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !isSourceFile(path) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				method, routePath, ok := detectRouteLine(line)
				if !ok {
					continue
				}
				apiID := findAPIIDNear(lines, i)
				routes = append(routes, apiRoute{
					File:   path,
					Line:   i + 1,
					Method: method,
					Path:   routePath,
					APIID:  apiID,
				})
			}
			return nil
		})
	}
	return routes
}

func detectRouteLine(line string) (string, string, bool) {
	framework := strings.ToLower(config.Scan.ApiRouteFramework)
	// Next.js route handlers
	if framework == "nextjs" || framework == "generic" || framework == "" {
		reNext := regexp.MustCompile(`export\s+(async\s+)?function\s+(GET|POST|PUT|DELETE|PATCH)\b`)
		if m := reNext.FindStringSubmatch(line); len(m) == 3 {
			return strings.ToUpper(m[2]), "", true
		}
	}
	// Express/Fastify
	if framework == "express" || framework == "generic" || framework == "" {
		reExpress := regexp.MustCompile(`\.(get|post|put|delete|patch)\s*\(\s*["']([^"']+)`)
		if m := reExpress.FindStringSubmatch(line); len(m) == 3 {
			return strings.ToUpper(m[1]), m[2], true
		}
	}
	// Generic route/path blocks
	if framework == "generic" || framework == "" {
		reGeneric := regexp.MustCompile(`\b(path|route)\s*:\s*["']([^"']+)`)
		if m := reGeneric.FindStringSubmatch(line); len(m) == 3 {
			return "", m[2], true
		}
	}
	return "", "", false
}

func findAPIIDNear(lines []string, start int) string {
	end := start + 5
	if end >= len(lines) {
		end = len(lines) - 1
	}
	reIDs := regexp.MustCompile(`API_IDS\.([A-Za-z0-9_-]+)`)
	reApiId := regexp.MustCompile(`apiId\s*:\s*"(API-[^"]+)"`)
	reApiIdConst := regexp.MustCompile(`apiId\s*:\s*API_IDS\.([A-Za-z0-9_-]+)`)
	reAPIID := regexp.MustCompile(`API_ID\s*:\s*"(API-[^"]+)"`)
	reAPIIDConst := regexp.MustCompile(`API_ID\s*:\s*API_IDS\.([A-Za-z0-9_-]+)`)

	for i := start; i <= end; i++ {
		line := lines[i]
		if signatureEnabled("apiId:") {
			if m := reApiId.FindStringSubmatch(line); len(m) == 2 {
				return m[1]
			}
			if m := reApiIdConst.FindStringSubmatch(line); len(m) == 2 {
				return "API_IDS." + m[1]
			}
		}
		if signatureEnabled("API_ID:") {
			if m := reAPIID.FindStringSubmatch(line); len(m) == 2 {
				return m[1]
			}
			if m := reAPIIDConst.FindStringSubmatch(line); len(m) == 2 {
				return "API_IDS." + m[1]
			}
		}
		if signatureEnabled("API_IDS.") {
			if m := reIDs.FindStringSubmatch(line); len(m) == 2 {
				return "API_IDS." + m[1]
			}
		}
	}
	return ""
}

func signatureEnabled(sig string) bool {
	for _, s := range config.Scan.ApiIDSignatures {
		if s == sig {
			return true
		}
	}
	return false
}

func resolveAPIID(raw string, known map[string]struct{}) string {
	if raw == "" {
		return ""
	}
	if _, ok := known[raw]; ok {
		return raw
	}
	if strings.HasPrefix(raw, "API_IDS.") {
		name := strings.TrimPrefix(raw, "API_IDS.")
		if _, ok := known[name]; ok {
			return name
		}
		if _, ok := known["API-"+name]; ok {
			return "API-" + name
		}
		return raw
	}
	if !strings.HasPrefix(raw, "API-") {
		if _, ok := known["API-"+raw]; ok {
			return "API-" + raw
		}
	}
	return raw
}

func apiIDSet(reg *registry) map[string]struct{} {
	set := map[string]struct{}{}
	if reg == nil {
		return set
	}
	if raw, ok := reg.IDs["API"]; ok {
		switch v := raw.(type) {
		case map[string]interface{}:
			for k := range v {
				set[k] = struct{}{}
			}
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					set[s] = struct{}{}
				}
			}
		}
	}
	return set
}

func checkDealerUiLLIDDisplay(workspace string) (int, []finding, float64) {
	paths := dealerUIInventory(workspace)
	missing := []finding{}
	dataPages := 0
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(workspace, path))
		if err != nil {
			continue
		}
		text := string(data)
		if !isDealerDataPage(text) {
			continue
		}
		dataPages++
		if !hasAnySignature(text, config.Scan.UILLIDDisplaySignatures) {
			missing = append(missing, finding{File: path, Line: 1, Message: "missing LLID display"})
		}
	}
	coverage := 100.0
	if dataPages > 0 {
		covered := dataPages - len(missing)
		coverage = float64(covered) / float64(dataPages) * 100
	}
	return dataPages, missing, coverage
}

func dealerUIInventory(workspace string) []string {
	roots := append([]string{}, config.Scan.DealerUIPaths...)
	items := []string{}
	globs := config.Scan.UIInventoryGlobs
	if len(globs) == 0 {
		switch strings.ToLower(config.Scan.UIInventoryFramework) {
		case "react-router":
			globs = []string{"**/routes/**/*.*", "**/components/**/*.*"}
		case "generic":
			globs = []string{"**/*.*"}
		default:
			globs = []string{"**/app/**/page.*", "**/pages/**/*.*", "**/components/**/*.*"}
		}
	}
	for _, rel := range roots {
		root := filepath.Join(workspace, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.Contains(path, "node_modules") || strings.Contains(path, "dist") || strings.Contains(path, "build") {
				return nil
			}
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)
			for _, g := range globs {
				if matchGlob(relPath, g) {
					items = append(items, filepath.ToSlash(filepath.Join(rel, relPath)))
					break
				}
			}
			return nil
		})
	}
	return uniqueStrings(items)
}

func isDealerDataPage(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "api/dealer") {
		return true
	}
	if strings.Contains(lower, "contracts/dealer") {
		return true
	}
	for _, ent := range config.Scan.DealerEntities {
		if strings.Contains(lower, strings.ToLower(ent)) {
			return true
		}
	}
	return false
}

func hasAnySignature(text string, signatures []string) bool {
	for _, sig := range signatures {
		if strings.Contains(text, sig) {
			return true
		}
	}
	return false
}

func listBackupSnapshots(workspace string) []string {
	backupDir := filepath.Join(workspace, ".b2b", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil
	}
	ids := []string{}
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids
}

func loadScanOverrides(workspace string) {
	path := filepath.Join(workspace, ".b2b", "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(support.StripBOM(data), &raw); err != nil {
		return
	}
	setStringSlice := func(key string, target *[]string) {
		if v, ok := raw[key]; ok {
			*target = toStringSlice(v)
		}
	}
	setString := func(key string, target *string) {
		if v, ok := raw[key]; ok {
			if s, ok := v.(string); ok {
				*target = s
			}
		}
	}
	setInt := func(key string, target *int) {
		if v, ok := raw[key]; ok {
			switch n := v.(type) {
			case int:
				*target = n
			case int64:
				*target = int(n)
			}
		}
	}
	setBool := func(key string, target *bool) {
		if v, ok := raw[key]; ok {
			if b, ok := v.(bool); ok {
				*target = b
			}
		}
	}

	setStringSlice("dealer_ui_paths", &config.Scan.DealerUIPaths)
	setStringSlice("admin_ui_paths", &config.Scan.AdminUIPaths)
	setStringSlice("dealer_bff_paths", &config.Scan.DealerBFFPaths)
	setStringSlice("admin_bff_paths", &config.Scan.AdminBFFPaths)
	setStringSlice("wrapper_signatures", &config.Scan.WrapperSignatures)
	setStringSlice("contract_spec_globs", &config.Scan.ContractSpecGlobs)
	setString("contract_version_file", &config.Scan.ContractVersionFile)
	setStringSlice("contract_version_required_fields", &config.Scan.ContractVersionRequiredFields)
	setStringSlice("compatibility_allowed_values", &config.Scan.CompatibilityAllowedValues)
	setString("ui_registry_path", &config.Scan.UIRegistryPath)
	setString("ui_registry_enforcement", &config.Scan.UIRegistryEnforcement)
	setStringSlice("ui_registry_critical_patterns", &config.Scan.UIRegistryCriticalPatterns)
	setString("ui_inventory_framework", &config.Scan.UIInventoryFramework)
	setStringSlice("ui_inventory_globs", &config.Scan.UIInventoryGlobs)
	setStringSlice("llid_field_names", &config.Scan.LLIDFieldNames)
	setStringSlice("dealer_contract_tags", &config.Scan.DealerContractTags)
	setStringSlice("dealer_contract_paths", &config.Scan.DealerContractPaths)
	setStringSlice("audit_wrapper_signatures", &config.Scan.AuditWrapperSignatures)
	setStringSlice("external_gateway_forbidden_patterns", &config.Scan.ExternalGatewayForbiddenPatterns)
	setStringSlice("external_gateway_forbidden_paths", &config.Scan.ExternalGatewayForbiddenPaths)
	setStringSlice("semantic_protected_paths", &config.Scan.SemanticProtectedPaths)

	setStringSlice("api_id_signatures", &config.Scan.ApiIDSignatures)
	setString("api_route_framework", &config.Scan.ApiRouteFramework)
	setString("api_route_enforcement", &config.Scan.ApiRouteEnforcement)
	setString("ui_llid_display_enforcement", &config.Scan.UILLIDDisplayEnforcement)
	setStringSlice("ui_llid_display_signatures", &config.Scan.UILLIDDisplaySignatures)
	setStringSlice("dealer_entities", &config.Scan.DealerEntities)
	setInt("history_max_snapshots", &config.Scan.HistoryMaxSnapshots)
	setInt("history_keep_days", &config.Scan.HistoryKeepDays)
	setBool("shadow_fail_on_mismatch", &config.Scan.ShadowFailOnMismatch)
}

func toStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	case string:
		return []string{t}
	default:
		return nil
	}
}
