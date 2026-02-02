# gres-b2b setup Mode

This document describes the shared Setup + Connect + Run flow used by both the Wizard and the CLI.

## Commands

Interactive:
```bash
gres-b2b setup
```

Non-interactive:
```bash
gres-b2b setup --non-interactive --target <path> --client cursor --action scan
```

## Behavior (S1–S7)

- **S1 Select Target** (local path or Git repo URL)
- **S2 Detect AI Agents** (.b2b/agent-detect.json)
- **S3 Connect AI Agent** (backup + MCP config write)
- **S4 Validate Connection** (.b2b/agent-validate.json)
- **S5 Classify Project** (greenfield/brownfield)
- **S6 Scan** (scan/verify/watch/shadow/fix/support-bundle/rollback/doctor)
- **S7 Fix Loop** (scan -> fix -> rescan until green or max attempts)
- **S8 Final Verification + Shortcut** (verify + HUD shortcut)

## Output

`.b2b/setup.json` example:
```json
{
  "version": "1.0",
  "status": "IN_PROGRESS",
  "currentStep": "S4",
  "stepsCompleted": ["S1", "S2", "S3", "S4"],
  "target": {
    "type": "local",
    "path": "C:\\repo",
    "workspaceRoot": "C:\\repo"
  },
  "selectedAgents": ["cursor"],
  "action": {
    "name": "scan"
  },
  "updatedAtUtc": "2026-01-31T01:00:00Z",
  "resumeAvailable": true
}
```

## Notes

- Setup is resumable; re-running continues from the next step.
- Wizard and CLI execute the same steps in the same order.
- All writes use atomic helpers.
