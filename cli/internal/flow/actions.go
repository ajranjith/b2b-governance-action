package flow

import "fmt"

func runAction(ctx Context, opts Options, state *State, runner Runner) error {
	action := opts.Action
	if action.Name == "" && state.Action.Name != "" {
		action = state.Action
	}
	if action.Name == "" && !opts.ForceAction {
		return fmt.Errorf("action is required")
	}
	state.Action = action

	if runner == nil || action.Name == "" {
		return nil
	}

	targetRoot := state.Target.WorkspaceRoot
	if targetRoot == "" {
		targetRoot = state.Target.Path
	}
	if targetRoot == "" {
		targetRoot = rootFor(ctx, opts)
	}

	return runner.Run(action, targetRoot)
}
