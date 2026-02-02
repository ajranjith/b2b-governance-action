package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type WorkspaceEntry struct {
	RepoURL     string `json:"repoUrl"`
	Ref         string `json:"ref,omitempty"`
	Subdir      string `json:"subdir,omitempty"`
	Path        string `json:"path"`
	AddedAtUtc  string `json:"addedAtUtc"`
	WorkspaceID string `json:"workspaceId"`
}

func selectTarget(ctx Context, opts Options, state *State) error {
	if opts.TargetPath == "" && opts.RepoURL == "" {
		if state.Target.WorkspaceRoot != "" {
			return nil
		}
		return fmt.Errorf("target is required")
	}

	if opts.TargetPath != "" {
		abs, err := filepath.Abs(opts.TargetPath)
		if err != nil {
			return err
		}
		if _, err := os.Stat(abs); err != nil {
			return err
		}
		state.Target = Target{
			Type:          "local",
			Path:          abs,
			WorkspaceRoot: abs,
		}
		return nil
	}

	repoURL := opts.RepoURL
	root := rootFor(ctx, opts)
	wsRoot := filepath.Join(root, ".b2b", "workspaces")
	if err := os.MkdirAll(wsRoot, 0o755); err != nil {
		return err
	}

	workspaceID := sanitizeWorkspaceID(repoURL, opts.Ref, opts.Subdir)
	clonePath := filepath.Join(wsRoot, workspaceID)

	if _, err := os.Stat(clonePath); err != nil {
		if err := cloneRepo(repoURL, opts.Ref, clonePath); err != nil {
			return err
		}
	}

	workspaceRoot := clonePath
	if opts.Subdir != "" {
		workspaceRoot = filepath.Join(clonePath, opts.Subdir)
		if _, err := os.Stat(workspaceRoot); err != nil {
			return err
		}
	}

	state.Target = Target{
		Type:          "git",
		RepoURL:       repoURL,
		Ref:           opts.Ref,
		Subdir:        opts.Subdir,
		WorkspaceRoot: workspaceRoot,
	}

	entry := WorkspaceEntry{
		RepoURL:     repoURL,
		Ref:         opts.Ref,
		Subdir:      opts.Subdir,
		Path:        workspaceRoot,
		AddedAtUtc:  time.Now().UTC().Format(time.RFC3339),
		WorkspaceID: workspaceID,
	}

	return writeWorkspaces(root, entry)
}

func writeWorkspaces(root string, entry WorkspaceEntry) error {
	path := filepath.Join(root, ".b2b", "workspaces.json")
	existing := []WorkspaceEntry{}
	if data, err := readFile(path); err == nil {
		_ = decodeJSON(data, &existing)
	}

	replaced := false
	for i := range existing {
		if existing[i].WorkspaceID == entry.WorkspaceID {
			existing[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		existing = append(existing, entry)
	}

	return support.WriteJSONAtomic(path, existing)
}

func sanitizeWorkspaceID(repoURL, ref, subdir string) string {
	base := repoURL
	if ref != "" {
		base += "@" + ref
	}
	if subdir != "" {
		base += ":" + subdir
	}
	re := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	return re.ReplaceAllString(base, "_")
}

func cloneRepo(repoURL, ref, dst string) error {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repoURL, dst)
	if !runCommand("git", args, 2*time.Minute) {
		return fmt.Errorf("git clone failed for %s", repoURL)
	}
	return nil
}
