package agent

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"guarantee-agent/internal/config"
)

// NewChatModel 是 Eino 模型工厂的边界。
//
// provider 取值:
//   - "eino-script":返回确定性 ScriptModel,无网络依赖,用于离线/测试;
//   - "ark"(默认 LLM provider):用 eino-ext 的 Ark Responses API 接入豆包,
//     凭据从 cfg.APIKey 或 cfg.APIKeyEnv 指向的环境变量读取,模型名取 cfg.Model;
//   - 其他:报错,便于显式失败而非静默回退。
//
// 切换到别的 eino-ext provider(OpenAI 兼容、Anthropic 等)时,在此分支构造即可,
// Runner、工具层、IR、导出层无需改动。
func NewChatModel(ctx context.Context, cfg config.ModelConfig) (model.ToolCallingChatModel, error) {
	switch strings.ToLower(cfg.Provider) {
	case "", "eino-script", "script":
		return &ScriptModel{}, nil
	case "ark":
		return newArkModel(ctx, cfg)
	default:
		return nil, errors.New("model provider " + cfg.Provider + " is not configured in this build; use eino-script or ark")
	}
}

// newArkModel 用 Ark Responses API 构造豆包 chat 模型。
// endpoint 默认 https://ark.cn-beijing.volces.com/api/v3(ResponsesAPI 在内部补 /responses);
// BaseURL 留空时用该默认值。
func newArkModel(ctx context.Context, cfg config.ModelConfig) (model.ToolCallingChatModel, error) {
	if cfg.Model == "" {
		return nil, errors.New("ark model requires config.model (endpoint id / model name)")
	}
	apiKey := cfg.APIKey
	if apiKey == "" && cfg.APIKeyEnv != "" {
		apiKey = os.Getenv(cfg.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, errors.New("ark model requires api key via config.model.apiKey or " + cfg.APIKeyEnv)
	}
	timeout := 10 * time.Minute
	arkCfg := &ark.ResponsesAPIConfig{
		APIKey:  apiKey,
		Model:   cfg.Model,
		BaseURL: cfg.BaseURL,
		Timeout: &timeout,
	}
	if cfg.MaxTokens > 0 {
		mt := cfg.MaxTokens
		arkCfg.MaxOutputTokens = &mt
	}
	if cfg.Temperature != nil {
		arkCfg.Temperature = cfg.Temperature
	}
	return ark.NewResponsesAPIChatModel(ctx, arkCfg)
}

// ScriptModel 是确定性 runner 使用的空操作 ToolCallingChatModel,仅用于离线/测试。
// 真实 LLM 由 NewChatModel 在 ark 分支返回。
type ScriptModel struct{}

// WithTools 返回新实例(满足接口,不依赖工具)。
func (m *ScriptModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &ScriptModel{}, nil
}

// Generate 返回占位 assistant 消息。脚本 runner 不依赖模型输出。
func (m *ScriptModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("script model is driven by the Go runner", nil), nil
}

// Stream 返回单元素流,包装同样的占位消息。
func (m *ScriptModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("script model is driven by the Go runner", nil)}), nil
}
