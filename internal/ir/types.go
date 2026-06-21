package ir

import "time"

// ElementFingerprint captures identifying attributes of an interacted element,
// used to validate regenerated locators in exported tests.
type ElementFingerprint struct {
	TagName        string `json:"tagName,omitempty"`
	Role           string `json:"role,omitempty"`
	AccessibleName string `json:"accessibleName,omitempty"`
	ID             string `json:"id,omitempty"`
	NameAttr       string `json:"nameAttr,omitempty"`
	TypeAttr       string `json:"typeAttr,omitempty"`
	Placeholder    string `json:"placeholder,omitempty"`
	AriaLabel      string `json:"ariaLabel,omitempty"`
	TestID         string `json:"testId,omitempty"`
	TextSnippet    string `json:"textSnippet,omitempty"`
}

// LocatorCandidate is one ranked locator strategy (test id, role, label, css,
// text) for an element; higher Score wins.
type LocatorCandidate struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Score int    `json:"score"`
}

// ActionRecord is one line in ir.jsonl describing a single tool action during
// a spec run, sufficient to replay and to export a Playwright test.
type ActionRecord struct {
	RunID         string              `json:"runId"`
	SpecPath      string              `json:"specPath"`
	StepIndex     int                 `json:"stepIndex"`
	StepText      string              `json:"stepText"`
	ToolName      string              `json:"toolName"`
	ToolInput     any                 `json:"toolInput,omitempty"`
	Outcome       string              `json:"outcome"`
	ErrorCode     string              `json:"errorCode,omitempty"`
	PageURL       string              `json:"pageUrl,omitempty"`
	Element       *ElementFingerprint `json:"element,omitempty"`
	ChosenLocator *LocatorCandidate   `json:"chosenLocator,omitempty"`
	Timestamp     time.Time           `json:"timestamp"`
}
