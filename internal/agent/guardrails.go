package agent

import "fmt"

// GuardrailLimits 是从配置加载的单用例安全阈值。
type GuardrailLimits struct{ MaxToolCallsPerSpec, MaxConsecutiveErrors, MaxRetriesPerStep int }

// GuardrailCounters 记录单个用例运行时的工具调用/错误/重试计数。
type GuardrailCounters struct {
	ToolCalls, ConsecutiveErrors int
	RetriesPerStep               map[int]int
}

// GuardrailError 在某条护栏阈值被突破时返回。Code 是稳定标识
// (GUARDRAIL_MAX_*),用于日志和 IR。
type GuardrailError struct {
	Code    string
	Message string
}

func (e *GuardrailError) Error() string { return e.Code + ": " + e.Message }

// NewCounters 返回清零的计数器,并初始化重试 map。
func NewCounters() GuardrailCounters { return GuardrailCounters{RetriesPerStep: map[int]int{}} }

// OnToolCall 累加工具调用总数,当用例超过 MaxToolCallsPerSpec 时返回错误。
func (c *GuardrailCounters) OnToolCall(limits GuardrailLimits) error {
	c.ToolCalls++
	if c.ToolCalls > limits.MaxToolCallsPerSpec {
		return &GuardrailError{Code: "GUARDRAIL_MAX_TOOL_CALLS", Message: fmt.Sprintf("tool calls exceeded %d", limits.MaxToolCallsPerSpec)}
	}
	return nil
}

// OnToolResult 在工具返回后更新连续错误计数和单步重试计数。
// 成功结果会重置连续错误;失败结果可能触发连续错误或单步重试护栏。
func (c *GuardrailCounters) OnToolResult(step int, ok bool, limits GuardrailLimits) error {
	if ok {
		c.ConsecutiveErrors = 0
		return nil
	}
	c.ConsecutiveErrors++
	if step > 0 {
		c.RetriesPerStep[step]++
	}
	if c.ConsecutiveErrors > limits.MaxConsecutiveErrors {
		return &GuardrailError{Code: "GUARDRAIL_MAX_CONSECUTIVE_ERRORS", Message: fmt.Sprintf("consecutive errors exceeded %d", limits.MaxConsecutiveErrors)}
	}
	if step > 0 && c.RetriesPerStep[step] > limits.MaxRetriesPerStep {
		return &GuardrailError{Code: "GUARDRAIL_MAX_RETRIES_PER_STEP", Message: fmt.Sprintf("step %d retries exceeded %d", step, limits.MaxRetriesPerStep)}
	}
	return nil
}
