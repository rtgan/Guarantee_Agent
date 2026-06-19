package markdown

import "testing"

// TestParseChineseSpec 验证中文标题、中文断言关键字、中文"预期:"子句都能被正确识别。
func TestParseChineseSpec(t *testing.T) {
	spec, err := Parse(`# 登录测试

## 前置条件
- 应用已启动

## 步骤
1. 打开 {{BASE_URL}}
2. 验证页面上能看到 "首页" 文字
3. 点击登录按钮
   - 预期: 进入仪表盘
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Preconditions) != 1 || spec.Preconditions[0] != "应用已启动" {
		t.Fatalf("preconditions = %#v", spec.Preconditions)
	}
	if len(spec.Steps) != 3 {
		t.Fatalf("steps = %d", len(spec.Steps))
	}
	// "验证..." 开头 → assertion
	if spec.Steps[1].Kind != StepKindAssertion {
		t.Fatalf("step 2 kind = %s, want assertion", spec.Steps[1].Kind)
	}
	// 中文"预期:"子句被抽取
	if spec.Steps[2].ExpectedResult != "进入仪表盘" {
		t.Fatalf("step 3 expected = %q", spec.Steps[2].ExpectedResult)
	}
}
