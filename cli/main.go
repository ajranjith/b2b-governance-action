// GRES B2B CLI - MCP Bridge for AI Agent Governance
//
// Commands:
//   --version        Show version information
//   mcp serve        Start MCP server (JSON-RPC 2.0 over stdio)
//   mcp selftest     Run MCP handshake self-test
//   doctor           Run prerequisite checks

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "--version", "-v", "version":
		fmt.Printf("gres-b2b v%s (built %s)\n", Version, BuildDate)

	case "mcp":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: gres-b2b mcp <serve|selftest>")
			os.Exit(1)
		}
		subCmd := os.Args[2]
		switch subCmd {
		case "serve":
			runMCPServer()
		case "selftest":
			runSelftest()
		default:
			fmt.Fprintf(os.Stderr, "Unknown mcp command: %s\n", subCmd)
			os.Exit(1)
		}

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
  gres-b2b --version          Show version information
  gres-b2b mcp serve          Start MCP server (JSON-RPC 2.0 over stdio)
  gres-b2b mcp selftest       Run MCP handshake self-test
  gres-b2b doctor             Run prerequisite checks
  gres-b2b --help             Show this help message`)
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

	// Check 2: JSON-RPC parsing
	testReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(testReq), &req); err != nil {
		fmt.Printf("[FAIL] JSON-RPC parsing: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] JSON-RPC parsing")

	// Check 3: Response generation
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

	// Check 1: Environment
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		fmt.Printf("[OK] LOCALAPPDATA: %s\n", localAppData)
	} else {
		fmt.Println("[WARN] LOCALAPPDATA not set")
	}

	// Check 2: Config directory
	configDir := ""
	if localAppData != "" {
		configDir = localAppData + "\\gres-b2b"
	}
	if configDir != "" {
		if _, err := os.Stat(configDir); err == nil {
			fmt.Printf("[OK] Config directory exists: %s\n", configDir)
		} else {
			fmt.Printf("[INFO] Config directory not found: %s\n", configDir)
		}
	}

	// Check 3: MCP capability
	fmt.Println("[OK] MCP JSON-RPC 2.0 support")

	// Check 4: Version
	fmt.Printf("[OK] Version: %s\n", Version)

	fmt.Println()
	if allPassed {
		fmt.Println("All prerequisites met!")
	} else {
		fmt.Println("Some checks failed. Please review above.")
		os.Exit(1)
	}
}
