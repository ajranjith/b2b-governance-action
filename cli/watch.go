package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
	"github.com/fsnotify/fsnotify"
)

func runWatch(root string) {
	runWatchWithStop(root, nil)
}

func runWatchWithStop(root string, stop <-chan struct{}) {
	if root == "" {
		root = config.Paths.WorkspaceRoot
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: watch init failed: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	if err := addWatchRecursive(watcher, root); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: watch failed: %v\n", err)
		os.Exit(1)
	}

	var timer *time.Timer
	debounce := 300 * time.Millisecond
	trigger := func() {
		runScan()
		if _, err := os.Stat(filepath.Join(root, ".b2b", "results.json")); err == nil {
			runVerify()
		}
		writeHints(root)
		writeReportHTML(root)
		writeHistorySnapshot(root)
		updateWatchReport(root)
	}

	stopCh := stop

	for {
		select {
		case <-stopCh:
			return
		case ev := <-watcher.Events:
			if strings.Contains(ev.Name, string(filepath.Separator)+".b2b"+string(filepath.Separator)) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, trigger)
		case err := <-watcher.Errors:
			fmt.Fprintf(os.Stderr, "ERROR: watch error: %v\n", err)
		}
	}
}

func addWatchRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.Contains(path, string(filepath.Separator)+".b2b") {
				return filepath.SkipDir
			}
			return w.Add(path)
		}
		return nil
	})
}

func writeHints(workspace string) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		return
	}
	hints := []finding{}
	for _, r := range rep.Rules {
		if r.Status == "FAIL" || r.Status == "WARN" {
			hints = append(hints, r.Violations...)
		}
	}
	_ = support.WriteJSONAtomic(filepath.Join(workspace, ".b2b", "hints.json"), hints)
}

func writeReportHTML(workspace string) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	html := "<html><body><h2>Governance HUD</h2><p>HUD will turn RED if the template performs a DB mutation without the mandatory audit wrapper/marker (Naked Mutation).</p><pre>" + string(data) + "</pre></body></html>"
	_ = support.WriteFileAtomic(filepath.Join(workspace, ".b2b", "report.html"), []byte(html))
}

func writeHistorySnapshot(workspace string) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	status := "PASS"
	var rep report
	if err := json.Unmarshal(data, &rep); err == nil {
		for _, r := range rep.Rules {
			if r.Status == "FAIL" {
				status = "FAIL"
				break
			}
		}
	}
	sha := gitShortSHA(workspace)
	ts := time.Now().UTC().Format("20060102_150405")
	name := fmt.Sprintf("%s_%s_%s.report.json", ts, sha, status)
	historyDir := filepath.Join(workspace, ".b2b", "history")
	_ = os.MkdirAll(historyDir, 0o755)
	_ = support.WriteFileAtomic(filepath.Join(historyDir, name), data)
	rotateHistory(historyDir)
}

func rotateHistory(historyDir string) {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return
	}
	type item struct {
		name string
		time time.Time
	}
	items := []item{}
	for _, e := range entries {
		name := e.Name()
		if len(name) < 15 {
			continue
		}
		ts := name[:15]
		t, err := time.Parse("20060102_150405", ts)
		if err != nil {
			continue
		}
		items = append(items, item{name: name, time: t})
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -config.Scan.HistoryKeepDays)
	for _, it := range items {
		if it.time.Before(cutoff) {
			_ = os.Remove(filepath.Join(historyDir, it.name))
		}
	}
	entries, _ = os.ReadDir(historyDir)
	if len(entries) <= config.Scan.HistoryMaxSnapshots {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].time.Before(items[j].time) })
	excess := len(entries) - config.Scan.HistoryMaxSnapshots
	for i := 0; i < excess && i < len(items); i++ {
		_ = os.Remove(filepath.Join(historyDir, items[i].name))
	}
}

func gitShortSHA(workspace string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		return "nogit"
	}
	return strings.TrimSpace(string(out))
}

func updateWatchReport(workspace string) {
	reportPath := filepath.Join(workspace, ".b2b", "report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		return
	}

	hintsPath := filepath.Join(workspace, ".b2b", "hints.json")
	htmlPath := filepath.Join(workspace, ".b2b", "report.html")
	historyDir := filepath.Join(workspace, ".b2b", "history")

	rep.Rules = upsertRule(rep.Rules, makeRule("4.2.1", "PASS", "medium", map[string]interface{}{
		"watchEnabled": true,
	}, nil, ""))

	statusHints := "FAIL"
	if fileExists(hintsPath) {
		statusHints = "PASS"
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.2.2", statusHints, "medium", map[string]interface{}{
		"hintsPath": hintsPath,
	}, nil, "Ensure watch writes hints.json on each scan."))

	statusHTML := "FAIL"
	if fileExists(htmlPath) {
		statusHTML = "PASS"
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.2.3", statusHTML, "medium", map[string]interface{}{
		"reportHtmlPath": htmlPath,
	}, nil, "Ensure watch regenerates report.html on each scan."))

	snapshotCount := countFiles(historyDir)
	statusHistory := "FAIL"
	if snapshotCount > 0 {
		statusHistory = "PASS"
	}
	rep.Rules = upsertRule(rep.Rules, makeRule("4.2.4", statusHistory, "medium", map[string]interface{}{
		"historyDir":           historyDir,
		"historySnapshotCount": snapshotCount,
	}, nil, "Ensure history snapshots are created and rotated."))

	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(reportPath, rep)
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}
