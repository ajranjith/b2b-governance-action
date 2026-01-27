// GRES B2B Bootstrapper - State Machine Wizard EXE
//
// Deliverable: Single Windows EXE (GRES-B2B-Installer.exe)
// - Runs asInvoker (no elevation required)
// - Uses native dialogs (dlgs)
// - Automates install + PATH + config + verification
// - Opens onboarding success page at the end
//
// State Machine Flow:
// Step 1: Welcome (Permission Gate)
// Step 2: Agent Select (Input Gate)
// Step 3: Project Location (Input Gate)
// Step 4: Automation Install (Silent)
// Step 5: Verification (Gatekeeper)

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/gen2brain/dlgs"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ============================================================================
// Configuration Constants
// ============================================================================

const (
	GitHubOwner = "ajranjith"
	GitHubRepo  = "b2b-governance-action"

	OnboardingReadyURL = "https://ajranjith.github.io/b2b-governance-action/onboarding/?status=ready"

	InstallSubdir = `Programs\gres-b2b`
	BinaryName    = "gres-b2b.exe"

	CfgSubdir = `gres-b2b`
	CfgName   = "config.toml"

	// Timeouts
	DownloadTimeout = 2 * time.Minute
	CmdTimeout      = 30 * time.Second
)

// Agent options for selection
var AgentOptions = []string{
	"Claude Desktop",
	"Cursor",
	"VS Code (Windsurf)",
	"Codex CLI",
	"Custom MCP",
}

// ============================================================================
// GitHub Release Types
// ============================================================================

type GitHubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ============================================================================
// Main Entry Point - State Machine Wizard
// ============================================================================

func main() {
	if runtime.GOOS != "windows" {
		nativeError("Platform Error", "This installer is Windows-only.")
		os.Exit(1)
	}

	// ========================================================================
	// STEP 1: WELCOME (Permission Gate)
	// ========================================================================
	cont, _ := dlgs.Question("GRES B2B Setup",
		"This will install GRES B2B Governance tools.\n\n"+
			"What will be installed:\n"+
			"  - gres-b2b CLI tool\n"+
			"  - PATH configuration\n"+
			"  - Agent configuration file\n\n"+
			"No admin rights required.\n\n"+
			"Continue?", true)
	if !cont {
		os.Exit(0)
	}

	// ========================================================================
	// STEP 2: AGENT SELECT (Input Gate)
	// ========================================================================
	selectedAgent := selectAgent()
	if selectedAgent == "" {
		dlgs.Error("Required", "You must select an AI agent to continue.")
		os.Exit(1)
	}

	// ========================================================================
	// STEP 3: PROJECT LOCATION (Input Gate)
	// ========================================================================
	projectPath, ok, _ := dlgs.File("Select Project Folder", "", true)
	if !ok || strings.TrimSpace(projectPath) == "" {
		dlgs.Error("Required", "You must select a project folder to continue.")
		os.Exit(1)
	}
	if err := validateFolder(projectPath); err != nil {
		dlgs.Error("Invalid Folder", err.Error())
		os.Exit(1)
	}

	// Resolve paths
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		nativeError("Setup Error", "LOCALAPPDATA environment variable is not set.")
		os.Exit(1)
	}

	installPath := filepath.Join(localAppData, InstallSubdir)
	binPath := filepath.Join(installPath, BinaryName)
	cfgDir := filepath.Join(localAppData, CfgSubdir)
	cfgPath := filepath.Join(cfgDir, CfgName)

	// ========================================================================
	// STEP 4: AUTOMATION INSTALL (Silent)
	// ========================================================================
	dlgs.Info("Installing", "Installing gres-b2b and configuring your environment...\n\nThis may take a moment.")

	// Create directories
	if err := os.MkdirAll(installPath, 0o755); err != nil {
		nativeError("Install Failed", "Could not create install directory:\n"+err.Error())
		os.Exit(1)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		nativeError("Install Failed", "Could not create config directory:\n"+err.Error())
		os.Exit(1)
	}

	// Download binary from GitHub Releases
	version, err := downloadLatestBinary(binPath)
	if err != nil {
		nativeError("Download Failed", "Could not download gres-b2b.exe:\n\n"+err.Error())
		os.Exit(1)
	}

	// PATH update (HKCU Environment) - idempotent
	if err := addToUserPath(installPath); err != nil {
		nativeError("PATH Update Failed", "Could not update PATH:\n\n"+err.Error()+"\n\nYou may need to add this folder to PATH manually:\n"+installPath)
		// Continue anyway - binary is installed
	}
	broadcastEnvChange()

	// Config generation (TOML format)
	if err := writeConfigTOML(cfgPath, selectedAgent, projectPath, version); err != nil {
		nativeError("Config Failed", "Could not write configuration file:\n\n"+err.Error())
		os.Exit(1)
	}

	// ========================================================================
	// STEP 5: VERIFICATION (Gatekeeper)
	// ========================================================================
	dlgs.Info("Verification", "Verifying MCP connection and running diagnostics...\n\nPlease ensure your AI host is running.")

	// 5.1 MCP selftest (protocol-level handshake verification)
	if err := runCmdWithTimeout(binPath, "mcp", "selftest"); err != nil {
		dlgs.Error("MCP Connection Refused",
			"MCP was not detected.\n\n"+
				"Please ensure:\n"+
				"  1. Your AI host ("+selectedAgent+") is running\n"+
				"  2. MCP is enabled in your AI host settings\n"+
				"  3. The MCP server is properly configured\n\n"+
				"Then re-run the installer.\n\n"+
				"Details:\n"+err.Error())
		os.Exit(1)
	}

	// 5.2 Doctor (prerequisite check)
	if err := runCmdWithTimeout(binPath, "--config", cfgPath, "doctor"); err != nil {
		dlgs.Error("Doctor Failed",
			"Prerequisites check did not pass.\n\n"+
				"Please review the output and fix any issues,\n"+
				"then re-run the installer.\n\n"+
				"Details:\n"+err.Error())
		os.Exit(1)
	}

	// ========================================================================
	// SUCCESS - Open Onboarding Dashboard
	// ========================================================================
	dlgs.Info("Success",
		"GRES B2B Governance is installed and verified!\n\n"+
			"Installation details:\n"+
			"  Binary:  "+binPath+"\n"+
			"  Config:  "+cfgPath+"\n"+
			"  Version: "+version+"\n"+
			"  Agent:   "+selectedAgent+"\n"+
			"  Project: "+projectPath+"\n\n"+
			"Opening your onboarding dashboard...")

	openBrowser(OnboardingReadyURL)
}

// ============================================================================
// Step 3: Folder Validation
// ============================================================================

func validateFolder(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("folder does not exist: %w", err)
	}
	if !fi.IsDir() {
		return errors.New("selected path is not a folder")
	}
	f, err := os.Open(p)
	if err != nil {
		return errors.New("folder is not readable")
	}
	f.Close()
	return nil
}

// ============================================================================
// Step 4: Binary Download from GitHub Releases
// ============================================================================

func downloadLatestBinary(destPath string) (version string, err error) {
	// Fetch latest release info
	release, err := fetchLatestRelease()
	if err != nil {
		return "", fmt.Errorf("could not fetch release info: %w", err)
	}

	version = strings.TrimPrefix(release.TagName, "v")

	// Find Windows asset
	downloadURL, err := findWindowsAsset(release, version)
	if err != nil {
		return "", err
	}

	// Download and extract
	if err := downloadAndExtract(downloadURL, destPath); err != nil {
		return "", err
	}

	return version, nil
}

func fetchLatestRelease() (*GitHubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func findWindowsAsset(release *GitHubRelease, version string) (string, error) {
	// Try exact match first
	wantName := fmt.Sprintf("gres-b2b_%s_windows_amd64.zip", version)
	for _, asset := range release.Assets {
		if asset.Name == wantName {
			return asset.BrowserDownloadURL, nil
		}
	}

	// Fallback: any Windows zip
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "windows") && strings.HasSuffix(name, ".zip") {
			return asset.BrowserDownloadURL, nil
		}
	}

	// Fallback: any Windows exe directly
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "windows") && strings.HasSuffix(name, ".exe") {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", errors.New("no Windows binary found in release assets")
}

func downloadAndExtract(downloadURL, destPath string) error {
	client := &http.Client{Timeout: DownloadTimeout}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// If it's a direct exe, just save it
	if strings.HasSuffix(strings.ToLower(downloadURL), ".exe") {
		return downloadToFile(resp.Body, destPath)
	}

	// Otherwise, assume zip and extract
	tmpFile, err := os.CreateTemp("", "gres-b2b-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	return extractBinaryFromZip(tmpPath, destPath)
}

func downloadToFile(r io.Reader, destPath string) error {
	// Write to temp file first, then rename (atomic)
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	os.Remove(destPath)
	return os.Rename(tmpPath, destPath)
}

func extractBinaryFromZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, BinaryName) || filepath.Base(f.Name) == BinaryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			return downloadToFile(rc, destPath)
		}
	}

	return errors.New("binary not found in zip archive")
}

// ============================================================================
// Step 4: PATH Management (HKCU\Environment - no admin required)
// ============================================================================

func addToUserPath(dir string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	cur, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return err
	}

	// Idempotent: check if already in PATH
	if pathContains(cur, dir) {
		return nil
	}

	// Append to PATH
	next := strings.TrimSpace(cur)
	if next != "" && !strings.HasSuffix(next, ";") {
		next += ";"
	}
	next += dir

	return k.SetStringValue("Path", next)
}

func pathContains(pathValue, dir string) bool {
	normDir := strings.TrimRight(strings.ToLower(filepath.Clean(dir)), `\/`)
	for _, p := range strings.Split(pathValue, ";") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n := strings.TrimRight(strings.ToLower(filepath.Clean(p)), `\/`)
		if n == normDir {
			return true
		}
	}
	return false
}

// Broadcast WM_SETTINGCHANGE so new terminals pick up PATH immediately
func broadcastEnvChange() {
	const HWND_BROADCAST = 0xFFFF
	const WM_SETTINGCHANGE = 0x001A
	const SMTO_ABORTIFHUNG = 0x0002

	envPtr, _ := windows.UTF16PtrFromString("Environment")

	user32 := windows.NewLazySystemDLL("user32.dll")
	sendMsgTimeout := user32.NewProc("SendMessageTimeoutW")

	sendMsgTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envPtr)),
		uintptr(SMTO_ABORTIFHUNG),
		uintptr(2000),
		0,
	)
}

// ============================================================================
// Step 4: Config Generation (TOML format)
// ============================================================================

func writeConfigTOML(cfgPath, agentName, projectRoot, version string) error {
	content := fmt.Sprintf(`# GRES B2B Governance Configuration
# Generated by installer at %s

[agent]
name = %q
mcp_enabled = true

[project]
root = %q

[governance]
auto_scan = true

[install]
version = %q
binary_path = %q
`,
		time.Now().Format(time.RFC3339),
		agentName,
		filepath.ToSlash(projectRoot),
		version,
		filepath.ToSlash(filepath.Join(os.Getenv("LOCALAPPDATA"), InstallSubdir, BinaryName)),
	)

	return os.WriteFile(cfgPath, []byte(content), 0o644)
}

// ============================================================================
// Step 5: Command Execution with Timeout
// ============================================================================

func runCmdWithTimeout(binPath string, args ...string) error {
	cmd := exec.Command(binPath, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			output := strings.TrimSpace(out.String())
			if output == "" {
				output = "(no output)"
			}
			return fmt.Errorf("%v\n\nOutput:\n%s", err, output)
		}
		return nil
	case <-time.After(CmdTimeout):
		cmd.Process.Kill()
		output := strings.TrimSpace(out.String())
		if output == "" {
			output = "(no output)"
		}
		return fmt.Errorf("command timed out after %v\n\nOutput:\n%s", CmdTimeout, output)
	}
}

// ============================================================================
// Step 2: Agent Selection (using Entry dialog for reliability)
// ============================================================================

func selectAgent() string {
	// Build numbered options string
	optionsText := "Which AI Agent are you using?\n\n"
	for i, agent := range AgentOptions {
		optionsText += fmt.Sprintf("  %d. %s\n", i+1, agent)
	}
	optionsText += "\nEnter the number (1-5):"

	// Use Entry dialog which is more reliable than List
	input, ok, _ := dlgs.Entry("AI Agent Selection", optionsText, "1")
	if !ok {
		return ""
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Parse the number
	var num int
	if _, err := fmt.Sscanf(input, "%d", &num); err != nil {
		// Try matching by name as fallback
		for _, agent := range AgentOptions {
			if strings.EqualFold(input, agent) || strings.HasPrefix(strings.ToLower(agent), strings.ToLower(input)) {
				return agent
			}
		}
		return ""
	}

	// Validate range
	if num < 1 || num > len(AgentOptions) {
		return ""
	}

	return AgentOptions[num-1]
}

// ============================================================================
// Helpers
// ============================================================================

func nativeError(title, msg string) {
	windows.MessageBox(0,
		windows.StringToUTF16Ptr(msg),
		windows.StringToUTF16Ptr(title),
		windows.MB_ICONERROR)
}

func openBrowser(u string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}
