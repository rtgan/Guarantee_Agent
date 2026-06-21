package agent

import "testing"

func TestGuardrails(t *testing.T) {
	c := NewCounters()
	limits := GuardrailLimits{MaxToolCallsPerSpec: 2, MaxConsecutiveErrors: 1, MaxRetriesPerStep: 1}
	if err := c.OnToolCall(limits); err != nil {
		t.Fatal(err)
	}
	if err := c.OnToolCall(limits); err != nil {
		t.Fatal(err)
	}
	if err := c.OnToolCall(limits); err == nil {
		t.Fatal("expected max tool calls")
	}

	c = NewCounters()
	if err := c.OnToolResult(1, false, limits); err != nil {
		t.Fatal(err)
	}
	if err := c.OnToolResult(1, false, limits); err == nil {
		t.Fatal("expected consecutive error guardrail")
	}
}
