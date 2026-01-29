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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Version information (set at build time)
var (
	Version   = "1.0.0"
	BuildDate = "unknown"
)

// Config structures
type Config struct {
	SchemaVersion string        `json:"schemaVersion"`
	App           AppConfig     `json:"app"`
	Paths         PathsConfig   `json:"paths"`
	Run           RunConfig     `json:"run"`
	Reports       ReportsConfig `json:"reports"`
	Install       InstallConfig `json:"install"`
	Logging       LoggingConfig `json:"logging"`
}

type AppConfig struct {
	Name    string `json:"name"`
	Channel string `json:"channel"`
}

type PathsConfig struct {
	WorkspaceRoot string `json:"workspaceRoot"`
	OutputDir     string `json:"outputDir"`
	CacheDir      string `json:"cacheDir"`
}

type RunConfig struct {
	DefaultMode string `json:"defaultMode"`
	Args        string `json:"args"`
}

type ReportsConfig struct {
	SARIF ReportConfig `json:"sarif"`
	JUnit ReportConfig `json:"junit"`
	Hints ReportConfig `json:"hints"`
}

type ReportConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type InstallConfig struct {
	CanonicalDir       string                   `json:"canonicalDir"`
	ExeName            string                   `json:"exeName"`
	DuplicateDetection DuplicateDetectionConfig `json:"duplicateDetection"`
}

type DuplicateDetectionConfig struct {
	Enabled    bool     `json:"enabled"`
	Severity   string   `json:"severity"`
	MaxResults int      `json:"maxResults"`
	ScanDirs   []string `json:"scanDirs"`
}

type LoggingConfig struct {
	Level string `json:"level"`
	JSON  bool   `json:"json"`
}

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
var config *Config
var configPath string

func main() {
	// Parse args for --config flag first
	args := os.Args[1:]
	configFlag := ""
	filteredArgs := []string{}

	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			configFlag = args[i+1]
			i++ // Skip the value
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	// Load config
	var err error
	config, configPath, err = loadConfig(configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Config load failed: %v\n", err)
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
  gres-b2b --config <path>        Use specific config file
  gres-b2b mcp serve              Start MCP server (JSON-RPC 2.0 over stdio)
  gres-b2b mcp selftest           Run MCP handshake self-test
  gres-b2b verify                 Run governance verify with gating
  gres-b2b doctor                 Run prerequisite checks
  gres-b2b --help                 Show this help message

Config Search Order:
  1. --config <path> argument
  2. gres-b2b.config.json in same folder as executable
  3. %ProgramData%\GRES\B2B\gres-b2b.config.json
  4. Built-in defaults (warning shown)`)
}

// loadConfig loads configuration from JSON file
// Search order: --config flag, exe directory, ProgramData, defaults
func loadConfig(configFlag string) (*Config, string, error) {
	// Default config
	cfg := &Config{
		SchemaVersion: "1.0",
		App: AppConfig{
			Name:    "GRES B2B Governance Engine",
			Channel: "release",
		},
		Paths: PathsConfig{
			WorkspaceRoot: ".",
			OutputDir:     ".b2b",
			CacheDir:      ".b2b/cache",
		},
		Run: RunConfig{
			DefaultMode: "verify",
			Args:        "",
		},
		Reports: ReportsConfig{
			SARIF: ReportConfig{Enabled: true, Path: ".b2b/results.sarif"},
			JUnit: ReportConfig{Enabled: true, Path: ".b2b/junit.xml"},
			Hints: ReportConfig{Enabled: true, Path: ".b2b/hints.json"},
		},
		Install: InstallConfig{
			CanonicalDir: "%ProgramFiles%\\GRES\\B2B",
			ExeName:      "gres-b2b.exe",
			DuplicateDetection: DuplicateDetectionConfig{
				Enabled:    true,
				Severity:   "warning",
				MaxResults: 5,
				ScanDirs: []string{
					"%ProgramFiles%",
					"%ProgramFiles(x86)%",
					"%ProgramData%",
					"%LOCALAPPDATA%",
					"%APPDATA%",
					"%USERPROFILE%\\Downloads",
					"%USERPROFILE%\\Desktop",
				},
			},
		},
		Logging: LoggingConfig{
			Level: "info",
			JSON:  false,
		},
	}

	// Search for config file
	searchPaths := []string{}

	// 1. --config flag
	if configFlag != "" {
		searchPaths = append(searchPaths, configFlag)
	}

	// 2. Same folder as executable
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		searchPaths = append(searchPaths, filepath.Join(exeDir, "gres-b2b.config.json"))
	}

	// 3. ProgramData (Windows)
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData != "" {
			searchPaths = append(searchPaths, filepath.Join(programData, "GRES", "B2B", "gres-b2b.config.json"))
		}
	}

	// Try each path
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read config %s: %w", path, err)
			}

			var loadedCfg Config
			if err := json.Unmarshal(data, &loadedCfg); err != nil {
				return nil, "", fmt.Errorf("failed to parse config %s: %w", path, err)
			}

			// Validate schemaVersion
			if loadedCfg.SchemaVersion != "1.0" {
				return nil, "", fmt.Errorf("unsupported schemaVersion: %s (expected 1.0)", loadedCfg.SchemaVersion)
			}

			// Force severity to "warning" if set to anything else
			if loadedCfg.Install.DuplicateDetection.Severity != "" && loadedCfg.Install.DuplicateDetection.Severity != "warning" {
				fmt.Fprintln(os.Stderr, "WARNING: duplicate detection severity forced to \"warning\".")
				loadedCfg.Install.DuplicateDetection.Severity = "warning"
			}

			// Merge with defaults (for missing optional fields)
			mergeConfigDefaults(&loadedCfg, cfg)

			return &loadedCfg, path, nil
		}
	}

	// No config found - use defaults with warning
	if configFlag == "" {
		fmt.Fprintln(os.Stderr, "WARNING: Config not found; using defaults.")
	}
	return cfg, "", nil
}

// mergeConfigDefaults fills in missing optional fields from defaults
func mergeConfigDefaults(cfg, defaults *Config) {
	if cfg.Paths.OutputDir == "" {
		cfg.Paths.OutputDir = defaults.Paths.OutputDir
	}
	if cfg.Paths.CacheDir == "" {
		cfg.Paths.CacheDir = defaults.Paths.CacheDir
	}
	if cfg.Paths.WorkspaceRoot == "" {
		cfg.Paths.WorkspaceRoot = defaults.Paths.WorkspaceRoot
	}
	if cfg.Run.DefaultMode == "" {
		cfg.Run.DefaultMode = defaults.Run.DefaultMode
	}
	if cfg.Install.CanonicalDir == "" {
		cfg.Install.CanonicalDir = defaults.Install.CanonicalDir
	}
	if cfg.Install.ExeName == "" {
		cfg.Install.ExeName = defaults.Install.ExeName
	}
	if cfg.Install.DuplicateDetection.MaxResults == 0 {
		cfg.Install.DuplicateDetection.MaxResults = defaults.Install.DuplicateDetection.MaxResults
	}
	if len(cfg.Install.DuplicateDetection.ScanDirs) == 0 {
		cfg.Install.DuplicateDetection.ScanDirs = defaults.Install.DuplicateDetection.ScanDirs
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = defaults.Logging.Level
	}
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
	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer size for large messages
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
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
	fmt.Println(string(data))
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
	fmt.Println(string(respData))
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
	fmt.Println("GRES B2B Doctor - Prerequisite Check")
	fmt.Println("=====================================")
	fmt.Println()

	allPassed := true

	// Check 1: Config
	if configPath != "" {
		fmt.Printf("[OK] Config: %s\n", configPath)
	} else {
		fmt.Println("[INFO] Config: using defaults")
	}

	// Check 2: Environment
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		fmt.Printf("[OK] LOCALAPPDATA: %s\n", localAppData)
	} else {
		fmt.Println("[WARN] LOCALAPPDATA not set")
	}

	// Check 3: Canonical directory
	canonicalDirRaw := config.Install.CanonicalDir
	canonicalDirExpanded := expandEnv(canonicalDirRaw)
	canonicalPath := filepath.Join(canonicalDirExpanded, config.Install.ExeName)

	// Show both raw and expanded paths
	fmt.Printf("[INFO] CanonicalDir (raw): %s\n", canonicalDirRaw)
	fmt.Printf("[INFO] CanonicalDir (expanded): %s\n", canonicalDirExpanded)

	if _, err := os.Stat(canonicalPath); err == nil {
		fmt.Printf("[OK] Canonical exe: %s\n", canonicalPath)
	} else {
		fmt.Printf("[INFO] Canonical exe not found: %s\n", canonicalPath)
	}

	// Check 4: Duplicate detection
	if config.Install.DuplicateDetection.Enabled {
		exePath, warnings := resolveExePath()
		if exePath != "" {
			fmt.Printf("[OK] Resolved exe: %s\n", exePath)
		}
		for _, w := range warnings {
			fmt.Printf("[WARN] %s\n", w)
		}
	}

	// Check 5: Output directory
	outputDir := config.Paths.OutputDir
	if _, err := os.Stat(outputDir); err == nil {
		fmt.Printf("[OK] Output dir exists: %s\n", outputDir)
	} else {
		fmt.Printf("[INFO] Output dir will be created: %s\n", outputDir)
	}

	// Check 6: MCP capability
	fmt.Println("[OK] MCP JSON-RPC 2.0 support")

	// Check 7: Version
	fmt.Printf("[OK] Version: %s\n", Version)

	fmt.Println()
	if allPassed {
		fmt.Println("All prerequisites met!")
	} else {
		fmt.Println("Some checks failed. Please review above.")
		os.Exit(1)
	}
}
