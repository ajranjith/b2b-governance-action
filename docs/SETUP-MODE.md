# gres-b2b --setup Mode

This document describes the current `--setup` behavior in the CLI.

## Command

```bash
gres-b2b --setup
```

## Behavior

- Reads `.b2b/setup.json` if present
- Advances the setup step counter
- Writes updated setup state back to `.b2b/setup.json` (atomic)
- Updates `.b2b/report.json` with rule `4.6.4` evidence

## Output

`.b2b/setup.json` example:
```json
{
  "currentStep": 3,
  "steps": ["registry", "bff", "contracts", "uiRegistry", "verify"],
  "status": "IN_PROGRESS",
  "updatedAtUtc": "2026-01-31T01:00:00Z"
}
```

## Notes

- This command does not start a web server.
- It is safe to re-run; it resumes from the existing setup state.
