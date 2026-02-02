package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ajranjith/b2b-governance-action/cli/internal/support"
)

type Agent struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	ConfigPath     string   `json:"configPath"`
	ConfigType     string   `json:"configType"`
	MCPKey         string   `json:"mcpKey"`
	GresKey        string   `json:"gresKey"`
	ProcessNames   []string `json:"processNames,omitempty"`
	RestartMessage string   `json:"restartMessage,omitempty"`
	Icon           string   `json:"icon,omitempty"`
	ConfigExists   bool     `json:"configExists"`
	ConfigValid    bool     `json:"configValid"`
	ConfigError    string   `json:"configError,omitempty"`
	HasGres        bool     `json:"hasGres"`
	Status         string   `json:"status"`
}

type AgentDetectEntry struct {
	ClientName        string `json:"clientName"`
	Installed         bool   `json:"installed"`
	ConfigPath        string `json:"configPath"`
	ConfigFormat      string `json:"configFormat"`
	CanWrite          bool   `json:"canWrite"`
	AlreadyConfigured bool   `json:"alreadyConfigured"`
	Notes             string `json:"notes,omitempty"`
}

type DetectResult struct {
	Success          bool               `json:"success"`
	Agents           []Agent            `json:"agents"`
	HasAgents        bool               `json:"hasAgents"`
	MultipleAgents   bool               `json:"multipleAgents"`
	IsManualFallback bool               `json:"isManualFallback"`
	Entries          []AgentDetectEntry `json:"entries"`
}

type AgentSignature struct {
	Name           string
	ConfigPaths    []string
	ConfigType     string
	MCPKey         string
	GresKey        string
	ProcessNames   []string
	RestartMessage string
	Icon           string
}

func defaultSignatures(home string) []AgentSignature {
	appData := os.Getenv("APPDATA")
	userProfile := os.Getenv("USERPROFILE")
	codexHome := os.Getenv("CODEX_HOME")
	if userProfile == "" {
		userProfile = home
	}
	if codexHome == "" {
		codexHome = filepath.Join(userProfile, ".codex")
	}

	pathsCursor := []string{}
	if appData != "" {
		pathsCursor = append(pathsCursor, filepath.Join(appData, "Cursor", "mcp.json"))
	}
	pathsCursor = append(pathsCursor, filepath.Join(home, ".cursor", "mcp.json"))

	pathsClaude := []string{}
	if appData != "" {
		pathsClaude = append(pathsClaude, filepath.Join(appData, "Claude", "claude_desktop_config.json"))
	}
	pathsClaude = append(pathsClaude,
		filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"),
		filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
	)

	pathsWindsurf := []string{}
	if userProfile != "" {
		pathsWindsurf = append(pathsWindsurf, filepath.Join(userProfile, ".codeium", "windsurf", "mcp_config.json"))
	}
	if appData != "" {
		pathsWindsurf = append(pathsWindsurf, filepath.Join(appData, "Windsurf", "User", "mcp.json"))
	}
	pathsWindsurf = append(pathsWindsurf, filepath.Join(home, ".config", "windsurf", "mcp.json"))

	pathsCodex := []string{filepath.Join(codexHome, "config.toml")}
	pathsCodex = append(pathsCodex, filepath.Join(home, ".codex", "config.toml"))
	pathsCodex = append(pathsCodex, filepath.Join(home, ".config", "codex", "config.toml"))

	return []AgentSignature{
		{
			Name:           "Cursor",
			ConfigType:     "json",
			MCPKey:         "mcpServers",
			GresKey:        "gres-b2b",
			ConfigPaths:    pathsCursor,
			ProcessNames:   []string{"Cursor.exe", "cursor.exe", "Cursor"},
			RestartMessage: "Please close and reopen Cursor to apply changes.",
			Icon:           "code",
		},
		{
			Name:           "Claude Desktop",
			ConfigType:     "json",
			MCPKey:         "mcpServers",
			GresKey:        "gres-b2b",
			ConfigPaths:    pathsClaude,
			ProcessNames:   []string{"Claude.exe", "claude.exe", "Claude"},
			RestartMessage: "Please close and reopen Claude Desktop to apply changes.",
			Icon:           "chat",
		},
		{
			Name:           "Windsurf",
			ConfigType:     "json",
			MCPKey:         "mcpServers",
			GresKey:        "gres-b2b",
			ConfigPaths:    pathsWindsurf,
			ProcessNames:   []string{"Windsurf.exe", "windsurf.exe", "Windsurf"},
			RestartMessage: "Please close and reopen Windsurf to apply changes.",
			Icon:           "wind",
		},
		{
			Name:           "Codex CLI",
			ConfigType:     "toml",
			MCPKey:         "mcp.servers",
			GresKey:        "gres_b2b",
			ConfigPaths:    pathsCodex,
			ProcessNames:   []string{"codex", "codex.exe"},
			RestartMessage: "Please restart your terminal/CLI session to apply changes.",
			Icon:           "terminal",
		},
		{
			Name:       "Generic MCP",
			ConfigType: "json",
			MCPKey:     "servers",
			GresKey:    "gres-b2b",
			ConfigPaths: []string{
				filepath.Join(home, ".mcp", "config.json"),
				filepath.Join(home, ".config", "mcp", "servers.json"),
			},
			RestartMessage: "Please restart your MCP client to apply changes.",
			Icon:           "puzzle",
		},
	}
}

func detectAgents(ctx Context, state *State, root string) error {
	result := DetectAgents(ctx)
	state.DetectedAgents = result.Agents
	path := filepath.Join(root, ".b2b", "agent-detect.json")
	return support.WriteJSONAtomic(path, map[string]interface{}{
		"generatedAtUtc": time.Now().UTC().Format(time.RFC3339),
		"clients":        result.Entries,
	})
}

func DetectAgents(ctx Context) DetectResult {
	home := ctx.HomeDir
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}

	detected := []Agent{}
	entries := []AgentDetectEntry{}
	for _, sig := range defaultSignatures(home) {
		entry := AgentDetectEntry{ClientName: sig.Name, Installed: false, ConfigFormat: sig.ConfigType}
		found := false
		for _, configPath := range sig.ConfigPaths {
			if configPath == "" {
				continue
			}
			if _, err := os.Stat(configPath); err == nil {
				agent := Agent{
					ID:             toAgentID(sig.Name),
					Name:           sig.Name,
					ConfigPath:     configPath,
					ConfigType:     sig.ConfigType,
					MCPKey:         sig.MCPKey,
					GresKey:        sig.GresKey,
					ProcessNames:   sig.ProcessNames,
					RestartMessage: sig.RestartMessage,
					Icon:           sig.Icon,
					ConfigExists:   true,
					Status:         "DETECTED",
				}

				data, err := os.ReadFile(configPath)
				if err != nil {
					agent.ConfigValid = false
					agent.ConfigError = err.Error()
				} else if sig.ConfigType == "toml" {
					parsed := parseTOML(string(data))
					agent.ConfigValid = true
					agent.HasGres = hasGresToml(parsed)
				} else {
					parsed, parseErr := parseJSONSafe(data)
					if parseErr != nil {
						agent.ConfigValid = false
						agent.ConfigError = parseErr.Error()
					} else {
						agent.ConfigValid = true
						agent.HasGres = hasGresJSON(parsed, sig.MCPKey, sig.GresKey)
					}
				}

				detected = append(detected, agent)
				entry.Installed = true
				entry.ConfigPath = configPath
				entry.CanWrite = canWritePath(configPath)
				entry.AlreadyConfigured = agent.HasGres
				found = true
				break
			}
		}
		if !found {
			entry.Notes = "config not found"
		}
		entries = append(entries, entry)
	}

	if len(detected) == 0 {
		fallback := manualFallback(home)
		detected = []Agent{fallback}
		entries = append(entries, AgentDetectEntry{
			ClientName:        fallback.Name,
			Installed:         false,
			ConfigPath:        fallback.ConfigPath,
			ConfigFormat:      fallback.ConfigType,
			CanWrite:          canWritePath(fallback.ConfigPath),
			AlreadyConfigured: false,
			Notes:             "manual configuration required",
		})
		return DetectResult{
			Success:          true,
			Agents:           detected,
			HasAgents:        true,
			MultipleAgents:   false,
			IsManualFallback: true,
			Entries:          entries,
		}
	}

	return DetectResult{
		Success:          true,
		Agents:           detected,
		HasAgents:        true,
		MultipleAgents:   len(detected) > 1,
		IsManualFallback: false,
		Entries:          entries,
	}
}

func connectAgents(ctx Context, opts Options, state *State, root string) error {
	agents := state.DetectedAgents
	if len(agents) == 0 {
		result := DetectAgents(ctx)
		agents = result.Agents
		state.DetectedAgents = agents
	}

	selected, err := selectAgents(opts, agents)
	if err != nil {
		return err
	}
	state.SelectedAgents = selected

	logPath := filepath.Join(root, ".b2b", "agent-connect.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	exePath := ctx.ExePath
	if opts.AgentBinaryPath != "" {
		exePath = opts.AgentBinaryPath
	}
	if exePath == "" {
		if p, err := os.Executable(); err == nil {
			exePath = p
		}
	}
	if exePath == "" {
		return fmt.Errorf("binary path is required")
	}
	if !filepath.IsAbs(exePath) {
		return fmt.Errorf("binary path must be absolute")
	}

	now := time.Now
	if ctx.Now != nil {
		now = ctx.Now
	}

	for _, id := range selected {
		agent, ok := findAgentByID(agents, id)
		if !ok {
			return fmt.Errorf("unknown agent: %s", id)
		}
		if opts.AgentConfigPath != "" {
			agent.ConfigPath = opts.AgentConfigPath
			if _, err := os.Stat(agent.ConfigPath); err == nil {
				agent.ConfigExists = true
			} else {
				agent.ConfigExists = false
			}
			for i := range state.DetectedAgents {
				if state.DetectedAgents[i].ID == agent.ID {
					state.DetectedAgents[i].ConfigPath = agent.ConfigPath
					state.DetectedAgents[i].ConfigExists = agent.ConfigExists
				}
			}
		}

		entry := map[string]interface{}{
			"timestampUtc": now().UTC().Format(time.RFC3339),
			"agentId":      agent.ID,
			"agentName":    agent.Name,
			"configPath":   agent.ConfigPath,
		}

		backupPath := ""
		if agent.ConfigExists {
			backupPath = backupConfig(root, agent, now())
			if backupPath == "" {
				entry["status"] = "FAILED"
				entry["error"] = "backup failed"
				appendJSONLine(logPath, entry)
				return fmt.Errorf("backup failed for %s", agent.ConfigPath)
			}
		}

		if err := writeAgentConfig(agent, exePath); err != nil {
			entry["status"] = "FAILED"
			entry["error"] = err.Error()
			entry["backupPath"] = backupPath
			appendJSONLine(logPath, entry)
			return err
		}

		entry["status"] = "OK"
		entry["backupPath"] = backupPath
		appendJSONLine(logPath, entry)
	}

	_ = updateAgentDetect(root, selected)
	return nil
}

func validateAgents(ctx Context, opts Options, state *State, root string) error {
	agents := state.DetectedAgents
	if len(agents) == 0 {
		result := DetectAgents(ctx)
		agents = result.Agents
		state.DetectedAgents = agents
	}

	selected, err := selectAgents(opts, agents)
	if err != nil {
		return err
	}
	state.SelectedAgents = selected

	exePath := ctx.ExePath
	if opts.AgentBinaryPath != "" {
		exePath = opts.AgentBinaryPath
	}
	if exePath == "" {
		if p, err := os.Executable(); err == nil {
			exePath = p
		}
	}

	results := []map[string]interface{}{}
	for _, id := range selected {
		agent, ok := findAgentByID(agents, id)
		if !ok {
			return fmt.Errorf("unknown agent: %s", id)
		}
		res := map[string]interface{}{
			"agentId":    agent.ID,
			"agentName":  agent.Name,
			"configPath": agent.ConfigPath,
		}
		res["configHasGres"] = checkConfigHasGres(agent)
		res["exePath"] = exePath
		if exePath != "" {
			if _, err := os.Stat(exePath); err != nil {
				res["exeExists"] = false
			} else {
				res["exeExists"] = true
			}
		}
		if ctx.SkipSelftest {
			res["mcpSelftest"] = "skipped"
		} else {
			ok := runSelftest(exePath)
			res["mcpSelftest"] = ok
		}
		results = append(results, res)
	}

	path := filepath.Join(root, ".b2b", "agent-validate.json")
	return support.WriteJSONAtomic(path, map[string]interface{}{
		"validatedAtUtc": time.Now().UTC().Format(time.RFC3339),
		"results":        results,
	})
}

func selectAgents(opts Options, agents []Agent) ([]string, error) {
	if opts.AllClients {
		ids := []string{}
		for _, a := range agents {
			ids = append(ids, a.ID)
		}
		return ids, nil
	}
	if len(opts.Clients) > 0 {
		return opts.Clients, nil
	}
	if len(agents) == 1 {
		return []string{agents[0].ID}, nil
	}
	return nil, fmt.Errorf("no agent selected")
}

func manualFallback(home string) Agent {
	defaultConfig := filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
	return Agent{
		ID:           "manual",
		Name:         "Generic AI Agent",
		ConfigPath:   defaultConfig,
		ConfigType:   "json",
		MCPKey:       "mcpServers",
		GresKey:      "gres-b2b",
		ConfigExists: false,
		ConfigValid:  true,
		HasGres:      false,
		Status:       "MANUAL",
		Icon:         "puzzle",
	}
}

func toAgentID(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

func findAgentByID(agents []Agent, id string) (Agent, bool) {
	for _, a := range agents {
		if a.ID == id || strings.EqualFold(a.Name, id) {
			return a, true
		}
	}
	return Agent{}, false
}

func parseJSONSafe(data []byte) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := json.Unmarshal(support.StripBOM(data), &out); err == nil {
		return out, nil
	}
	re := regexp.MustCompile(`,(\s*[}\]])`)
	fixed := re.ReplaceAll(data, []byte("$1"))
	if err := json.Unmarshal(support.StripBOM(fixed), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseTOML(content string) map[string]interface{} {
	result := map[string]interface{}{}
	current := result
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[]")
			parts := strings.Split(section, ".")
			current = result
			for _, p := range parts {
				if _, ok := current[p]; !ok {
					current[p] = map[string]interface{}{}
				}
				next, ok := current[p].(map[string]interface{})
				if !ok {
					next = map[string]interface{}{}
					current[p] = next
				}
				current = next
			}
			continue
		}
		if strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "\"")
			current[key] = value
		}
	}
	return result
}

func hasGresToml(parsed map[string]interface{}) bool {
	if servers, ok := parsed["mcp_servers"].(map[string]interface{}); ok {
		if _, ok := servers["gres_b2b"]; ok {
			return true
		}
	}
	if mcp, ok := parsed["mcp"].(map[string]interface{}); ok {
		if servers, ok := mcp["servers"].(map[string]interface{}); ok {
			if _, ok := servers["gres_b2b"]; ok {
				return true
			}
		}
	}
	return false
}

func hasGresJSON(parsed map[string]interface{}, mcpKey, gresKey string) bool {
	mcpServers := getNested(parsed, mcpKey)
	if mcpServers == nil {
		return false
	}
	if servers, ok := mcpServers.(map[string]interface{}); ok {
		_, ok := servers[gresKey]
		return ok
	}
	return false
}

func getNested(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := interface{}(data)
	for _, p := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[p]
		if current == nil {
			return nil
		}
	}
	return current
}

func setNested(data map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := data
	for i := 0; i < len(parts)-1; i++ {
		p := parts[i]
		next, ok := current[p].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			current[p] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func writeAgentConfig(agent Agent, exePath string) error {
	if exePath == "" {
		return fmt.Errorf("exe path missing")
	}
	if agent.ConfigType == "toml" {
		return writeTomlConfig(agent, exePath)
	}
	return writeJSONConfig(agent, exePath)
}

func writeJSONConfig(agent Agent, exePath string) error {
	data := map[string]interface{}{}
	if agent.ConfigExists {
		raw, err := os.ReadFile(agent.ConfigPath)
		if err != nil {
			return err
		}
		parsed, err := parseJSONSafe(raw)
		if err != nil {
			return err
		}
		data = parsed
	}
	entry := map[string]interface{}{
		"command": exePath,
		"args":    []string{"mcp", "serve"},
	}
	setNested(data, agent.MCPKey, map[string]interface{}{})
	mcpServers := getNested(data, agent.MCPKey)
	if m, ok := mcpServers.(map[string]interface{}); ok {
		m[agent.GresKey] = entry
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(agent.ConfigPath), 0o755); err != nil {
		return err
	}
	return support.WriteFileAtomic(agent.ConfigPath, bytes)
}

func writeTomlConfig(agent Agent, exePath string) error {
	content := ""
	if agent.ConfigExists {
		raw, err := os.ReadFile(agent.ConfigPath)
		if err != nil {
			return err
		}
		content = string(raw)
	}

	escaped := strings.ReplaceAll(exePath, "\\", "\\\\")
	section := fmt.Sprintf(`
[mcp_servers."gres-b2b"]
enabled = true
command = '%s'
args = ["mcp", "serve"]
env = {}
`, escaped)

	reNew := regexp.MustCompile(`(?s)\[mcp_servers\."?gres-b2b"?\].*?(?=\n\[|$)`)
	reOld := regexp.MustCompile(`(?s)\[mcp\.servers\.gres_b2b\].*?(?=\n\[|$)`)

	if reNew.MatchString(content) {
		content = reNew.ReplaceAllString(content, strings.TrimSpace(section))
	} else if reOld.MatchString(content) {
		content = reOld.ReplaceAllString(content, "")
		content = strings.TrimSpace(content) + "\n" + strings.TrimSpace(section) + "\n"
	} else {
		content = strings.TrimSpace(content) + "\n" + strings.TrimSpace(section) + "\n"
	}

	if err := os.MkdirAll(filepath.Dir(agent.ConfigPath), 0o755); err != nil {
		return err
	}
	return support.WriteFileAtomic(agent.ConfigPath, []byte(content))
}

func backupConfig(root string, agent Agent, now time.Time) string {
	if agent.ConfigPath == "" {
		return ""
	}
	backupDir := filepath.Join(root, ".b2b", "backups", "agent-config", agent.ID, now.UTC().Format("20060102_150405"))
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return ""
	}
	bakName := fmt.Sprintf("%s.bak.%s", filepath.Base(agent.ConfigPath), now.UTC().Format("20060102_150405"))
	bakPath := filepath.Join(filepath.Dir(agent.ConfigPath), bakName)
	_ = support.CopyFileAtomic(agent.ConfigPath, bakPath)

	copyPath := filepath.Join(backupDir, filepath.Base(agent.ConfigPath))
	if err := support.CopyFileAtomic(agent.ConfigPath, copyPath); err != nil {
		return ""
	}
	return copyPath
}

func appendJSONLine(path string, data map[string]interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(bytes, '\n'))
}

func checkConfigHasGres(agent Agent) bool {
	raw, err := os.ReadFile(agent.ConfigPath)
	if err != nil {
		return false
	}
	if agent.ConfigType == "toml" {
		parsed := parseTOML(string(raw))
		return hasGresToml(parsed)
	}
	parsed, err := parseJSONSafe(raw)
	if err != nil {
		return false
	}
	return hasGresJSON(parsed, agent.MCPKey, agent.GresKey)
}

func runSelftest(exePath string) bool {
	if exePath == "" {
		return false
	}
	return runCommand(exePath, []string{"mcp", "selftest"}, 5*time.Second)
}

func runCommand(path string, args []string, timeout time.Duration) bool {
	cmd := newCommand(path, args, timeout)
	if cmd == nil {
		return false
	}
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func canWritePath(path string) bool {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return false
		}
		_ = f.Close()
		return true
	}
	test := filepath.Join(dir, ".write-test")
	f, err := os.OpenFile(test, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(test)
	return true
}

func updateAgentDetect(root string, selected []string) error {
	path := filepath.Join(root, ".b2b", "agent-detect.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw struct {
		GeneratedAtUtc string             `json:"generatedAtUtc"`
		Clients        []AgentDetectEntry `json:"clients"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for i := range raw.Clients {
		for _, sel := range selected {
			if strings.EqualFold(raw.Clients[i].ClientName, sel) || strings.EqualFold(toAgentID(raw.Clients[i].ClientName), sel) {
				raw.Clients[i].AlreadyConfigured = true
			}
		}
	}
	return support.WriteJSONAtomic(path, raw)
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
