package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the compiled-in configuration with optional overrides.
type Config struct {
	SchemaVersion string        `json:"schemaVersion"`
	App           AppConfig     `json:"app"`
	Paths         PathsConfig   `json:"paths"`
	Run           RunConfig     `json:"run"`
	Reports       ReportsConfig `json:"reports"`
	Install       InstallConfig `json:"install"`
	Logging       LoggingConfig `json:"logging"`
	Scan          ScanConfig    `json:"scan"`
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

type ScanConfig struct {
	UIPaths                          []string `json:"ui_paths"`
	BFFPaths                         []string `json:"bff_paths"`
	WrapperSignatures                []string `json:"wrapper_signatures"`
	PermissionSignatures             []string `json:"permission_signatures"`
	AuditWrapperSignatures           []string `json:"audit_wrapper_signatures"`
	DealerUIPaths                    []string `json:"dealer_ui_paths"`
	AdminUIPaths                     []string `json:"admin_ui_paths"`
	DealerBFFPaths                   []string `json:"dealer_bff_paths"`
	AdminBFFPaths                    []string `json:"admin_bff_paths"`
	ContractSpecGlobs                []string `json:"contract_spec_globs"`
	ContractVersionFile              string   `json:"contract_version_file"`
	ContractVersionRequiredFields    []string `json:"contract_version_required_fields"`
	CompatibilityAllowedValues       []string `json:"compatibility_allowed_values"`
	UIRegistryPath                   string   `json:"ui_registry_path"`
	UIRegistryEnforcement            string   `json:"ui_registry_enforcement"`
	UIRegistryCriticalPatterns       []string `json:"ui_registry_critical_patterns"`
	UIInventoryFramework             string   `json:"ui_inventory_framework"`
	UIInventoryGlobs                 []string `json:"ui_inventory_globs"`
	LLIDFieldNames                   []string `json:"llid_field_names"`
	DealerContractTags               []string `json:"dealer_contract_tags"`
	DealerContractPaths              []string `json:"dealer_contract_paths"`
	ExternalGatewayForbiddenPatterns []string `json:"external_gateway_forbidden_patterns"`
	ExternalGatewayForbiddenPaths    []string `json:"external_gateway_forbidden_paths"`
	HistoryMaxSnapshots              int      `json:"history_max_snapshots"`
	HistoryKeepDays                  int      `json:"history_keep_days"`
	ShadowFailOnMismatch             bool     `json:"shadow_fail_on_mismatch"`
	SemanticProtectedPaths           []string `json:"semantic_protected_paths"`
	ApiIDSignatures                  []string `json:"api_id_signatures"`
	ApiRouteFramework                string   `json:"api_route_framework"`
	ApiRouteEnforcement              string   `json:"api_route_enforcement"`
	UILLIDDisplayEnforcement         string   `json:"ui_llid_display_enforcement"`
	UILLIDDisplaySignatures          []string `json:"ui_llid_display_signatures"`
	DealerEntities                   []string `json:"dealer_entities"`
}

type Flags struct {
	ConfigPath string
}

// Default returns the compiled-in defaults.
func Default() Config {
	return Config{
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
		Scan: ScanConfig{
			UIPaths: []string{
				"ui/",
				"apps/",
				"frontend/",
				"src/app/",
				"src/pages/",
			},
			BFFPaths: []string{
				"bff/",
				"src/bff/",
				"apps/bff/",
			},
			WrapperSignatures: []string{
				"withEnvelope(",
				"withAuth(",
				"withDealerEnvelope(",
				"withAdminEnvelope(",
			},
			PermissionSignatures: []string{
				"policy.check(",
				"authorize(",
				"permission.lookup(",
			},
			AuditWrapperSignatures: []string{
				"withAudit(",
				"audit(",
				"AUDIT(",
				"withMutationAudit(",
			},
			DealerUIPaths: []string{
				"ui/dealer/",
				"apps/dealer/",
				"frontend/dealer/",
			},
			AdminUIPaths: []string{
				"ui/admin/",
				"apps/admin/",
				"frontend/admin/",
			},
			DealerBFFPaths: []string{
				"bff/dealer/",
				"src/bff/dealer/",
			},
			AdminBFFPaths: []string{
				"bff/admin/",
				"src/bff/admin/",
			},
			ContractSpecGlobs: []string{
				"contracts/openapi.yaml",
				"contracts/openapi.yml",
				"contracts/openapi.json",
				"contracts/zod.ts",
				"contracts/schema.json",
			},
			ContractVersionFile: "contracts/version.json",
			ContractVersionRequiredFields: []string{
				"module",
				"version",
				"compatibility",
			},
			CompatibilityAllowedValues: []string{
				"backward",
				"forward",
				"strict",
			},
			UIRegistryPath:        "ui/registry.json",
			UIRegistryEnforcement: "fail",
			UIRegistryCriticalPatterns: []string{
				"checkout",
				"order",
				"payment",
				"pricing",
				"account",
			},
			UIInventoryFramework: "nextjs",
			UIInventoryGlobs: []string{
				"**/app/**/page.*",
				"**/pages/**/*.*",
				"**/components/**/*.*",
			},
			LLIDFieldNames: []string{
				"llid",
				"LLID",
				"trace.llid",
				"trace.LLID",
			},
			DealerContractTags:  []string{"dealer"},
			DealerContractPaths: []string{"contracts/dealer/"},
			ExternalGatewayForbiddenPatterns: []string{
				"HMAC",
				"apiKey",
				"x-api-key",
				"public gateway",
				"apikey",
				"key rotation",
				"PUBLIC_API_KEY",
				"HMAC_SECRET",
				"EXTERNAL_GATEWAY_",
			},
			ExternalGatewayForbiddenPaths: []string{
				"/public-api",
				"/external",
				"/partner-api",
			},
			HistoryMaxSnapshots:  50,
			HistoryKeepDays:      14,
			ShadowFailOnMismatch: true,
			SemanticProtectedPaths: []string{
				"domain/",
				"logic/",
				"core/",
			},
			ApiIDSignatures: []string{
				"API_IDS.",
				"apiId:",
				"API_ID:",
			},
			ApiRouteFramework:        "generic",
			ApiRouteEnforcement:      "fail",
			UILLIDDisplayEnforcement: "fail",
			UILLIDDisplaySignatures: []string{
				"TraceBadge",
				"LLIDBadge",
				"data-llid",
				"llid:",
				"trace.llid",
				"traceId",
				"LLID:",
			},
			DealerEntities: []string{
				"order",
				"price",
				"account",
				"invoice",
			},
		},
	}
}

// Load reads a JSON config from disk.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Resolve applies defaults and optional overrides, then validates.
func Resolve(flags Flags) (Config, string, []string, error) {
	cfg := Default()
	var cfgPath string
	var warnings []string

	if flags.ConfigPath != "" {
		loaded, err := Load(flags.ConfigPath)
		if err != nil {
			return Config{}, "", nil, err
		}
		mergeConfigDefaults(&loaded, &cfg)
		cfg = loaded
		cfgPath = flags.ConfigPath
	}

	if cfg.SchemaVersion == "" {
		cfg.SchemaVersion = "1.0"
	}
	if cfg.Install.DuplicateDetection.Severity != "" && cfg.Install.DuplicateDetection.Severity != "warning" {
		cfg.Install.DuplicateDetection.Severity = "warning"
		warnings = append(warnings, "duplicate detection severity forced to \"warning\"")
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, "", nil, err
	}

	return cfg, cfgPath, warnings, nil
}

// Validate checks the resolved configuration for consistency.
func (c *Config) Validate() error {
	if c.SchemaVersion != "1.0" {
		return fmt.Errorf("unsupported schemaVersion: %s (expected 1.0)", c.SchemaVersion)
	}
	return nil
}

func mergeConfigDefaults(cfg *Config, defaults *Config) {
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
	if len(cfg.Scan.UIPaths) == 0 {
		cfg.Scan.UIPaths = defaults.Scan.UIPaths
	}
	if len(cfg.Scan.BFFPaths) == 0 {
		cfg.Scan.BFFPaths = defaults.Scan.BFFPaths
	}
	if len(cfg.Scan.WrapperSignatures) == 0 {
		cfg.Scan.WrapperSignatures = defaults.Scan.WrapperSignatures
	}
	if len(cfg.Scan.PermissionSignatures) == 0 {
		cfg.Scan.PermissionSignatures = defaults.Scan.PermissionSignatures
	}
	if len(cfg.Scan.AuditWrapperSignatures) == 0 {
		cfg.Scan.AuditWrapperSignatures = defaults.Scan.AuditWrapperSignatures
	}
	if len(cfg.Scan.DealerUIPaths) == 0 {
		cfg.Scan.DealerUIPaths = defaults.Scan.DealerUIPaths
	}
	if len(cfg.Scan.AdminUIPaths) == 0 {
		cfg.Scan.AdminUIPaths = defaults.Scan.AdminUIPaths
	}
	if len(cfg.Scan.DealerBFFPaths) == 0 {
		cfg.Scan.DealerBFFPaths = defaults.Scan.DealerBFFPaths
	}
	if len(cfg.Scan.AdminBFFPaths) == 0 {
		cfg.Scan.AdminBFFPaths = defaults.Scan.AdminBFFPaths
	}
	if len(cfg.Scan.ContractSpecGlobs) == 0 {
		cfg.Scan.ContractSpecGlobs = defaults.Scan.ContractSpecGlobs
	}
	if cfg.Scan.ContractVersionFile == "" {
		cfg.Scan.ContractVersionFile = defaults.Scan.ContractVersionFile
	}
	if len(cfg.Scan.ContractVersionRequiredFields) == 0 {
		cfg.Scan.ContractVersionRequiredFields = defaults.Scan.ContractVersionRequiredFields
	}
	if len(cfg.Scan.CompatibilityAllowedValues) == 0 {
		cfg.Scan.CompatibilityAllowedValues = defaults.Scan.CompatibilityAllowedValues
	}
	if cfg.Scan.UIRegistryPath == "" {
		cfg.Scan.UIRegistryPath = defaults.Scan.UIRegistryPath
	}
	if cfg.Scan.UIRegistryEnforcement == "" {
		cfg.Scan.UIRegistryEnforcement = defaults.Scan.UIRegistryEnforcement
	}
	if len(cfg.Scan.UIRegistryCriticalPatterns) == 0 {
		cfg.Scan.UIRegistryCriticalPatterns = defaults.Scan.UIRegistryCriticalPatterns
	}
	if cfg.Scan.UIInventoryFramework == "" {
		cfg.Scan.UIInventoryFramework = defaults.Scan.UIInventoryFramework
	}
	if len(cfg.Scan.UIInventoryGlobs) == 0 {
		cfg.Scan.UIInventoryGlobs = defaults.Scan.UIInventoryGlobs
	}
	if len(cfg.Scan.LLIDFieldNames) == 0 {
		cfg.Scan.LLIDFieldNames = defaults.Scan.LLIDFieldNames
	}
	if len(cfg.Scan.DealerContractTags) == 0 {
		cfg.Scan.DealerContractTags = defaults.Scan.DealerContractTags
	}
	if len(cfg.Scan.DealerContractPaths) == 0 {
		cfg.Scan.DealerContractPaths = defaults.Scan.DealerContractPaths
	}
	if len(cfg.Scan.ExternalGatewayForbiddenPatterns) == 0 {
		cfg.Scan.ExternalGatewayForbiddenPatterns = defaults.Scan.ExternalGatewayForbiddenPatterns
	}
	if len(cfg.Scan.ExternalGatewayForbiddenPaths) == 0 {
		cfg.Scan.ExternalGatewayForbiddenPaths = defaults.Scan.ExternalGatewayForbiddenPaths
	}
	if cfg.Scan.HistoryMaxSnapshots == 0 {
		cfg.Scan.HistoryMaxSnapshots = defaults.Scan.HistoryMaxSnapshots
	}
	if cfg.Scan.HistoryKeepDays == 0 {
		cfg.Scan.HistoryKeepDays = defaults.Scan.HistoryKeepDays
	}
	if !cfg.Scan.ShadowFailOnMismatch {
		cfg.Scan.ShadowFailOnMismatch = defaults.Scan.ShadowFailOnMismatch
	}
	if len(cfg.Scan.SemanticProtectedPaths) == 0 {
		cfg.Scan.SemanticProtectedPaths = defaults.Scan.SemanticProtectedPaths
	}
	if len(cfg.Scan.ApiIDSignatures) == 0 {
		cfg.Scan.ApiIDSignatures = defaults.Scan.ApiIDSignatures
	}
	if cfg.Scan.ApiRouteFramework == "" {
		cfg.Scan.ApiRouteFramework = defaults.Scan.ApiRouteFramework
	}
	if cfg.Scan.ApiRouteEnforcement == "" {
		cfg.Scan.ApiRouteEnforcement = defaults.Scan.ApiRouteEnforcement
	}
	if cfg.Scan.UILLIDDisplayEnforcement == "" {
		cfg.Scan.UILLIDDisplayEnforcement = defaults.Scan.UILLIDDisplayEnforcement
	}
	if len(cfg.Scan.UILLIDDisplaySignatures) == 0 {
		cfg.Scan.UILLIDDisplaySignatures = defaults.Scan.UILLIDDisplaySignatures
	}
	if len(cfg.Scan.DealerEntities) == 0 {
		cfg.Scan.DealerEntities = defaults.Scan.DealerEntities
	}
}
