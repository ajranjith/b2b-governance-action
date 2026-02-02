package flow

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type StepID string

const (
	StepSelectTarget   StepID = "S1"
	StepDetectAgents   StepID = "S2"
	StepConnectAgents  StepID = "S3"
	StepValidateAgents StepID = "S4"
	StepClassify       StepID = "S5"
	StepScan           StepID = "S6"
	StepFixLoop        StepID = "S7"
	StepFinalVerify    StepID = "S8"
)

var stepOrder = []StepID{
	StepSelectTarget,
	StepDetectAgents,
	StepConnectAgents,
	StepValidateAgents,
	StepClassify,
	StepScan,
	StepFixLoop,
	StepFinalVerify,
}

type Target struct {
	Type          string `json:"type"` // local|git
	Path          string `json:"path,omitempty"`
	RepoURL       string `json:"repoUrl,omitempty"`
	Ref           string `json:"ref,omitempty"`
	Subdir        string `json:"subdir,omitempty"`
	WorkspaceRoot string `json:"workspaceRoot,omitempty"`
}

type Action struct {
	Name           string `json:"name,omitempty"`
	VectorsPath    string `json:"vectorsPath,omitempty"`
	FixDryRun      bool   `json:"fixDryRun,omitempty"`
	FixApply       bool   `json:"fixApply,omitempty"`
	WatchPath      string `json:"watchPath,omitempty"`
	RollbackLatest bool   `json:"rollbackLatest,omitempty"`
	RollbackTo     string `json:"rollbackTo,omitempty"`
}

type State struct {
	Version         string   `json:"version"`
	Status          string   `json:"status"`
	CurrentStep     StepID   `json:"currentStep"`
	StepsCompleted  []StepID `json:"stepsCompleted"`
	Target          Target   `json:"target"`
	Mode            string   `json:"mode,omitempty"`
	SelectedAgents  []string `json:"selectedAgents,omitempty"`
	DetectedAgents  []Agent  `json:"detectedAgents,omitempty"`
	Action          Action   `json:"action,omitempty"`
	UpdatedAtUtc    string   `json:"updatedAtUtc"`
	LastError       string   `json:"lastError,omitempty"`
	LastErrorStep   StepID   `json:"lastErrorStep,omitempty"`
	ResumeAvailable bool     `json:"resumeAvailable"`
}

type Options struct {
	Root       string
	TargetPath string
	RepoURL    string
	Ref        string
	Subdir     string

	Clients         []string
	AllClients      bool
	AgentConfigPath string
	AgentBinaryPath string
	Mode            string
	MaxFixAttempts  int
	Action          Action
	ForceAction     bool
}

type Context struct {
	Root         string
	HomeDir      string
	ExePath      string
	Now          func() time.Time
	SkipSelftest bool
}

type Runner interface {
	Run(action Action, targetRoot string) error
}

func Run(ctx Context, opts Options, runner Runner) (State, error) {
	state, _ := LoadState(rootFor(ctx, opts))
	start := 0
	if state.CurrentStep != "" {
		if idx := stepIndex(state.CurrentStep); idx >= 0 {
			start = idx + 1
		}
	}

	for i := start; i < len(stepOrder); i++ {
		_, err := RunStep(ctx, stepOrder[i], opts, runner)
		if err != nil {
			return LoadState(rootFor(ctx, opts))
		}
	}
	state, err := LoadState(rootFor(ctx, opts))
	if err == nil {
		state.Status = "COMPLETE"
		state.ResumeAvailable = false
		state.UpdatedAtUtc = time.Now().UTC().Format(time.RFC3339)
		_ = SaveState(rootFor(ctx, opts), state)
	}
	return LoadState(rootFor(ctx, opts))
}

func RunStep(ctx Context, step StepID, opts Options, runner Runner) (State, error) {
	root := rootFor(ctx, opts)
	state, _ := LoadState(root)
	if state.Version == "" {
		state.Version = "1.0"
	}

	now := ctx.Now
	if now == nil {
		now = time.Now
	}

	state.Status = "IN_PROGRESS"
	state.ResumeAvailable = true
	state.LastError = ""
	state.LastErrorStep = ""

	var err error

	switch step {
	case StepSelectTarget:
		err = selectTarget(ctx, opts, &state)
	case StepDetectAgents:
		err = detectAgents(ctx, &state, root)
	case StepConnectAgents:
		err = connectAgents(ctx, opts, &state, root)
	case StepValidateAgents:
		err = validateAgents(ctx, opts, &state, root)
	case StepClassify:
		err = classifyProject(ctx, opts, &state, root)
	case StepScan:
		err = runScanStep(ctx, opts, &state, runner)
	case StepFixLoop:
		err = runFixLoop(ctx, opts, &state, runner)
	case StepFinalVerify:
		err = runFinalVerify(ctx, opts, &state, runner)
	default:
		err = fmt.Errorf("unknown step: %s", step)
	}

	if err != nil {
		state.Status = "FAILED"
		state.LastError = err.Error()
		state.LastErrorStep = step
	} else {
		state.CurrentStep = step
		state.StepsCompleted = appendUnique(state.StepsCompleted, step)
	}

	state.UpdatedAtUtc = now().UTC().Format(time.RFC3339)
	if saveErr := SaveState(root, state); saveErr != nil && err == nil {
		err = saveErr
	}
	return state, err
}

func LoadState(root string) (State, error) {
	path := setupPath(root)
	var state State
	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return State{}, nil
		}
		return State{}, err
	}
	if err := decodeJSON(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func SaveState(root string, state State) error {
	path := setupPath(root)
	return support.WriteJSONAtomic(path, state)
}

func setupPath(root string) string {
	return filepath.Join(root, ".b2b", "setup.json")
}

func rootFor(ctx Context, opts Options) string {
	if opts.Root != "" {
		return opts.Root
	}
	if ctx.Root != "" {
		return ctx.Root
	}
	return "."
}

func stepIndex(step StepID) int {
	for i, s := range stepOrder {
		if s == step {
			return i
		}
	}
	return -1
}

func appendUnique(list []StepID, step StepID) []StepID {
	for _, s := range list {
		if s == step {
			return list
		}
	}
	return append(list, step)
}

func updateEvidence(state *State) error {
	// Evidence updates are performed by scan/verify/watch/etc.
	// This step exists to keep wizard/CLI flow parity.
	return nil
}
