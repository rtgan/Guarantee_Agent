package config

// Guardrails 是运行期间强制执行的单用例安全阈值。
type Guardrails struct {
	MaxToolCallsPerSpec  int `json:"maxToolCallsPerSpec" yaml:"maxToolCallsPerSpec"`
	MaxConsecutiveErrors int `json:"maxConsecutiveErrors" yaml:"maxConsecutiveErrors"`
	MaxRetriesPerStep    int `json:"maxRetriesPerStep" yaml:"maxRetriesPerStep"`
}

// PlanGuardrails 约束一次 plan/explore/generate 会话。
type PlanGuardrails struct {
	MaxAgentTurnsPerRun int `json:"maxAgentTurnsPerRun" yaml:"maxAgentTurnsPerRun"`
	MaxSnapshotsPerRun  int `json:"maxSnapshotsPerRun" yaml:"maxSnapshotsPerRun"`
	MaxPagesPerRun      int `json:"maxPagesPerRun" yaml:"maxPagesPerRun"`
	MaxTokenPerRun      int `json:"maxTokenPerRun" yaml:"maxTokenPerRun"`
}

// PlanConfig 配置 plan/explore/generate 命令。
type PlanConfig struct {
	BaseURL         string         `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	MaxDepth        int            `json:"maxDepth" yaml:"maxDepth"`
	MaxPages        int            `json:"maxPages" yaml:"maxPages"`
	IncludePatterns []string       `json:"includePatterns" yaml:"includePatterns"`
	ExcludePatterns []string       `json:"excludePatterns" yaml:"excludePatterns"`
	ExploreScope    string         `json:"exploreScope" yaml:"exploreScope"` // site | focused | single_page
	TestTypes       []string       `json:"testTypes" yaml:"testTypes"`
	Guardrails      PlanGuardrails `json:"guardrails" yaml:"guardrails"`
	LoginStepsSpec  string         `json:"loginStepsSpec,omitempty" yaml:"loginStepsSpec,omitempty"`
}

// ModelConfig 配置 Eino chat 模型。默认 provider 为 "eino-script";
// 要使用真实 LLM,把 provider 设为某个 eino-ext provider(如 ark)并提供
// BaseURL/APIKey/Model。
type ModelConfig struct {
	Provider    string   `json:"provider" yaml:"provider"`
	Model       string   `json:"model" yaml:"model"`
	BaseURL     string   `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	APIKeyEnv   string   `json:"apiKeyEnv" yaml:"apiKeyEnv"`
	APIKey      string   `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	MaxTurns    int      `json:"maxTurns" yaml:"maxTurns"`
	MaxTokens   int      `json:"maxTokens" yaml:"maxTokens"`
	Temperature *float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
}

// AutoqaConfig 是根配置对象(autoqa.config.json|yaml)。
type AutoqaConfig struct {
	SchemaVersion int         `json:"schemaVersion" yaml:"schemaVersion"`
	Guardrails    Guardrails  `json:"guardrails" yaml:"guardrails"`
	ExportDir     string      `json:"exportDir" yaml:"exportDir"`
	Model         ModelConfig `json:"model" yaml:"model"`
	Plan          PlanConfig  `json:"plan" yaml:"plan"`
}
