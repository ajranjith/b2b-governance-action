package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

func runSupportBundle(repoRoot string) {
	if repoRoot == "" {
		repoRoot = config.Paths.WorkspaceRoot
	}
	name := fmt.Sprintf("support-bundle_%s.zip", time.Now().UTC().Format("20060102_150405"))
	outPath := filepath.Join(repoRoot, ".b2b", name)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", outPath, os.Getpid())
	f, err := os.Create(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	zipw := zip.NewWriter(f)

	candidates := []string{
		".b2b/report.json",
		".b2b/report.html",
		".b2b/certificate.json",
		".b2b/audit.log",
		".b2b/doctor.json",
		".b2b/hints.json",
		".b2b/parity-report.json",
		".b2b/fix-plan.json",
		".b2b/fix.patch",
		".b2b/fix-apply.json",
		".b2b/config.yml",
	}

	for _, rel := range candidates {
		path := filepath.Join(repoRoot, rel)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := addFileToZip(zipw, path, rel); err != nil {
			_ = zipw.Close()
			_ = f.Close()
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	}

	if err := zipw.Close(); err != nil {
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	_ = os.Remove(outPath)
	if err := os.Rename(tmpPath, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	updateSupportBundleReport(repoRoot, outPath)
}

func addFileToZip(zipw *zip.Writer, path, name string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	w, err := zipw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func updateSupportBundleReport(workspace, bundlePath string) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		return
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.6.3", "PASS", "low", map[string]interface{}{
		"bundlePath": bundlePath,
	}, nil, ""))
	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(reportPath, rep)
}
