package markdown

// StepKind 把 Markdown 步骤分类为动作或断言。
type StepKind string

const (
	StepKindAction    StepKind = "action"
	StepKindAssertion StepKind = "assertion"
)

// Step 是 Markdown 用例中的一个编号步骤。
type Step struct {
	Index          int      `json:"index"`                    // 1-based 编号,保留列表起始值
	Text           string   `json:"text"`                     // 步骤正文指令
	ExpectedResult string   `json:"expectedResult,omitempty"` // 可选的 "Expected:" 子句
	Kind           StepKind `json:"kind"`                     // 动作或断言
}

// Spec 是已解析的 Markdown 验收用例。
type Spec struct {
	Title         string   `json:"title,omitempty"` // 首个 H1,可选
	Preconditions []string `json:"preconditions"`   // 必需的非空列表
	Steps         []Step   `json:"steps"`           // 必需的有序步骤
}

// ParseError 在用例格式错误时由 Parse 返回。Code 是稳定标识
// (MARKDOWN_MISSING_PRECONDITIONS、MARKDOWN_EMPTY_PRECONDITIONS、
// MARKDOWN_MISSING_STEPS、MARKDOWN_EMPTY_STEPS)。
type ParseError struct {
	Code    string
	Message string
}

func (e *ParseError) Error() string { return e.Code + ": " + e.Message }
