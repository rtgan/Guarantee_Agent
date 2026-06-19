package config

// DefaultExportDir 是成功用例导出的默认目录。
const DefaultExportDir = "tests/autoqa"

// DefaultGuardrails 是运行护栏默认值,与原项目保持一致。
var DefaultGuardrails = Guardrails{
	MaxToolCallsPerSpec:  200,
	MaxConsecutiveErrors: 8,
	MaxRetriesPerStep:    5,
}

// DefaultPlanGuardrails 是 plan/explore/generate 护栏默认值。
var DefaultPlanGuardrails = PlanGuardrails{
	MaxAgentTurnsPerRun: 1000,
	MaxSnapshotsPerRun:  500,
	MaxPagesPerRun:      100,
	MaxTokenPerRun:      5000000,
}

// DefaultPlanConfig 是 plan 配置默认值。
var DefaultPlanConfig = PlanConfig{
	MaxDepth:        3,
	MaxPages:        50,
	IncludePatterns: []string{},
	ExcludePatterns: []string{},
	ExploreScope:    "site",
	TestTypes:       []string{"functional", "form", "navigation", "responsive", "boundary", "security"},
	Guardrails:      DefaultPlanGuardrails,
}

// DefaultModelConfig 是模型配置默认值,使用豆包 Ark Responses API。
// API key 从 ARK_API_KEY 环境变量读取(.env 注入),不要写进配置文件。
var DefaultModelConfig = ModelConfig{
	Provider:  "ark",
	Model:     "deepseek-v4-flash-260425",
	APIKeyEnv: "ARK_API_KEY",
	MaxTurns:  50,
	MaxTokens: 4096,
}

// DefaultConfig 返回完整的默认配置。
func DefaultConfig() AutoqaConfig {
	return AutoqaConfig{
		SchemaVersion: 1,
		Guardrails:    DefaultGuardrails,
		ExportDir:     DefaultExportDir,
		Model:         DefaultModelConfig,
		Plan:          DefaultPlanConfig,
	}
}
