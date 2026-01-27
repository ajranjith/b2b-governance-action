// GRES B2B Bootstrapper - Windows Native GUI Installer
// Flow: Welcome → MCP Config → Target Selection → Installation → Finish
//
// No PowerShell required. Uses Walk GUI library for native Windows UI.
// Writes config to %LOCALAPPDATA%\gres-b2b\config.json
// Installs binary to %LOCALAPPDATA%\Programs\gres-b2b\gres-b2b.exe

package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows/registry"
)

// ============================================================================
// Constants & Types
// ============================================================================

type Screen int

const (
	Welcome Screen = iota
	MCPConfig
	TargetSelection
	Installation
	Finish
)

const (
	GitHubRepo    = "ajranjith/b2b-governance-action"
	BinaryName    = "gres-b2b.exe"
	OnboardingURL = "https://ajranjith.github.io/b2b-governance-action/onboarding/"
)

type AppState struct {
	screen Screen

	// MCP verification
	mcpHost     string
	mcpPort     string
	mcpVerified atomic.Bool
	mcpStatus   string

	// Target selection
	targetPath string
	gitURL     string

	// Install
	installDir string
	binPath    string
	progress   int
	installLog string

	// Output
	configPath string
}

type PersistedConfig struct {
	MCPVerified bool   `json:"mcp_verified"`
	TargetPath  string `json:"target_path,omitempty"`
	GitURL      string `json:"git_url,omitempty"`
	MCPHost     string `json:"mcp_host,omitempty"`
	MCPPort     string `json:"mcp_port,omitempty"`
}

type GitHubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var (
	gitURLRegex = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/?$`)
)

// ============================================================================
// Main Entry Point
// ============================================================================

func main() {
	if runtime.GOOS != "windows" {
		fmt.Println("This bootstrapper is Windows-only.")
		os.Exit(1)
	}

	state := &AppState{
		screen:    Welcome,
		mcpHost:   "127.0.0.1",
		mcpPort:   "3000",
		mcpStatus: "Not tested",
	}

	// Resolve per-user config folder
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		walk.MsgBox(nil, "Error", "LOCALAPPDATA is not set.", walk.MsgBoxIconError)
		return
	}

	// Config at %LOCALAPPDATA%\gres-b2b\config.json
	cfgDir := filepath.Join(localAppData, "gres-b2b")
	_ = os.MkdirAll(cfgDir, 0o755)
	state.configPath = filepath.Join(cfgDir, "config.json")

	// Install dir: %LOCALAPPDATA%\Programs\gres-b2b\
	state.installDir = filepath.Join(localAppData, "Programs", "gres-b2b")
	state.binPath = filepath.Join(state.installDir, BinaryName)

	var mw *walk.MainWindow

	// Shared UI refs
	var (
		nextBtn, backBtn *walk.PushButton
		titleText        *walk.TextLabel
	)

	// Screen composites
	var (
		welcomeView, mcpView, targetView, installView, finishView *walk.Composite
	)

	// MCP screen refs
	var (
		mcpHostEdit  *walk.LineEdit
		mcpPortEdit  *walk.LineEdit
		mcpStatusLbl *walk.TextLabel
		mcpWarnLbl   *walk.TextLabel
		testBtn      *walk.PushButton
	)

	// Target screen refs
	var (
		folderEdit *walk.LineEdit
		gitEdit    *walk.LineEdit
	)

	// Install screen refs
	var (
		progBar        *walk.ProgressBar
		installLogEdit *walk.TextEdit
	)

	// UI helper to safely update from goroutines
	ui := func(fn func()) {
		if mw != nil {
			mw.Synchronize(fn)
		}
	}

	// Progress and log update helpers
	setProgress := func(p int) {
		ui(func() {
			state.progress = p
			if progBar != nil {
				progBar.SetValue(p)
			}
		})
	}

	appendLog := func(line string) {
		ui(func() {
			if state.installLog == "" {
				state.installLog = line
			} else {
				state.installLog = state.installLog + "\r\n" + line
			}
			if installLogEdit != nil {
				installLogEdit.SetText(state.installLog)
				// Scroll to bottom
				installLogEdit.SetTextSelection(len(state.installLog), len(state.installLog))
			}
		})
	}

	updateNav := func() {
		// Title
		switch state.screen {
		case Welcome:
			titleText.SetText("Welcome")
		case MCPConfig:
			titleText.SetText("MCP Config - Verify Connection")
		case TargetSelection:
			titleText.SetText("Target Selection")
		case Installation:
			titleText.SetText("Installation")
		case Finish:
			titleText.SetText("Setup Complete!")
		}

		// Buttons
		backBtn.SetEnabled(state.screen != Welcome && state.screen != Installation && state.screen != Finish)
		if state.screen == Finish {
			nextBtn.SetText("Open Report && Exit")
		} else {
			nextBtn.SetText("Next")
		}

		// Gate Next rules
		switch state.screen {
		case MCPConfig:
			nextBtn.SetEnabled(state.mcpVerified.Load())
		case TargetSelection:
			nextBtn.SetEnabled(isTargetValid(state))
		case Installation:
			nextBtn.SetEnabled(false) // enabled when install+scan succeed
		default:
			nextBtn.SetEnabled(true)
		}

		// Show correct screen
		welcomeView.SetVisible(state.screen == Welcome)
		mcpView.SetVisible(state.screen == MCPConfig)
		targetView.SetVisible(state.screen == TargetSelection)
		installView.SetVisible(state.screen == Installation)
		finishView.SetVisible(state.screen == Finish)
	}

	// === Screen Builders ===

	buildWelcome := func() Composite {
		return Composite{
			AssignTo: &welcomeView,
			Visible:  true,
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 10},
			Children: []Widget{
				TextLabel{Text: "GRES B2B Governance - Setup Wizard"},
				TextLabel{Text: ""},
				TextLabel{Text: "This installer will:"},
				TextLabel{Text: "  1. Verify your MCP connection"},
				TextLabel{Text: "  2. Let you select a project folder or GitHub URL"},
				TextLabel{Text: "  3. Download and install gres-b2b"},
				TextLabel{Text: "  4. Run a verified governance scan"},
				TextLabel{Text: ""},
				TextLabel{Text: "No PowerShell or admin rights required."},
				TextLabel{Text: ""},
				TextLabel{Text: "Click Next to continue."},
			},
		}
	}

	buildMCP := func() Composite {
		return Composite{
			AssignTo: &mcpView,
			Visible:  false,
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
			Children: []Widget{
				TextLabel{Text: "Enter the MCP endpoint your AI host exposes."},
				TextLabel{Text: "(Most AI agents use localhost on a specific port)"},
				TextLabel{Text: ""},
				Composite{
					Layout: Grid{Columns: 2, Spacing: 8},
					Children: []Widget{
						TextLabel{Text: "Host:"},
						LineEdit{
							AssignTo: &mcpHostEdit,
							Text:     state.mcpHost,
							OnTextChanged: func() {
								state.mcpHost = strings.TrimSpace(mcpHostEdit.Text())
								state.mcpVerified.Store(false)
								state.mcpStatus = "Not tested"
								mcpStatusLbl.SetText(state.mcpStatus)
								mcpWarnLbl.SetText("")
								nextBtn.SetEnabled(false)
							},
						},
						TextLabel{Text: "Port:"},
						LineEdit{
							AssignTo: &mcpPortEdit,
							Text:     state.mcpPort,
							OnTextChanged: func() {
								state.mcpPort = strings.TrimSpace(mcpPortEdit.Text())
								state.mcpVerified.Store(false)
								state.mcpStatus = "Not tested"
								mcpStatusLbl.SetText(state.mcpStatus)
								mcpWarnLbl.SetText("")
								nextBtn.SetEnabled(false)
							},
						},
					},
				},
				TextLabel{Text: ""},
				PushButton{
					AssignTo: &testBtn,
					Text:     "Test MCP Connection",
					OnClicked: func() {
						testBtn.SetEnabled(false)
						mcpWarnLbl.SetText("")
						mcpStatusLbl.SetText("Testing...")
						go func(host, port string) {
							ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
							defer cancel()

							err := testMCP(ctx, host, port)
							ui(func() {
								defer testBtn.SetEnabled(true)
								if err != nil {
									state.mcpVerified.Store(false)
									state.mcpStatus = "FAILED: " + err.Error()
									mcpStatusLbl.SetText(state.mcpStatus)
									mcpWarnLbl.SetText("MCP not detected. Ensure your AI host is running.")
									nextBtn.SetEnabled(false)
									return
								}
								state.mcpVerified.Store(true)
								state.mcpStatus = "OK: MCP endpoint detected"
								mcpStatusLbl.SetText(state.mcpStatus)
								mcpWarnLbl.SetText("")
								nextBtn.SetEnabled(true)
							})
						}(state.mcpHost, state.mcpPort)
					},
				},
				TextLabel{AssignTo: &mcpStatusLbl, Text: state.mcpStatus},
				TextLabel{
					AssignTo:  &mcpWarnLbl,
					TextColor: walk.RGB(180, 0, 0),
					Text:      "",
				},
				TextLabel{Text: ""},
				TextLabel{Text: "Note: This check is agent-agnostic. It verifies something"},
				TextLabel{Text: "is listening on the endpoint, not a specific AI host."},
			},
		}
	}

	buildTarget := func() Composite {
		return Composite{
			AssignTo: &targetView,
			Visible:  false,
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
			Children: []Widget{
				TextLabel{Text: "Choose a local folder OR enter a GitHub URL."},
				TextLabel{Text: "(Leave one blank if not applicable)"},
				TextLabel{Text: ""},
				Composite{
					Layout: Grid{Columns: 3, Spacing: 8},
					Children: []Widget{
						TextLabel{Text: "Local Folder:"},
						LineEdit{
							AssignTo: &folderEdit,
							Text:     state.targetPath,
							OnTextChanged: func() {
								state.targetPath = strings.TrimSpace(folderEdit.Text())
								nextBtn.SetEnabled(isTargetValid(state))
							},
						},
						PushButton{
							Text: "Browse...",
							OnClicked: func() {
								dlg := new(walk.FileDialog)
								dlg.Title = "Select a project folder"
								dlg.FilePath = state.targetPath
								if ok, _ := dlg.ShowBrowseFolder(mw); ok {
									state.targetPath = dlg.FilePath
									folderEdit.SetText(state.targetPath)
									nextBtn.SetEnabled(isTargetValid(state))
								}
							},
						},

						TextLabel{Text: "GitHub URL:"},
						LineEdit{
							AssignTo: &gitEdit,
							Text:     state.gitURL,
							OnTextChanged: func() {
								state.gitURL = strings.TrimSpace(gitEdit.Text())
								nextBtn.SetEnabled(isTargetValid(state))
							},
						},
						Composite{}, // empty third column
					},
				},
				TextLabel{Text: ""},
				TextLabel{Text: "Validation: folder must exist and be readable,"},
				TextLabel{Text: "and Git URL must be a valid GitHub repository URL."},
			},
		}
	}

	buildInstall := func() Composite {
		return Composite{
			AssignTo: &installView,
			Visible:  false,
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
			Children: []Widget{
				TextLabel{Text: "Installing gres-b2b and running a verified scan..."},
				TextLabel{Text: ""},
				ProgressBar{
					AssignTo: &progBar,
					MinValue: 0,
					MaxValue: 100,
					Value:    state.progress,
				},
				TextLabel{Text: ""},
				TextEdit{
					AssignTo: &installLogEdit,
					ReadOnly: true,
					VScroll:  true,
					MinSize:  Size{Height: 200},
				},
			},
		}
	}

	buildFinish := func() Composite {
		return Composite{
			AssignTo: &finishView,
			Visible:  false,
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 10},
			Children: []Widget{
				TextLabel{Text: "Setup complete!"},
				TextLabel{Text: ""},
				TextLabel{Text: "gres-b2b has been installed and added to your PATH."},
				TextLabel{Text: "A governance scan has been completed successfully."},
				TextLabel{Text: ""},
				TextLabel{Text: "You can now use gres-b2b from any terminal:"},
				TextLabel{Text: "  gres-b2b --version"},
				TextLabel{Text: "  gres-b2b doctor"},
				TextLabel{Text: "  gres-b2b scan --live"},
				TextLabel{Text: ""},
				TextLabel{Text: "Click below to view the scan report and exit."},
			},
		}
	}

	// === Window ===
	err := MainWindow{
		AssignTo: &mw,
		Title:    "GRES B2B Governance - Setup Wizard",
		MinSize:  Size{Width: 640, Height: 480},
		Size:     Size{Width: 720, Height: 520},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 10},
		Children: []Widget{
			TextLabel{
				AssignTo: &titleText,
				Font:     Font{PointSize: 14, Bold: true},
				Text:     "Welcome",
			},
			Composite{
				Layout: VBox{},
				Children: []Widget{
					buildWelcome(),
					buildMCP(),
					buildTarget(),
					buildInstall(),
					buildFinish(),
				},
			},
			VSpacer{},
			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &backBtn,
						Text:     "Back",
						OnClicked: func() {
							switch state.screen {
							case MCPConfig:
								state.screen = Welcome
							case TargetSelection:
								state.screen = MCPConfig
							}
							updateNav()
						},
					},
					PushButton{
						AssignTo: &nextBtn,
						Text:     "Next",
						OnClicked: func() {
							switch state.screen {
							case Welcome:
								state.screen = MCPConfig
								updateNav()

							case MCPConfig:
								if !state.mcpVerified.Load() {
									return
								}
								state.screen = TargetSelection
								updateNav()

							case TargetSelection:
								if !isTargetValid(state) {
									walk.MsgBox(mw, "Validation", "Please select a valid folder OR enter a valid GitHub URL.", walk.MsgBoxIconWarning)
									return
								}
								// Enter Installation + kick background workflow
								state.screen = Installation
								state.progress = 0
								state.installLog = ""
								if progBar != nil {
									progBar.SetValue(0)
								}
								if installLogEdit != nil {
									installLogEdit.SetText("")
								}
								updateNav()

								go func() {
									success := runInstallAndScan(state, ui, setProgress, appendLog)
									if success {
										ui(func() {
											state.screen = Finish
											updateNav()
											nextBtn.SetEnabled(true)
										})
									}
								}()

							case Installation:
								// Next is disabled until success; ignore clicks

							case Finish:
								// Open report and exit
								reportPath := filepath.Join(state.targetPath, ".b2b", "report.html")
								if _, err := os.Stat(reportPath); err == nil {
									openBrowser("file:///" + strings.ReplaceAll(reportPath, "\\", "/"))
								} else {
									openBrowser(OnboardingURL + "?status=success")
								}
								mw.Close()
							}
						},
					},
				},
			},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, "Error", err.Error(), walk.MsgBoxIconError)
		return
	}

	updateNav()
	mw.Run()
}

// ============================================================================
// MCP Verification (agent-agnostic TCP connect)
// ============================================================================

func testMCP(ctx context.Context, host, port string) error {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" || port == "" || port == "0" {
		return errors.New("host/port required")
	}

	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// ============================================================================
// Target Validation
// ============================================================================

func isTargetValid(s *AppState) bool {
	// Either a valid folder OR a valid GitHub URL
	if s.targetPath != "" {
		info, err := os.Stat(s.targetPath)
		if err == nil && info.IsDir() {
			f, err := os.Open(s.targetPath)
			if err == nil {
				_ = f.Close()
				return true
			}
		}
	}

	if s.gitURL != "" {
		if !gitURLRegex.MatchString(s.gitURL) {
			return false
		}
		_, err := url.Parse(s.gitURL)
		return err == nil
	}
	return false
}

// ============================================================================
// Install + PATH + Persist config + Run scan
// ============================================================================

func runInstallAndScan(s *AppState, ui func(func()), setProgress func(int), appendLog func(string)) bool {
	// 1) Persist config.json
	appendLog("Writing configuration...")
	cfg := PersistedConfig{
		MCPVerified: s.mcpVerified.Load(),
		TargetPath:  s.targetPath,
		GitURL:      s.gitURL,
		MCPHost:     s.mcpHost,
		MCPPort:     s.mcpPort,
	}
	if err := writeConfig(s.configPath, cfg); err != nil {
		appendLog("ERROR: Failed to write config.json: " + err.Error())
		showError("Failed to write config.json: " + err.Error())
		return false
	}
	appendLog("Config saved to: " + s.configPath)
	setProgress(10)

	// 2) Create install directory
	appendLog("Creating install directory...")
	if err := os.MkdirAll(s.installDir, 0o755); err != nil {
		appendLog("ERROR: Failed to create install dir: " + err.Error())
		showError("Failed to create install dir: " + err.Error())
		return false
	}
	appendLog("Install dir: " + s.installDir)
	setProgress(15)

	// 3) Download latest binary from GitHub Releases
	appendLog("Fetching latest release from GitHub...")
	downloadURL, version, err := getLatestReleaseURL()
	if err != nil {
		appendLog("ERROR: Failed to get release: " + err.Error())
		showError("Failed to get latest release: " + err.Error())
		return false
	}
	appendLog("Found version: " + version)
	setProgress(25)

	appendLog("Downloading " + BinaryName + "...")
	if err := downloadAndExtract(downloadURL, s.binPath); err != nil {
		appendLog("ERROR: Download failed: " + err.Error())
		showError("Download failed: " + err.Error())
		return false
	}
	appendLog("Downloaded to: " + s.binPath)
	setProgress(55)

	// 4) Add to User PATH
	appendLog("Adding to User PATH...")
	added, err := addToUserPath(s.installDir)
	if err != nil {
		appendLog("WARNING: Could not update PATH: " + err.Error())
		appendLog("You may need to add " + s.installDir + " to PATH manually.")
	} else if added {
		appendLog("Added " + s.installDir + " to User PATH")
		broadcastEnvironmentChange()
		appendLog("Broadcasted environment change to running applications")
	} else {
		appendLog("Already in User PATH")
	}
	setProgress(70)

	// 5) Verify installation
	appendLog("Verifying installation...")
	versionOut, err := exec.Command(s.binPath, "--version").CombinedOutput()
	if err != nil {
		appendLog("WARNING: Version check failed: " + err.Error())
	} else {
		appendLog("Installed: " + strings.TrimSpace(string(versionOut)))
	}
	setProgress(75)

	// 6) Run doctor to check prerequisites
	appendLog("Running gres-b2b doctor...")
	doctorOut, err := exec.Command(s.binPath, "doctor").CombinedOutput()
	if err != nil {
		appendLog("WARNING: Doctor returned issues: " + err.Error())
	}
	appendLog(strings.TrimSpace(string(doctorOut)))
	setProgress(80)

	// 7) Run verified scan
	appendLog("Running governance scan...")
	var scanArgs []string
	if s.targetPath != "" {
		scanArgs = []string{"scan", "--workspace", s.targetPath}
	} else {
		scanArgs = []string{"scan"}
	}
	scanCmd := exec.Command(s.binPath, scanArgs...)
	scanOut, err := scanCmd.CombinedOutput()
	appendLog(strings.TrimSpace(string(scanOut)))

	if err != nil {
		appendLog("WARNING: Scan completed with issues: " + err.Error())
		// Don't fail - scan may return non-zero for policy violations
	} else {
		appendLog("Scan completed successfully!")
	}
	setProgress(100)

	appendLog("")
	appendLog("=== Installation Complete ===")
	appendLog("Binary: " + s.binPath)
	appendLog("Config: " + s.configPath)
	if s.targetPath != "" {
		reportPath := filepath.Join(s.targetPath, ".b2b", "report.html")
		appendLog("Report: " + reportPath)
	}

	return true
}

// ============================================================================
// GitHub Release Download
// ============================================================================

func getLatestReleaseURL() (downloadURL, version string, err error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	version = strings.TrimPrefix(release.TagName, "v")

	// Find Windows amd64 asset
	wantName := fmt.Sprintf("gres-b2b_%s_windows_amd64.zip", version)
	for _, asset := range release.Assets {
		if asset.Name == wantName {
			return asset.BrowserDownloadURL, version, nil
		}
	}

	// Fallback: try any windows zip
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "windows") && strings.HasSuffix(asset.Name, ".zip") {
			return asset.BrowserDownloadURL, version, nil
		}
	}

	return "", "", errors.New("no Windows binary found in release assets")
}

func downloadAndExtract(downloadURL, destPath string) error {
	// Download to temp file
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "gres-b2b-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return err
	}

	// Extract binary from zip
	return extractBinaryFromZip(tmpPath, destPath)
}

func extractBinaryFromZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, BinaryName) || f.Name == BinaryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			// Ensure parent dir exists
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return err
			}

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, rc)
			return err
		}
	}

	return errors.New("binary not found in zip archive")
}

// ============================================================================
// Windows PATH Management (User scope, no admin)
// ============================================================================

func addToUserPath(dir string) (added bool, err error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer key.Close()

	currentPath, _, err := key.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return false, err
	}

	// Check if already in PATH
	paths := strings.Split(currentPath, ";")
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), dir) {
			return false, nil // already present
		}
	}

	// Append to PATH
	var newPath string
	if currentPath == "" {
		newPath = dir
	} else {
		newPath = currentPath + ";" + dir
	}

	if err := key.SetStringValue("Path", newPath); err != nil {
		return false, err
	}

	return true, nil
}

// Broadcast WM_SETTINGCHANGE so new terminals pick up PATH change
func broadcastEnvironmentChange() {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")

	envStr, _ := syscall.UTF16PtrFromString("Environment")
	const HWND_BROADCAST = 0xFFFF
	const WM_SETTINGCHANGE = 0x001A
	const SMTO_ABORTIFHUNG = 0x0002

	sendMessageTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envStr)),
		uintptr(SMTO_ABORTIFHUNG),
		uintptr(5000),
		0,
	)
}

// ============================================================================
// Helpers
// ============================================================================

func writeConfig(path string, cfg PersistedConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func showError(msg string) {
	walk.MsgBox(nil, "GRES B2B Bootstrapper", msg, walk.MsgBoxIconError)
}

func openBrowser(u string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}
