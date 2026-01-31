package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type doctorReport struct {
	GeneratedAtUtc string           `json:"generatedAtUtc"`
	RepoRoot       string           `json:"repoRoot"`
	Registry       doctorRegistry   `json:"registry"`
	BFF            doctorBFF        `json:"bff"`
	Contracts      doctorContracts  `json:"contracts"`
	UIRegistry     doctorUIRegistry `json:"uiRegistry"`
	Modes          doctorModes      `json:"modes"`
	Status         string           `json:"status"`
	Reasons        []string         `json:"reasons,omitempty"`
}

type doctorRegistry struct {
	Found              bool            `json:"found"`
	Path               string          `json:"path,omitempty"`
	Valid              bool            `json:"valid"`
	RequiredNamespaces map[string]bool `json:"requiredNamespaces"`
}

type doctorBFF struct {
	DealerBffFound                 bool `json:"dealerBffFound"`
	AdminBffFound                  bool `json:"adminBffFound"`
	WrapperSignaturesConfigured    bool `json:"wrapperSignaturesConfigured"`
	PermissionSignaturesConfigured bool `json:"permissionSignaturesConfigured"`
}

type doctorContracts struct {
	ModulesWithVersionFile    []string `json:"modulesWithVersionFile"`
	ModulesMissingVersionFile []string `json:"modulesMissingVersionFile"`
}

type doctorUIRegistry struct {
	Exists      bool    `json:"exists"`
	Valid       bool    `json:"valid"`
	CoveragePct float64 `json:"coveragePct,omitempty"`
}

type doctorModes struct {
	VerifyAvailable bool `json:"verifyAvailable"`
	WatchAvailable  bool `json:"watchAvailable"`
	ShadowAvailable bool `json:"shadowAvailable"`
	FixAvailable    bool `json:"fixAvailable"`
}

func buildDoctorReport() doctorReport {
	workspace := config.Paths.WorkspaceRoot
	reg, regPath, regValid := loadRegistry(workspace)
	ns := map[string]bool{"API": false, "SVC": false, "DB": false}
	if regValid {
		ns["API"] = hasNamespace(reg.IDs, "API")
		ns["SVC"] = hasNamespace(reg.IDs, "SVC")
		ns["DB"] = hasNamespace(reg.IDs, "DB")
	}

	dealerBff := len(listHandlerFiles(workspace, config.Scan.DealerBFFPaths)) > 0
	adminBff := len(listHandlerFiles(workspace, config.Scan.AdminBFFPaths)) > 0

	modules := resolveModules(workspace)
	withVersion := []string{}
	missingVersion := []string{}
	for _, m := range modules {
		path := filepath.Join(workspace, m.Root, config.Scan.ContractVersionFile)
		if _, err := os.Stat(path); err == nil {
			withVersion = append(withVersion, m.Name)
		} else {
			missingVersion = append(missingVersion, m.Name)
		}
	}

	uiReg, uiViolations := loadUIRegistry(workspace)
	coverage := 0.0
	if uiReg != nil {
		_, _, coverage = mapRegistryCoverage(uiReg, buildUIInventory(workspace))
	}

	status := "OK"
	reasons := []string{}
	if !regValid || !ns["API"] || !ns["SVC"] || !ns["DB"] {
		status = "DEGRADED"
		reasons = append(reasons, "registry invalid or missing namespaces")
	}
	if !dealerBff || !adminBff {
		status = "DEGRADED"
		reasons = append(reasons, "missing dealer/admin BFF")
	}
	if len(missingVersion) > 0 {
		status = "DEGRADED"
		reasons = append(reasons, "missing contract version files")
	}
	if len(uiViolations) > 0 {
		status = "DEGRADED"
		reasons = append(reasons, "ui registry invalid")
	}

	return doctorReport{
		GeneratedAtUtc: time.Now().UTC().Format(time.RFC3339),
		RepoRoot:       workspace,
		Registry: doctorRegistry{
			Found:              regPath != "",
			Path:               regPath,
			Valid:              regValid,
			RequiredNamespaces: ns,
		},
		BFF: doctorBFF{
			DealerBffFound:                 dealerBff,
			AdminBffFound:                  adminBff,
			WrapperSignaturesConfigured:    len(config.Scan.WrapperSignatures) > 0,
			PermissionSignaturesConfigured: len(config.Scan.PermissionSignatures) > 0,
		},
		Contracts: doctorContracts{
			ModulesWithVersionFile:    withVersion,
			ModulesMissingVersionFile: missingVersion,
		},
		UIRegistry: doctorUIRegistry{
			Exists:      uiReg != nil,
			Valid:       len(uiViolations) == 0,
			CoveragePct: coverage,
		},
		Modes: doctorModes{
			VerifyAvailable: true,
			WatchAvailable:  true,
			ShadowAvailable: true,
			FixAvailable:    true,
		},
		Status:  status,
		Reasons: reasons,
	}
}

func updateDoctorReport(workspace string, docReport doctorReport) {
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
	if docReport.Status != "OK" {
		status = "FAIL"
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.6.2", status, "high", map[string]interface{}{
		"doctorStatus": docReport.Status,
		"reasons":      docReport.Reasons,
	}, nil, "Ensure doctor readiness checks pass."))

	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(reportPath, rep)
}
