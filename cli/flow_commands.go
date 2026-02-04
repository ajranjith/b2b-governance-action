package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ajranjith/b2b-governance-action/cli/internal/flow"
	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type flowRunner struct{}

func (flowRunner) Run(action flow.Action, targetRoot string) error {
	if targetRoot == "" {
		targetRoot = config.Paths.WorkspaceRoot
	}
	prev := config.Paths.WorkspaceRoot
	config.Paths.WorkspaceRoot = targetRoot
	defer func() { config.Paths.WorkspaceRoot = prev }()

	switch action.Name {
	case "scan":
		runScan()
	case "verify":
		runVerify()
	case "watch":
		runWatch(action.WatchPath)
	case "shadow":
		runShadow(action.VectorsPath, targetRoot)
	case "fix":
		runFix(action.FixDryRun)
	case "support-bundle":
		runSupportBundle(targetRoot)
	case "rollback":
		if action.RollbackLatest {
			runRollbackLatest()
		} else if action.RollbackTo != "" {
			runRollbackTo(action.RollbackTo)
		} else {
			return fmt.Errorf("rollback requires --latest-green or --to <timestamp>")
		}
	case "doctor":
		runDoctor()
	default:
		return fmt.Errorf("unknown action: %s", action.Name)
	}
	return nil
}

func runSetup(args []string) {
	opts, interactive, err := parseSetupArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if interactive {
		if err := fillSetupInteractive(&opts); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	}

	state, err := flow.Run(flowContext(), opts, flowRunner{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	updateSetupReport(config.Paths.WorkspaceRoot, state)
}

func runTargetCommand(args []string) {
	opts := flow.Options{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--target":
			if i+1 < len(args) {
				opts.TargetPath = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				opts.RepoURL = args[i+1]
				i++
			}
		case "--ref":
			if i+1 < len(args) {
				opts.Ref = args[i+1]
				i++
			}
		case "--subdir":
			if i+1 < len(args) {
				opts.Subdir = args[i+1]
				i++
			}
		}
	}
	if opts.TargetPath == "" && opts.RepoURL == "" {
		fmt.Fprintln(os.Stderr, "ERROR: target requires --target or --repo")
		os.Exit(1)
	}
	if _, err := flow.RunStep(flowContext(), flow.StepSelectTarget, opts, flowRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runClassifyCommand(args []string) {
	opts := flow.Options{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mode":
			if i+1 < len(args) {
				opts.Mode = args[i+1]
				i++
			}
		case "--target":
			if i+1 < len(args) {
				opts.TargetPath = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				opts.RepoURL = args[i+1]
				i++
			}
		case "--ref":
			if i+1 < len(args) {
				opts.Ref = args[i+1]
				i++
			}
		case "--subdir":
			if i+1 < len(args) {
				opts.Subdir = args[i+1]
				i++
			}
		}
	}
	if opts.Mode == "" {
		fmt.Fprintln(os.Stderr, "ERROR: classify requires --mode greenfield|brownfield")
		os.Exit(1)
	}
	if opts.TargetPath != "" || opts.RepoURL != "" {
		if _, err := flow.RunStep(flowContext(), flow.StepSelectTarget, opts, flowRunner{}); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	}
	if _, err := flow.RunStep(flowContext(), flow.StepClassify, opts, flowRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runDetectAgentsCommand() {
	if _, err := flow.RunStep(flowContext(), flow.StepDetectAgents, flow.Options{}, flowRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runConnectAgentsCommand(args []string) {
	opts := parseAgentArgs(args)
	if _, err := flow.RunStep(flowContext(), flow.StepConnectAgents, opts, flowRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runValidateAgentsCommand(args []string) {
	opts := parseAgentArgs(args)
	if _, err := flow.RunStep(flowContext(), flow.StepValidateAgents, opts, flowRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func runActionCommand(actionName string, args []string) {
	action := flow.Action{Name: actionName}
	opts := flow.Options{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--vectors":
			if i+1 < len(args) {
				action.VectorsPath = args[i+1]
				i++
			}
		case "--dry-run":
			action.FixDryRun = true
		case "--apply":
			action.FixApply = true
		case "--watch":
			if i+1 < len(args) {
				action.WatchPath = args[i+1]
				i++
			}
		case "--latest-green":
			action.RollbackLatest = true
		case "--to":
			if i+1 < len(args) {
				action.RollbackTo = args[i+1]
				i++
			}
		case "--target":
			if i+1 < len(args) {
				opts.TargetPath = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				opts.RepoURL = args[i+1]
				i++
			}
		case "--ref":
			if i+1 < len(args) {
				opts.Ref = args[i+1]
				i++
			}
		case "--subdir":
			if i+1 < len(args) {
				opts.Subdir = args[i+1]
				i++
			}
		case "--mode":
			if i+1 < len(args) {
				opts.Mode = args[i+1]
				i++
			}
		case "--max-fix-attempts":
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					opts.MaxFixAttempts = v
				}
				i++
			}
		}
	}

	opts.Action = action
	opts.ForceAction = true

	if requiresTarget(actionName) {
		if err := ensureTargetForAction(&opts); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			fmt.Fprintln(os.Stderr, "Hint: run `gres-b2b setup` or provide --target/--repo.")
			os.Exit(1)
		}
		if _, err := flow.RunStep(flowContext(), flow.StepSelectTarget, opts, flowRunner{}); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	}

	if actionName == "fix-loop" {
		if _, err := flow.RunStep(flowContext(), flow.StepFixLoop, opts, flowRunner{}); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if _, err := flow.RunStep(flowContext(), flow.StepScan, opts, flowRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func parseSetupArgs(args []string) (flow.Options, bool, error) {
	opts := flow.Options{}
	interactive := true

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--non-interactive":
			interactive = false
		case "--target":
			if i+1 < len(args) {
				opts.TargetPath = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				opts.RepoURL = args[i+1]
				i++
			}
		case "--ref":
			if i+1 < len(args) {
				opts.Ref = args[i+1]
				i++
			}
		case "--subdir":
			if i+1 < len(args) {
				opts.Subdir = args[i+1]
				i++
			}
		case "--client":
			if i+1 < len(args) {
				opts.Clients = append(opts.Clients, args[i+1])
				i++
			}
		case "--all":
			opts.AllClients = true
		case "--config":
			if i+1 < len(args) {
				opts.AgentConfigPath = args[i+1]
				i++
			}
		case "--bin":
			if i+1 < len(args) {
				opts.AgentBinaryPath = args[i+1]
				i++
			}
		case "--mode":
			if i+1 < len(args) {
				opts.Mode = args[i+1]
				i++
			}
		case "--max-fix-attempts":
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					opts.MaxFixAttempts = v
				}
				i++
			}
		case "--action":
			if i+1 < len(args) {
				opts.Action.Name = args[i+1]
				i++
			}
		case "--vectors":
			if i+1 < len(args) {
				opts.Action.VectorsPath = args[i+1]
				i++
			}
		case "--dry-run":
			opts.Action.FixDryRun = true
		case "--apply":
			opts.Action.FixApply = true
		}
	}

	if !interactive && opts.Action.Name == "" {
		return opts, interactive, fmt.Errorf("--action is required for --non-interactive setup")
	}
	return opts, interactive, nil
}

func fillSetupInteractive(opts *flow.Options) error {
	reader := bufio.NewReader(os.Stdin)
	if !opts.AllClients && len(opts.Clients) == 0 {
		fmt.Print("Agent client (id or name, or 'all'): ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.EqualFold(line, "all") {
			opts.AllClients = true
		} else if line != "" {
			opts.Clients = []string{line}
		}
	}

	if opts.Mode == "" {
		fmt.Print("Mode (greenfield|brownfield): ")
		line, _ := reader.ReadString('\n')
		opts.Mode = strings.TrimSpace(line)
	}

	if opts.TargetPath == "" && opts.RepoURL == "" {
		if err := promptTarget(reader, opts); err != nil {
			return err
		}
	}

	if opts.Action.Name == "" {
		fmt.Print("Action (scan|verify|watch|shadow|fix|fix-loop|support-bundle|rollback|doctor): ")
		line, _ := reader.ReadString('\n')
		opts.Action.Name = strings.TrimSpace(line)
	}

	if opts.Action.Name == "shadow" && opts.Action.VectorsPath == "" {
		fmt.Print("Vectors file path: ")
		line, _ := reader.ReadString('\n')
		opts.Action.VectorsPath = strings.TrimSpace(line)
	}

	if opts.Action.Name == "watch" && opts.Action.WatchPath == "" {
		fmt.Print("Watch path (default target): ")
		line, _ := reader.ReadString('\n')
		opts.Action.WatchPath = strings.TrimSpace(line)
	}

	if opts.Action.Name == "fix" && !opts.Action.FixDryRun && !opts.Action.FixApply {
		fmt.Print("Fix mode (dry-run|apply): ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "apply" {
			opts.Action.FixApply = true
		} else {
			opts.Action.FixDryRun = true
		}
	}

	if opts.Action.Name == "rollback" && !opts.Action.RollbackLatest && opts.Action.RollbackTo == "" {
		fmt.Print("Rollback (latest-green|timestamp): ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "latest-green" || line == "latest" {
			opts.Action.RollbackLatest = true
		} else {
			opts.Action.RollbackTo = line
		}
	}

	return nil
}

func requiresTarget(action string) bool {
	switch action {
	case "scan", "verify", "watch", "shadow", "fix", "fix-loop":
		return true
	default:
		return false
	}
}

func ensureTargetForAction(opts *flow.Options) error {
	if opts.TargetPath != "" || opts.RepoURL != "" {
		return nil
	}

	root := flowContext().Root
	if root == "" {
		root = config.Paths.WorkspaceRoot
	}
	state, _ := flow.LoadState(root)
	if state.Target.WorkspaceRoot != "" || state.Target.Path != "" {
		opts.TargetPath = state.Target.WorkspaceRoot
		if opts.TargetPath == "" {
			opts.TargetPath = state.Target.Path
		}
		return nil
	}

	if !isInteractive() {
		return fmt.Errorf("target is required")
	}

	reader := bufio.NewReader(os.Stdin)
	return promptTarget(reader, opts)
}

func promptTarget(reader *bufio.Reader, opts *flow.Options) error {
	fmt.Print("Target (local path or GitHub URL): ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return fmt.Errorf("target is required")
	}
	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") || strings.Contains(line, "://") {
		opts.RepoURL = line
		fmt.Print("Ref (default main): ")
		ref, _ := reader.ReadString('\n')
		opts.Ref = strings.TrimSpace(ref)
		if opts.Ref == "" {
			opts.Ref = "main"
		}
		fmt.Print("Subdir (optional): ")
		subdir, _ := reader.ReadString('\n')
		opts.Subdir = strings.TrimSpace(subdir)
	} else {
		opts.TargetPath = line
	}
	return nil
}

func isInteractive() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func parseAgentArgs(args []string) flow.Options {
	opts := flow.Options{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--client":
			if i+1 < len(args) {
				opts.Clients = append(opts.Clients, args[i+1])
				i++
			}
		case "--all":
			opts.AllClients = true
		case "--config":
			if i+1 < len(args) {
				opts.AgentConfigPath = args[i+1]
				i++
			}
		case "--bin":
			if i+1 < len(args) {
				opts.AgentBinaryPath = args[i+1]
				i++
			}
		}
	}
	return opts
}

func flowContext() flow.Context {
	exe := ""
	if p, err := os.Executable(); err == nil {
		exe = p
	}
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = os.Getenv("HOME")
	}
	return flow.Context{Root: config.Paths.WorkspaceRoot, HomeDir: home, ExePath: exe, SkipSelftest: os.Getenv("GRES_SKIP_SELFTEST") == "1"}
}

func updateSetupReport(workspace string, state flow.State) {
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
		"setupStatus":    state.Status,
		"currentStep":    state.CurrentStep,
		"stepsCompleted": state.StepsCompleted,
	}, nil, ""))

	rep.Phase4Status = phase4Status(rep.Rules)
	_ = support.WriteJSONAtomic(reportPath, rep)
}
