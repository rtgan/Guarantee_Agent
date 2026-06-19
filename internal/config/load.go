package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load 从 cwd 读取配置文件(json 或 yaml),以默认值为起点叠加文件值。
// 若不存在配置文件则返回默认值。合并后的配置会先校验再返回。
func Load(cwd string) (AutoqaConfig, error) {
	cfg := DefaultConfig()
	path, ok := findConfig(cwd)
	if !ok {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	var fileCfg AutoqaConfig
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &fileCfg)
	default:
		err = json.Unmarshal(data, &fileCfg)
	}
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	merge(&cfg, fileCfg)
	return cfg, Validate(cfg)
}

// findConfig 在 cwd 中查找第一个存在的配置文件(json > yaml > yml)。
func findConfig(cwd string) (string, bool) {
	for _, name := range []string{"autoqa.config.json", "autoqa.config.yaml", "autoqa.config.yml"} {
		path := filepath.Join(cwd, name)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, true
		}
	}
	return "", false
}

// merge 把 src 中非零/非 nil 的值叠加到 dst 上。文件中的零值会被忽略,
// 以免文件缺省值把内置默认值清掉。
func merge(dst *AutoqaConfig, src AutoqaConfig) {
	if src.SchemaVersion != 0 {
		dst.SchemaVersion = src.SchemaVersion
	}
	if src.ExportDir != "" {
		dst.ExportDir = src.ExportDir
	}
	if src.Guardrails.MaxToolCallsPerSpec != 0 {
		dst.Guardrails.MaxToolCallsPerSpec = src.Guardrails.MaxToolCallsPerSpec
	}
	if src.Guardrails.MaxConsecutiveErrors != 0 {
		dst.Guardrails.MaxConsecutiveErrors = src.Guardrails.MaxConsecutiveErrors
	}
	if src.Guardrails.MaxRetriesPerStep != 0 {
		dst.Guardrails.MaxRetriesPerStep = src.Guardrails.MaxRetriesPerStep
	}
	if src.Model.Provider != "" {
		dst.Model.Provider = src.Model.Provider
	}
	if src.Model.Model != "" {
		dst.Model.Model = src.Model.Model
	}
	if src.Model.BaseURL != "" {
		dst.Model.BaseURL = src.Model.BaseURL
	}
	if src.Model.APIKeyEnv != "" {
		dst.Model.APIKeyEnv = src.Model.APIKeyEnv
	}
	if src.Model.APIKey != "" {
		dst.Model.APIKey = src.Model.APIKey
	}
	if src.Model.MaxTurns != 0 {
		dst.Model.MaxTurns = src.Model.MaxTurns
	}
	if src.Model.MaxTokens != 0 {
		dst.Model.MaxTokens = src.Model.MaxTokens
	}
	if src.Model.Temperature != nil {
		dst.Model.Temperature = src.Model.Temperature
	}
	if src.Plan.BaseURL != "" {
		dst.Plan.BaseURL = src.Plan.BaseURL
	}
	if src.Plan.MaxDepth != 0 {
		dst.Plan.MaxDepth = src.Plan.MaxDepth
	}
	if src.Plan.MaxPages != 0 {
		dst.Plan.MaxPages = src.Plan.MaxPages
	}
	if src.Plan.IncludePatterns != nil {
		dst.Plan.IncludePatterns = src.Plan.IncludePatterns
	}
	if src.Plan.ExcludePatterns != nil {
		dst.Plan.ExcludePatterns = src.Plan.ExcludePatterns
	}
	if src.Plan.ExploreScope != "" {
		dst.Plan.ExploreScope = src.Plan.ExploreScope
	}
	if src.Plan.TestTypes != nil {
		dst.Plan.TestTypes = src.Plan.TestTypes
	}
	if src.Plan.LoginStepsSpec != "" {
		dst.Plan.LoginStepsSpec = src.Plan.LoginStepsSpec
	}
	if src.Plan.Guardrails.MaxAgentTurnsPerRun != 0 {
		dst.Plan.Guardrails.MaxAgentTurnsPerRun = src.Plan.Guardrails.MaxAgentTurnsPerRun
	}
	if src.Plan.Guardrails.MaxSnapshotsPerRun != 0 {
		dst.Plan.Guardrails.MaxSnapshotsPerRun = src.Plan.Guardrails.MaxSnapshotsPerRun
	}
	if src.Plan.Guardrails.MaxPagesPerRun != 0 {
		dst.Plan.Guardrails.MaxPagesPerRun = src.Plan.Guardrails.MaxPagesPerRun
	}
	if src.Plan.Guardrails.MaxTokenPerRun != 0 {
		dst.Plan.Guardrails.MaxTokenPerRun = src.Plan.Guardrails.MaxTokenPerRun
	}
}

// Validate 在合并后校验必需字段和取值范围。
func Validate(cfg AutoqaConfig) error {
	if cfg.SchemaVersion < 1 {
		return errors.New("schemaVersion must be >= 1")
	}
	if cfg.ExportDir == "" {
		return errors.New("exportDir is required")
	}
	if cfg.Guardrails.MaxToolCallsPerSpec <= 0 || cfg.Guardrails.MaxConsecutiveErrors <= 0 || cfg.Guardrails.MaxRetriesPerStep <= 0 {
		return errors.New("guardrails values must be positive")
	}
	switch cfg.Plan.ExploreScope {
	case "site", "focused", "single_page":
	case "":
		return errors.New("plan.exploreScope is required")
	default:
		return fmt.Errorf("unsupported plan.exploreScope %q", cfg.Plan.ExploreScope)
	}
	return nil
}

// WriteDefault 把默认配置写为 JSON 文件。拒绝覆盖已存在的文件。
func WriteDefault(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	data, err := json.MarshalIndent(DefaultConfig(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
