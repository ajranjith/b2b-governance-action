# gres-b2b --setup Mode Specification

This document specifies the requirements for the `--setup` mode in the `gres-b2b` Go binary, which enables live progress streaming through a browser-based UI.

## Overview

The `--setup` mode provides an interactive wizard experience that:
1. Runs preflight checks
2. Opens a browser with real-time progress
3. Streams scan output via Server-Sent Events (SSE)
4. Shows RED/AMBER/GREEN results when complete

## Command

```bash
gres-b2b --setup [--port=8765] [--workspace=/path/to/project]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8765` | SSE server port |
| `--workspace` | `.` | Path to workspace to scan |
| `--no-browser` | `false` | Don't auto-open browser |

## Architecture

```
┌─────────────────┐      ┌─────────────────┐
│   Browser       │◄────►│  gres-b2b       │
│  progress.html  │ SSE  │  --setup mode   │
└─────────────────┘      └─────────────────┘
        ▲                        │
        │                        ▼
        │                ┌─────────────────┐
        └────────────────│  Scan Engine    │
           Open report   └─────────────────┘
```

## SSE Event Types

The Go binary must emit these SSE events:

### 1. Preflight Check Events

```json
event: preflight
data: {"check": "binary", "status": "pass", "detail": "gres-b2b v4.0.0"}

event: preflight
data: {"check": "git", "status": "pass", "detail": "git version 2.40.0"}

event: preflight
data: {"check": "config", "status": "pass", "detail": "config.toml valid"}

event: preflight
data: {"check": "workspace", "status": "pass", "detail": "/path/to/workspace"}
```

Status values: `pending`, `running`, `pass`, `fail`

### 2. Phase Events

```json
event: phase
data: {"phase": "validate", "status": "active", "message": "Validating configuration..."}

event: phase
data: {"phase": "validate", "status": "complete", "message": "Configuration valid"}

event: phase
data: {"phase": "scan", "status": "active", "message": "Scanning 47 files..."}

event: phase
data: {"phase": "scan", "status": "complete", "message": "47 files scanned"}

event: phase
data: {"phase": "analyze", "status": "active", "message": "Analyzing findings..."}

event: phase
data: {"phase": "report", "status": "active", "message": "Writing reports..."}
```

Phase names: `validate`, `scan`, `analyze`, `report`
Status values: `pending`, `active`, `complete`, `error`

### 3. Output Events

```json
event: output
data: {"message": "Checking API boundaries...", "level": "info"}

event: output
data: {"message": "Found 12 checks to report", "level": "success"}

event: output
data: {"message": "Warning: Missing audit hook", "level": "warning"}

event: output
data: {"message": "Error: Invalid pattern", "level": "error"}
```

Level values: `info`, `success`, `warning`, `error`

### 4. Result Event

```json
event: result
data: {"status": "AMBER", "red": 2, "amber": 3, "green": 7, "reportPath": ".b2b/report.html"}
```

Status values: `GREEN`, `AMBER`, `RED`

### 5. Error Event

```json
event: error
data: {"message": "Failed to read workspace directory", "code": "E_WORKSPACE"}
```

## Implementation Requirements

### Go Server Structure

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "os/exec"
    "runtime"
)

type SSEEvent struct {
    Event string      `json:"-"`
    Data  interface{} `json:"data"`
}

func setupHandler(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Send events as scan progresses
    sendEvent(w, flusher, "preflight", map[string]string{
        "check":  "binary",
        "status": "pass",
        "detail": "gres-b2b v4.0.0",
    })

    // ... continue with scan phases ...
}

func sendEvent(w http.ResponseWriter, f http.Flusher, event string, data interface{}) {
    jsonData, _ := json.Marshal(data)
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
    f.Flush()
}
```

### Browser Opening

```go
func openBrowser(url string) error {
    var cmd string
    var args []string

    switch runtime.GOOS {
    case "windows":
        cmd = "cmd"
        args = []string{"/c", "start", url}
    case "darwin":
        cmd = "open"
        args = []string{url}
    default: // linux
        cmd = "xdg-open"
        args = []string{url}
    }

    return exec.Command(cmd, args...).Start()
}
```

### Main Setup Flow

```go
func runSetupMode(port int, workspace string, noBrowser bool) error {
    // 1. Start HTTP server on port
    mux := http.NewServeMux()
    mux.HandleFunc("/events", setupHandler)
    mux.Handle("/", http.FileServer(http.Dir("docs/onboarding")))

    server := &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: mux,
    }

    go server.ListenAndServe()

    // 2. Open browser to progress page
    progressURL := fmt.Sprintf("http://localhost:%d/progress.html?port=%d&workspace=%s",
        port, port, url.QueryEscape(workspace))

    if !noBrowser {
        openBrowser(progressURL)
    }

    fmt.Printf("Setup server running at http://localhost:%d\n", port)
    fmt.Printf("Progress page: %s\n", progressURL)

    // 3. Run scan with event streaming
    // (the HTTP handler streams events as scan progresses)

    return nil
}
```

## Preflight Checks

The binary must verify:

1. **Binary exists** - Self-check (always passes)
2. **Git available** - Run `git --version`
3. **Config valid** - Parse `config.toml` if exists
4. **Workspace exists** - Check directory exists and is readable

## Browser Progress Page

The `docs/onboarding/progress.html` file is designed to:
- Connect to SSE endpoint at `http://localhost:{port}/events`
- Display preflight check status
- Show phase progress with animations
- Stream live terminal output
- Display final RED/AMBER/GREEN results
- Offer actions: Open Report, Run Auto-Fix, Scan Again

## Integration with Wizard

The main wizard (`index.html`) includes a "Live Progress" tab that:
1. Shows the `gres-b2b --setup` command
2. Links to `progress.html` documentation
3. Explains what the command will do

When the user runs `gres-b2b --setup`:
1. The Go binary starts an HTTP server
2. Opens `progress.html` in default browser
3. Streams scan events via SSE
4. progress.html displays live updates
5. On completion, shows results with action buttons

## Testing

```bash
# Start setup mode on default port
gres-b2b --setup

# Start on custom port
gres-b2b --setup --port=9000

# Start without opening browser
gres-b2b --setup --no-browser

# Scan specific workspace
gres-b2b --setup --workspace=/path/to/project
```

## Error Handling

If SSE connection fails:
- progress.html shows "Connection lost - retrying..."
- SSE auto-reconnects (built into EventSource)
- After 3 failed retries, show manual fallback instructions

If port is in use:
- Try next port (8766, 8767, etc.)
- Report actual port to user

## Security Considerations

- Server only binds to localhost (127.0.0.1)
- No authentication required (local-only)
- CORS headers allow browser access
- Server shuts down after scan completes (or timeout)
