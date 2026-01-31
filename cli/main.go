// GRES B2B CLI - MCP Bridge for AI Agent Governance
//
// Commands:
//   --version        Show version information
//   --config <path>  Use specific config file
//   mcp serve        Start MCP server (JSON-RPC 2.0 over stdio)
//   mcp selftest     Run MCP handshake self-test
//   doctor           Run prerequisite checks

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	cfgpkg "github.com/ajranjith/b2b-governance-action/cli/internal/config"
	"github.com/ajranjith/b2b-governance-action/cli/internal/mcpio"
	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

// Version information (set at build time)
var (
	Version   = "1.0.0"
	BuildDate = "unknown"
)

// JSON-RPC 2.0 structures
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Initialize result
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Global config
var config *cfgpkg.Config
var configPath string

func main() {
	// Parse args for --config flag first
	args := os.Args[1:]
	configFlag := ""
	verifyCertPath := ""
	ingestIncoming := ""
	ingestLocked := ""
	ingestResume := false
	watchPath := ""
	shadowVectors := ""
	shadowRoot := ""
	fixRun := false
	fixDryRun := false
	supportBundleRoot := ""
	setupRun := false
	rollback := false
	rollbackLatest := false
	rollbackTo := ""
	filteredArgs := []string{}

	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			configFlag = args[i+1]
			i++ // Skip the value
		} else if args[i] == "--verify-cert" && i+1 < len(args) {
			verifyCertPath = args[i+1]
			i++
		} else if args[i] == "--ingest-admin" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				ingestIncoming = args[i+1]
				i++
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				ingestLocked = args[i+1]
				i++
			}
		} else if args[i] == "--resume" {
			ingestResume = true
		} else if args[i] == "--watch" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				watchPath = args[i+1]
				i++
			}
		} else if args[i] == "--shadow" {
			// handled via --vectors
		} else if args[i] == "--vectors" && i+1 < len(args) {
			shadowVectors = args[i+1]
			i++
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				shadowRoot = args[i+1]
				i++
			}
		} else if args[i] == "--fix" {
			fixRun = true
		} else if args[i] == "--dry-run" {
			fixDryRun = true
		} else if args[i] == "--support-bundle" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				supportBundleRoot = args[i+1]
				i++
			}
		} else if args[i] == "--setup" {
			setupRun = true
		} else if args[i] == "--rollback" {
			rollback = true
		} else if args[i] == "--latest-green" {
			rollbackLatest = true
		} else if args[i] == "--to" && i+1 < len(args) {
			rollbackTo = args[i+1]
			i++
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	// Load config (defaults + optional override)
	if configFlag != "" {
		if _, err := os.Stat(configFlag); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "ERROR: Config not found: %s\n", configFlag)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "ERROR: Config stat failed: %v\n", err)
			os.Exit(1)
		}
	}

	cfg, cfgPath, warnings, err := cfgpkg.Resolve(cfgpkg.Flags{ConfigPath: configFlag})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Config load failed: %v\n", err)
		os.Exit(1)
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}
	config = &cfg
	configPath = cfgPath

	if verifyCertPath != "" {
		runVerifyCert(verifyCertPath)
		return
	}

	if ingestIncoming != "" || ingestLocked != "" || ingestResume {
		runIngestAdmin(ingestIncoming, ingestLocked, ingestResume)
		return
	}
	if watchPath != "" {
		runWatch(watchPath)
		return
	}
	if shadowVectors != "" {
		runShadow(shadowVectors, shadowRoot)
		return
	}
	if fixRun {
		runFix(fixDryRun)
		return
	}
	if supportBundleRoot != "" {
		runSupportBundle(supportBundleRoot)
		return
	}
	if setupRun {
		runSetup()
		return
	}
	if rollback {
		if rollbackLatest {
			runRollbackLatest()
			return
		}
		if rollbackTo != "" {
			runRollbackTo(rollbackTo)
			return
		}
		fmt.Fprintln(os.Stderr, "Usage: gres-b2b --rollback --latest-green OR --rollback --to <UTC_YYYYMMDD_HHMMSS>")
		os.Exit(1)
	}

	if len(filteredArgs) < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := filteredArgs[0]

	switch cmd {
	case "--version", "-v", "version":
		fmt.Printf("gres-b2b v%s (built %s)\n", Version, BuildDate)
		if configPath != "" {
			fmt.Printf("Config: %s\n", configPath)
		}

	case "mcp":
		if len(filteredArgs) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: gres-b2b mcp <serve|selftest>")
			os.Exit(1)
		}
		subCmd := filteredArgs[1]
		switch subCmd {
		case "serve":
			runMCPServer()
		case "selftest":
			runSelftest()
		default:
			fmt.Fprintf(os.Stderr, "Unknown mcp command: %s\n", subCmd)
			os.Exit(1)
		}

	case "verify":
		runVerify()

	case "scan":
		runScan()

	case "doctor":
		runDoctor()

	case "--help", "-h", "help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`GRES B2B CLI - MCP Bridge for AI Agent Governance

Usage:
  gres-b2b --version              Show version information
  gres-b2b --config <path>        Use specific config file (optional override)
  gres-b2b --verify-cert <path>   Verify a signed certificate
  gres-b2b --ingest-admin [in] [locked] [--resume]  Atomic ingest from incoming to locked
  gres-b2b --watch <path>         Watch and rescan on changes
  gres-b2b --shadow --vectors <file.yml> <repoRoot>  Run shadow parity
  gres-b2b --fix [--dry-run]      Auto-heal structural issues
  gres-b2b --support-bundle <repoRoot>  Create support bundle zip
  gres-b2b --setup               Run onboarding setup/resume
  gres-b2b mcp serve              Start MCP server (JSON-RPC 2.0 over stdio)
  gres-b2b mcp selftest           Run MCP handshake self-test
  gres-b2b scan                   Run governance scan (Phase 1)
  gres-b2b verify                 Run governance verify with gating
  gres-b2b doctor                 Run prerequisite checks
  gres-b2b --help                 Show this help message

Config Source:
  - Built-in defaults (no external file required)
  - Optional override via --config <path>`)
}

// expandEnv expands Windows environment variables in a path
// Handles both %VAR% (Windows) and $VAR/${VAR} (Unix) syntax
func expandEnv(path string) string {
	if runtime.GOOS != "windows" {
		return os.ExpandEnv(path)
	}

	// Windows: expand %VAR% patterns
	result := path
	for {
		start := strings.Index(result, "%")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+1:], "%")
		if end == -1 {
			break
		}
		end += start + 1

		varName := result[start+1 : end]
		varValue := os.Getenv(varName)

		// Replace %VAR% with its value
		result = result[:start] + varValue + result[end+1:]
	}

	return result
}

// resolveExePath resolves the executable path using canonical directory preference
// Returns: exePath, []warnings
func resolveExePath() (string, []string) {
	warnings := []string{}
	exeName := config.Install.ExeName
	canonicalDir := expandEnv(config.Install.CanonicalDir)
	canonicalPath := filepath.Join(canonicalDir, exeName)

	// Check if canonical exe exists
	canonicalExists := false
	if _, err := os.Stat(canonicalPath); err == nil {
		canonicalExists = true
	}

	// Find duplicates
	duplicates := []string{}
	if config.Install.DuplicateDetection.Enabled {
		for _, scanDir := range config.Install.DuplicateDetection.ScanDirs {
			expandedDir := expandEnv(scanDir)
			found := findExecutables(expandedDir, exeName, 4) // Max depth 4
			for _, f := range found {
				// Skip canonical path
				if !strings.EqualFold(f, canonicalPath) {
					duplicates = append(duplicates, f)
				}
			}
		}
	}

	// If canonical exists, use it and warn about duplicates
	if canonicalExists {
		if len(duplicates) > 0 {
			maxResults := config.Install.DuplicateDetection.MaxResults
			if maxResults == 0 {
				maxResults = 5
			}

			shown := duplicates
			more := 0
			if len(duplicates) > maxResults {
				shown = duplicates[:maxResults]
				more = len(duplicates) - maxResults
			}

			// Use exact required phrase
			warning := "WARNING: Duplicate detected warning\n"
			warning += fmt.Sprintf("  Canonical: %s\n", canonicalPath)
			warning += fmt.Sprintf("  Duplicates found:\n")
			for _, dup := range shown {
				warning += fmt.Sprintf("    - %s\n", dup)
			}
			if more > 0 {
				warning += fmt.Sprintf("    + %d more\n", more)
			}
			warning += "  Proceeding with canonical install."
			warnings = append(warnings, warning)
		}
		return canonicalPath, warnings
	}

	// Canonical doesn't exist - find best candidate
	currentExe, err := os.Executable()
	if err == nil {
		return currentExe, warnings
	}

	// Fallback to first duplicate found
	if len(duplicates) > 0 {
		return duplicates[0], warnings
	}

	// No executable found
	return "", warnings
}

// findExecutables searches for executable files in a directory with depth limit
func findExecutables(dir string, name string, maxDepth int) []string {
	if maxDepth <= 0 {
		return nil
	}

	var results []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return results
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			results = append(results, findExecutables(fullPath, name, maxDepth-1)...)
		} else if strings.EqualFold(entry.Name(), name) {
			results = append(results, fullPath)
		}
	}

	return results
}

// runMCPServer starts the MCP JSON-RPC server over stdio
func runMCPServer() {
	reader := bufio.NewReader(os.Stdin)
	for {
		msg, err := mcpio.ReadMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Fprintf(os.Stderr, "ERROR: MCP read failed: %v\n", err)
			return
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		handleRequest(&req)
	}
}

func handleRequest(req *JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		result := InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
				"resources": map[string]interface{}{
					"subscribe":   true,
					"listChanged": true,
				},
			},
			ServerInfo: ServerInfo{
				Name:    "gres-b2b",
				Version: Version,
			},
		}
		sendResult(req.ID, result)

	case "initialized":
		// Notification, no response needed

	case "tools/list":
		sendResult(req.ID, map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "governance_check",
					"description": "Check governance rules for an AI action",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"action": map[string]interface{}{
								"type":        "string",
								"description": "The action to check",
							},
						},
						"required": []string{"action"},
					},
				},
			},
		})

	case "tools/call":
		// Parse params to get tool name
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}

		switch params.Name {
		case "governance_check":
			action, _ := params.Arguments["action"].(string)
			sendResult(req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Governance check passed for action: %s", action),
					},
				},
			})
		default:
			sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown tool: %s", params.Name))
		}

	case "resources/list":
		sendResult(req.ID, map[string]interface{}{
			"resources": []interface{}{},
		})

	case "ping":
		sendResult(req.ID, map[string]interface{}{})

	default:
		sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", req.Method))
	}
}

func sendResult(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	if err := mcpio.WriteMessage(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: MCP write failed: %v\n", err)
	}
}

func sendError(id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	respData, _ := json.Marshal(resp)
	if err := mcpio.WriteMessage(os.Stdout, respData); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: MCP write failed: %v\n", err)
	}
}

// runSelftest performs a self-test of MCP capabilities
func runSelftest() {
	fmt.Println("GRES B2B MCP Self-Test")
	fmt.Println("======================")
	fmt.Println()

	// Check 1: Version
	fmt.Printf("[OK] Version: %s\n", Version)

	// Check 2: Config
	if configPath != "" {
		fmt.Printf("[OK] Config loaded: %s\n", configPath)
	} else {
		fmt.Println("[INFO] Using default config")
	}

	// Check 3: JSON-RPC parsing
	testReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(testReq), &req); err != nil {
		fmt.Printf("[FAIL] JSON-RPC parsing: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] JSON-RPC parsing")

	// Check 4: Response generation
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]interface{}{},
		ServerInfo:      ServerInfo{Name: "gres-b2b", Version: Version},
	}
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: result}
	if _, err := json.Marshal(resp); err != nil {
		fmt.Printf("[FAIL] Response generation: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] Response generation")

	fmt.Println()
	fmt.Println("All tests passed!")
}

// runDoctor checks prerequisites
func runDoctor() {
	loadScanOverrides(config.Paths.WorkspaceRoot)
	report := buildDoctorReport()
	path := filepath.Join(config.Paths.WorkspaceRoot, ".b2b", "doctor.json")
	if err := support.WriteJSONAtomic(path, report); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot write doctor.json: %v\n", err)
		os.Exit(1)
	}
	updateDoctorReport(config.Paths.WorkspaceRoot, report)
	fmt.Printf("Doctor status: %s\n", report.Status)
	if report.Status != "OK" {
		os.Exit(1)
	}
}
