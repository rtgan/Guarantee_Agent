# Guarantee_Agent

让 Markdown 测试用例自己跑起来的 AI 测试工具。核心理念：**文档即测试**。你用 Markdown 写好验收用例，AI 自动控制真实浏览器帮你执行，记录每一步操作，成功的用例还能导出成 Playwright 测试代码。

## 能做什么

1. **解析 Markdown 测试用例**：支持 `## Preconditions`（前置条件）+ 有序步骤 + `Expected:`（期望结果）+ `include:`（复用 `steps/` 下的步骤片段）+ 严格的 `{{变量}}` 模板语法。
2. **用真实浏览器跑**：通过 Playwright Chromium 驱动（**在你本机拉起一个真实 Chromium 让 AI 操控它**），AI 自主决定调用哪些工具（打开网页 / 点击 / 填写 / 下拉选择 / 滚动 / 等待 / 验证文字出现 / 验证元素可见 / 读取页面快照），在真实网页上跑。
3. **AI 自主驱动**：默认接入豆包 Ark 的 Responses API（模型 `deepseek-v4-pro-260425`），让 AI 自己规划该调什么工具；有保护机制（最大调用次数 / 连续出错 / 单步重试）+ "断言必须真成功"硬校验，防止 AI 糊弄过去假装通过了。
4. **记录操作日志并支持导出**：每次工具调用都会写入 `Guarantee_Agent/.autoqa/runs/<runId>/ir.jsonl`；跑成功的用例会自动导出为 `Guarantee_Agent/tests/autoqa/test_<name>.py`。【注：`ir.jsonl` 一行就是一个 [ActionRecord](vscode-webview://08tl54h03nehmajltabign5atut4qdnbg8duc5m4gtdh2hobd9mu/internal/ir/types.go)，字段是机器消费导向的，便于导出测试代码】

5. **自动生成测试用例**：`autoqa plan` 让 AI 先探索网页，再根据探索结果自动生成 Markdown 测试用例，生成的用例可以直接被 `autoqa run` 跑起来。

CLI 子命令：`init`、`run <file-or-dir>`、`plan`、`plan-explore`、`plan-generate`；退出码 `0` 成功、`1` 运行失败、`2` 配置错误。

## 技术栈

| 关注点 | 选型 |
|---|---|
| 语言 / CLI | Go + spf13/cobra |
| Agent 框架 | 字节 CloudWeGo Eino（`components/model`、`components/tool`、`schema`） |
| LLM | 豆包 Ark，通过 `eino-ext/components/model/ark` 的 Responses API |
| 浏览器 | playwright-go（Chromium） |
| Markdown 解析 | yuin/goldmark |
| 配置 | JSON / YAML |
| 日志 | log/slog（JSON 格式） |

## 快速开始

```bash
# 1) 安装依赖和浏览器
go mod tidy
go run github.com/playwright-community/playwright-go/cmd/playwright install chromium

# 2) 配置豆包 API key
export ARK_API_KEY=你的key   # 或者写到 .env 文件里

# 3) 初始化 + 跑测试用例
go run ./cmd/autoqa init --force
go run ./cmd/autoqa run specs/example.md --url http://localhost:8080 --headless

# 4) 让 AI 自动生成测试用例，然后跑起来
go run ./cmd/autoqa plan --url http://localhost:8080 --headless
go run ./cmd/autoqa run specs/ --url http://localhost:8080 --headless
```

模型配置在 `autoqa.config.json` 的 `model` 段；默认 `provider=ark`、`model=deepseek-v4-pro-260425`，API key 从 `ARK_API_KEY` 环境变量读取。设置 `provider=eino-script` 可以切换到离线确定性模式（不调 AI，仅用于测试开发）。

## Markdown 用例格式

```markdown
# 登录冒烟测试

## Preconditions
- 应用已启动。

## Steps
1. 打开 {{BASE_URL}}
2. 验证页面上能看到 "Example" 文字
```

支持的变量：`{{BASE_URL}}`、`{{LOGIN_BASE_URL}}`、`{{ENV}}`、`{{USERNAME}}`、`{{PASSWORD}}`（变量不存在或值为空会报错，不会悄悄跳过）。可以用 `include: login` 引用 `steps/login.md` 文件中的步骤，被引用片段会原地展开并与后续步骤合并成一个连续的有序列表（不支持嵌套 include）。可运行的示例见 `testdata/specs/login_flow.md` + `testdata/steps/login.md`：片段里用 `{{USERNAME}}`/`{{PASSWORD}}` 复用变量并用 `Expected:` 声明预期，外层用例以 `include: login` 接入后继续编号 `4.`、`5.`。

## 项目结构

```
cmd/autoqa/main.go                程序入口
internal/cli/                     注册命令（init / run / plan 各种子命令）
internal/config/                  配置结构、默认值、加载与合并逻辑
internal/env/                     加载 .env 文件（已有的环境变量不会被覆盖）
internal/markdown/                解析 Markdown、展开 include、渲染变量模板
internal/specs/                   发现测试用例（单文件或整个目录）
internal/browser/                 封装 Playwright 浏览器操作
internal/tools/                   AI 可调用的工具集合 + 统一返回格式
internal/agent/                   AI 模型边界、提示词、保护机制、ReAct 循环
internal/runner/                  测试运行主流程 + 成功用例导出
internal/ir/                      操作记录类型、JSONL 写入和读取
internal/logging/                 结构化日志（slog JSON）+ URL 脱敏
internal/plan/                    探索网页 + 生成用例（plan/explore/generate）+ URL 范围过滤
testdata/                        示例测试用例和测试用的 HTML 页面
```

## 整体流程

### 入口

```
cmd/autoqa/main.go::main
   └─ cli.Execute              [internal/cli/root.go]
        └─ newRootCommand      注册 init / run / plan / plan-explore / plan-generate
             └─ cobra.Command.Execute
                  └─ 各子命令 RunE
```

`cli.Execute` 把子命令返回的错误翻译成进程退出码：`ExitOK=0`、`ExitRuntime=1`、`ExitConfig=2`。

### `autoqa init`

```
cli.newInitCommand.RunE       [internal/cli/init.go]
   ├─ config.WriteDefault     写出 autoqa.config.json（加 --force 会先删再写）
   ├─ os.MkdirAll specs/      创建测试用例目录
   ├─ 写入 specs/example.md   仅当文件不存在时写入
   └─ os.MkdirAll steps/      创建可复用步骤目录
```

### `autoqa run <file-or-dir>`（核心流程）

```
cli.newRunCommand.RunE                            [internal/cli/run.go]
   └─ runner.Run                                  [internal/runner/run_specs.go]
        ├─ env.Load(cwd, envName)                 [internal/env/load.go]
        │     先加载 .env，再加载 .env.<envName>；已有的环境变量不会被覆盖
        ├─ config.Load(cwd)                       [internal/config/load.go]
        │     默认配置 + 文件配置合并 + 校验
        ├─ baseURL 解析                           CLI 参数 > AUTOQA_BASE_URL 环境变量 > plan 里的 baseUrl
        ├─ specs.Discover(path)                   [internal/specs/discover.go]
        ├─ logging.New(runDir, debug)             [internal/logging/logger.go]
        └─ 对每个测试用例:
             ├─ markdown.ExpandIncludes           [internal/markdown/include.go]
             ├─ markdown.RenderTemplate           [internal/markdown/template.go]
             │     替换 {{BASE_URL}} {{LOGIN_BASE_URL}} {{ENV}} {{USERNAME}} {{PASSWORD}}
             ├─ markdown.Parse                    [internal/markdown/parse.go]
             └─ agent.Runner.Run(ctx, opts)       [internal/agent/runner.go]
                  ├─ browser.NewPage              [internal/browser/browser.go]
                  │     启动 Playwright Chromium（可控制是否无头模式），创建 context 和 page
                  ├─ ir.NewWriter(...ir.jsonl)    [internal/ir/writer.go]
                  ├─ tools.Registry.EinoTools     [internal/tools/registry.go]
                  │     用 utils.InferTool 构建 9 个 Eino 可调用工具，绑定到 Page 对象
                  ├─ agent.NewChatModel           [internal/agent/models.go]
                  │     provider=ark → 豆包 Ark Responses API
                  │     provider=eino-script → 离线脚本模型（不调 AI，用于测试）
                  ├─ model.WithTools(infos)       绑定工具定义给模型
                  └─ ReAct 循环（最多 maxReactRounds 轮）:
                       ├─ model.Generate(messages)        AI 生成下一步
                       ├─ 若无 tool_calls → 任务完成，结束
                       └─ 否则逐个执行 tool_call:
                            ├─ executeTool → inv[name].InvokableRun
                            │     工具内部 → Page.Navigate/Click/Fill/AssertText/Snapshot
                            │     工具回调 OnResult → ir.Writer.Write
                            ├─ counters.OnToolResult(...)  保护机制：连续错误 + 单步重试
                            ├─ 记录断言成功标记(stepIndex)
                            └─ 追加 schema.ToolMessage 回填结果给 AI
                  └─ 最终硬校验：Expected/Verify 步骤必须至少有一次断言成功
                                  否则判定为 STEP_VALIDATION_FAILED
        └─ 成功的用例 → runner.ExportPlaceholder   [internal/runner/export.go]
             生成 Guarantee_Agent/tests/autoqa/test_<name>.py
```

每个用例失败只计数，不会中断整个批次；最终按 `Failed > 0` 决定退出码。

### ReAct 循环（AI 是怎么驱动测试的）

Runner 不硬编码"第几步该用什么工具"，而是把可用工具和任务描述一起扔给 AI，让 AI 自己决定下一步该干什么：

```
messages = [system(规则), user(任务)]
loop:
   assistant = model.Generate(messages)
   messages.append(assistant)
   if assistant 无 tool_calls: break        # AI 认为完成了
   for call in assistant.ToolCalls:
       res = executeTool(call)              # 在 Playwright 上真实执行
       更新保护机制 / 断言标记 / 操作记录
       messages.append(ToolMessage(res))    # 把执行结果告诉 AI，让它继续决策
最终：检查每个 Expected/Verify 步骤是否都有成功的断言
```

保护机制（`agent.GuardrailCounters`）在每轮和每个工具结果上都生效：超过 `MaxToolCallsPerSpec`（总调用次数上限）、`MaxConsecutiveErrors`（连续出错次数）、`MaxRetriesPerStep`（单步重试次数）会立即判定失败。硬校验保证 AI 就算只输出文字也无法绕过断言。

### 工具层（Eino 边界）

```
tools.Registry.EinoTools                [internal/tools/registry.go]
   每个工具 = utils.InferTool[Input, Result]
        ├─ Input 内嵌 BaseInput.StepIndex（AI 必须传这一步的编号）
        ├─ 函数体调用 browser.Page 的对应方法
        ├─ 预期失败 → tools.Fail(code, msg, retriable)
        ├─ 成功     → tools.OK(msg, data)
        └─ 通过 r.OnResult 把结果转给 runner 写入操作记录
```

| 工具名 | Page 方法 | 失败码 |
|---|---|---|
| `snapshot` | `Page.Snapshot`（取 body 文本） | — |
| `navigate` | `Page.Navigate` | `NAVIGATION_FAILED` |
| `click` | `Page.Click` | `ELEMENT_NOT_FOUND` |
| `fill` | `Page.Fill` | `ELEMENT_NOT_FOUND` |
| `select_option` | `Page.SelectOption`（真实 `<select>` 选择） | `ELEMENT_NOT_FOUND` |
| `scroll` | `Page.Scroll`（滚轮滚动一屏） | `SCROLL_FAILED` |
| `wait` | `Page.Wait`（阻塞指定毫秒） | `WAIT_FAILED` |
| `assertTextPresent` / `assertElementVisible` | `Page.AssertText` | `ASSERTION_FAILED` |

`Result` 序列化为 JSON 后作为 tool 消息内容回到 AI。预期内的失败返回 `Result`（不是 Go error），保持 ReAct 循环可恢复。

### 浏览器抽象（Playwright）

`internal/browser.Page` 控制真实 Chromium：

- `NewPage(baseURL, runDir, headless)`：启动 Playwright → Chromium → context（1440×900）→ page。
- `Navigate`：`page.Goto`，等 DOMContentLoaded 事件，HTTP 状态码 < 400 才算成功。
- `Click`/`Fill`：通过 `locatorFor` 多策略定位（精确文本 → label → placeholder → 模糊文本 → CSS 选择器），取 `First()`。
- `AssertText`：`page.GetByText` + `IsVisible`。
- `Snapshot`：返回 `body` 内联文本作为 AI 的观察值。
- `Screenshot`：截图写入 `runDir/snapshots/<name>.png`。
- `Close`：关闭浏览器和 Playwright 进程。

### `autoqa plan`（探索网页 + 生成用例）

```
cli.newPlanCommand.RunE                       [internal/cli/plan.go]
   └─ runPlan(ctx, url, headless)
        ├─ planSetup                          加载 env/config、解析 baseURL、建运行目录
        ├─ plan.Explore                       [internal/plan/agent.go]
        │     ├─ browser.NewPage              启动真实浏览器
        │     ├─ tools.Registry.EinoTools     同 run 的工具集
        │     ├─ agent.NewChatModel           豆包 Ark
        │     └─ ReAct 循环：AI 自主打开网页、截图、观察页面
        │        最终输出 JSON（url/title/description/elements/links）
        ├─ 写出 Guarantee_Agent/.autoqa/runs/<runId>/plan-explore/summary.json
        └─ plan.GenerateSpecs
             ├─ AI 根据 summary 生成 Markdown（系统提示词规定结构）
             ├─ markdown.Parse 自检           必须能被解析，否则报错不写入
             └─ 写入 Guarantee_Agent/specs/<sanitized-title>.md
```

`plan-explore` 只跑探索步骤并写 summary；`plan-generate --from <summary.json>` 只从已有的 summary 生成用例。生成的用例可以直接 `autoqa run specs/` 跑起来。

URL 范围过滤 `internal/plan/url_scope.go::IsURLInScope`（exclude 优先 → include 命中 → site 默认放行）为后续多页探索预留。

## 怎么换 AI 模型

`internal/agent/models.go::NewChatModel` 是替换 AI 模型的唯一入口：

- `provider=ark`：豆包 Ark Responses API，凭证从 `config.model.apiKey` 或 `ARK_API_KEY` 环境变量读取，模型名取 `config.model.model`。
- `provider=eino-script`：离线确定性 `ScriptModel`，不调 AI，仅用于测试开发。
- 其他 provider（OpenAI 兼容、Anthropic 通过 eino-ext）：在 switch 分支构造对应的 `ToolCallingChatModel` 即可，Runner、工具层、操作记录、导出层都无需改动。

## 产物布局

```
Guarantee_Agent/
├─ .autoqa/
│  └─ runs/<runId>/
│     ├─ run.log.jsonl                 slog JSON 格式日志
│     ├─ ir.jsonl                      每次工具调用一行 ActionRecord
│     ├─ snapshots/<name>.png          浏览器截图
│     └─ plan-explore/summary.json     探索结果
├─ tests/
│  └─ autoqa/test_<spec-name>.py       成功用例导出的 Playwright(Python) 测试代码
└─ specs/
   └─ <generated>.md                   plan 生成的 Markdown 测试用例（parser 自检通过）
```

`ir.ActionRecord` 字段：`runId / specPath / stepIndex / stepText / toolName / toolInput / outcome / errorCode / pageUrl / element / chosenLocator / timestamp`，可用 `ir.ReadFile` 读取来回放或导出。

## 验证

```bash
go vet ./... && go test ./...                      # 静态检查 + 单元测试
go build -o /tmp/autoqa ./cmd/autoqa               # 构建

# 真实 AI + Playwright 端到端测试
export ARK_API_KEY=你的key
python3 -m http.server 18090 --directory testdata/www &
tmp=$(mktemp -d); cd "$tmp"
/tmp/autoqa init --force
cp <repo>/testdata/specs/simple.md specs/simple.md
/tmp/autoqa run specs/simple.md --url http://127.0.0.1:18090 --headless
# 期望：passed=1 failed=0；ir.jsonl 包含 navigate/snapshot/assertTextPresent

# plan 自动生成用例并跑通
/tmp/autoqa plan --url http://127.0.0.1:18090 --headless
/tmp/autoqa run specs/ --url http://127.0.0.1:18090 --headless
```
