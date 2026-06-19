# Guarantee_Agent

让 Markdown 测试用例自己跑起来的 AI 测试 Agent —— 核心理念:**文档即测试**。你用 Markdown 写验收用例,Agent 用真实浏览器 + 大模型自动把它们执行出来,记录每一步动作,成功用例还能导出成 Playwright 测试。

## 主要功能

1. **解析 Markdown 验收用例**:`## Preconditions` + 有序步骤 + `Expected:` 子句 + `include:` 复用步骤 + 严格 `{{VAR}}` 模板渲染。
2. **真实浏览器执行**:Playwright Chromium 驱动,模型经 Eino ReAct 循环发出工具调用(snapshot / navigate / click / fill / select_option / scroll / wait / assertTextPresent / assertElementVisible)在真实页面上跑。
3. **真实 LLM 驱动**:默认接豆包 Ark Responses API(模型 `deepseek-v4-pro-260425`),由模型自主规划工具调用;护栏(最大调用数 / 连续错误 / 单步重试)+ "断言必须真成功"硬校验,防止谎报通过。
4. **记录 IR 并导出**:每次工具调用落到 `.autoqa/runs/<runId>/ir.jsonl`;成功用例导出 `tests/autoqa/<name>.spec.ts`。
5. **plan 自动生成用例**:`autoqa plan` 用 LLM 探索页面、生成 Markdown 用例并 parser 自检,生成的用例可直接被 `autoqa run` 跑通。

CLI 子命令:`init`、`run <file-or-dir>`、`plan`、`plan-explore`、`plan-generate`;退出码 `0` 成功、`1` 运行失败、`2` 配置错误。

## 技术栈

| 关注点 | 选型 |
|---|---|
| 语言 / CLI | Go + spf13/cobra |
| Agent 框架 | 字节 CloudWeGo Eino(`components/model`、`components/tool`、`schema`) |
| LLM | 豆包 Ark,经 `eino-ext/components/model/ark` 的 Responses API |
| 浏览器 | playwright-go(Chromium) |
| Markdown | yuin/goldmark |
| 配置 | JSON / YAML |
| 日志 | log/slog(JSON) |

## 快速开始

```bash
# 1) 安装依赖与浏览器
go mod tidy
go run github.com/playwright-community/playwright-go/cmd/playwright install chromium

# 2) 配置豆包 API key
export ARK_API_KEY=你的key   # 或写入 .env

# 3) 初始化 + 跑用例
go run ./cmd/autoqa init --force
go run ./cmd/autoqa run specs/example.md --url http://localhost:8080 --headless

# 4) 让 LLM 自动生成并验证用例
go run ./cmd/autoqa plan --url http://localhost:8080 --headless
go run ./cmd/autoqa run specs/ --url http://localhost:8080 --headless
```

模型配置在 `autoqa.config.json` 的 `model` 段;默认 `provider=ark`、`model=deepseek-v4-pro-260425`,API key 从 `ARK_API_KEY` 环境变量读取。设 `provider=eino-script` 可切到离线确定性模式(不调用 LLM,仅用于测试)。

## Markdown 用例格式

```markdown
# Login smoke

## Preconditions
- The app is available.

## Steps
1. Navigate to {{BASE_URL}}
2. Verify Example text is visible
```

支持的模板变量:`{{BASE_URL}}` `{{LOGIN_BASE_URL}}` `{{ENV}}` `{{USERNAME}}` `{{PASSWORD}}`(未知或空值会报错,绝不静默运行)。可用 `include: login` 复用 `steps/login.md`。

## 项目结构

```text
cmd/autoqa/main.go                进程入口
internal/cli/                     命令注册(root/init/run/plan)
internal/config/                  配置 schema、默认值、加载与合并
internal/env/                     .env 文件加载(已有环境变量优先)
internal/markdown/                Markdown 解析、include 展开、模板渲染
internal/specs/                   用例发现(单文件 / 递归目录)
internal/browser/                 Playwright 浏览器抽象 Page
internal/tools/                   Eino typed 工具集合 + 统一 Result
internal/agent/                   Eino 模型边界、提示词、护栏、ReAct Runner
internal/runner/                  run 主流程编排 + 成功用例导出
internal/ir/                      IR 类型、JSONL writer/reader
internal/logging/                 结构化日志(slog JSON)与 URL 脱敏
internal/plan/                    探索 + 生成(plan/explore/generate)+ URL scope
testdata/                         冒烟用例和 fixture HTML 页面
```

## 调用链

### 入口

```text
cmd/autoqa/main.go::main
   └─ cli.Execute              [internal/cli/root.go]
        └─ newRootCommand      注册 init / run / plan / plan-explore / plan-generate
             └─ cobra.Command.Execute
                  └─ 各子命令 RunE
```

`cli.Execute` 把子命令返回的 `exitError` 翻译成进程退出码:`ExitOK=0`、`ExitRuntime=1`、`ExitConfig=2`。

### `autoqa init`

```text
cli.newInitCommand.RunE       [internal/cli/init.go]
   ├─ config.WriteDefault     写出 autoqa.config.json(--force 先删后写)
   ├─ os.MkdirAll specs/      创建用例目录
   ├─ 写入 specs/example.md   仅当不存在时写入
   └─ os.MkdirAll steps/      创建可复用步骤目录
```

### `autoqa run <file-or-dir>`(核心)

```text
cli.newRunCommand.RunE                            [internal/cli/run.go]
   └─ runner.Run                                  [internal/runner/run_specs.go]
        ├─ env.Load(cwd, envName)                 [internal/env/load.go]
        │     先 .env,再 .env.<envName>;已存在的环境变量绝不覆盖
        ├─ config.Load(cwd)                       [internal/config/load.go]
        │     默认值起点 + 文件值 merge + Validate
        ├─ baseURL 解析                           CLI > AUTOQA_BASE_URL > plan.baseUrl
        ├─ specs.Discover(path)                   [internal/specs/discover.go]
        ├─ logging.New(runDir, debug)             [internal/logging/logger.go]
        └─ 对每个 spec 路径:
             ├─ markdown.ExpandIncludes           [internal/markdown/include.go]
             ├─ markdown.RenderTemplate           [internal/markdown/template.go]
             │     {{BASE_URL}} {{LOGIN_BASE_URL}} {{ENV}} {{USERNAME}} {{PASSWORD}}
             ├─ markdown.Parse                    [internal/markdown/parse.go]
             └─ agent.Runner.Run(ctx, opts)       [internal/agent/runner.go]
                  ├─ browser.NewPage              [internal/browser/browser.go]
                  │     启动 Playwright Chromium(headless 可控),开 context+page
                  ├─ ir.NewWriter(...ir.jsonl)    [internal/ir/writer.go]
                  ├─ tools.Registry.EinoTools     [internal/tools/registry.go]
                  │     utils.InferTool 构建 9 个 Eino InvokableTool,绑定到 Page
                  ├─ agent.NewChatModel           [internal/agent/models.go]
                  │     provider=ark → ark.NewResponsesAPIChatModel(豆包 Responses API)
                  │     provider=eino-script → ScriptModel(离线测试)
                  ├─ model.WithTools(infos)       绑定工具 schema
                  └─ ReAct 循环(最多 maxReactRounds 轮):
                       ├─ model.Generate(messages)        模型生成
                       ├─ 若无 tool_calls → 任务完成,结束
                       └─ 否则逐个执行 tool_call:
                            ├─ executeTool → inv[name].InvokableRun
                            │     工具内部 → Page.Navigate/Click/Fill/AssertText/Snapshot
                            │     工具回调 OnResult → ir.Writer.Write
                            ├─ counters.OnToolResult(...)  护栏:连续错误 + 单步重试
                            ├─ 记录断言成功标记(stepIndex)
                            └─ 追加 schema.ToolMessage 回填结果给模型
                  └─ 最终硬校验:Expected/Verify 步骤必须有过一次成功断言
                                  否则 STEP_VALIDATION_FAILED
        └─ 成功 spec → runner.ExportPlaceholder   [internal/runner/export.go]
             生成 tests/autoqa/<name>.spec.ts
```

每个 spec 失败只计数,不中断整批;最终按 `Failed > 0` 决定退出码。

### ReAct 循环(LLM 如何驱动)

Runner 不硬编码"步骤→工具"映射,而是把工具 schema 与任务提示词交给模型,由模型自主发出 `tool_calls`:

```text
messages = [system(硬规则), user(BuildPrompt 的任务)]
loop:
   assistant = model.Generate(messages)
   messages.append(assistant)
   if assistant 无 tool_calls: break        # 模型判定完成
   for call in assistant.ToolCalls:
       res = executeTool(call)              # 真实在 Playwright 上执行
       更新护栏 / 断言标记 / IR
       messages.append(ToolMessage(res))    # 回填观察,供下一轮决策
最终:校验每个 Expected/Verify 步骤都有成功断言
```

护栏(`agent.GuardrailCounters`)在每轮与每个工具结果上生效:超过 `MaxToolCallsPerSpec` / `MaxConsecutiveErrors` / `MaxRetriesPerStep` 立即失败。硬校验保证模型即便只输出文本也无法绕过断言。

### 工具层(Eino 边界)

```text
tools.Registry.EinoTools                [internal/tools/registry.go]
   每个工具 = utils.InferTool[Input, Result]
        ├─ Input 内嵌 BaseInput.StepIndex(模型必须传)
        ├─ 函数体调用 browser.Page 的对应方法
        ├─ 预期失败 → tools.Fail(code, msg, retriable)
        ├─ 成功     → tools.OK(msg, data)
        └─ 通过 r.OnResult 把结果转给 runner 写 IR
```

| 工具名 | Page 方法 | 失败码 |
|---|---|---|
| `snapshot` | `Page.Snapshot`(body 文本) | — |
| `navigate` | `Page.Navigate` | `NAVIGATION_FAILED` |
| `click` | `Page.Click` | `ELEMENT_NOT_FOUND` |
| `fill` / `select_option` | `Page.Fill` | `ELEMENT_NOT_FOUND` |
| `scroll` / `wait` | 占位 | — |
| `assertTextPresent` / `assertElementVisible` | `Page.AssertText` | `ASSERTION_FAILED` |

`Result` 序列化为 JSON 后作为 tool 消息内容回到模型。预期失败返回 `Result`(非 Go error),保持 ReAct 可恢复。

### 浏览器抽象(Playwright)

`internal/browser.Page` 用真实 Chromium:

- `NewPage(baseURL, runDir, headless)`:启动 Playwright → Chromium → context(1440×900)→ page。
- `Navigate`:`page.Goto` 等 DOMContentLoaded,`< 400` 才成功。
- `Click`/`Fill`:经 `locatorFor` 多策略定位(精确文本 → label → placeholder → 模糊文本 → CSS 选择器),取 `First()`。
- `AssertText`:`page.GetByText` + `IsVisible`。
- `Snapshot`:返回 `body` 内联文本作为给模型的观察值。
- `Screenshot`:写 `runDir/snapshots/<name>.png`。
- `Close`:释放 browser + playwright 进程。

### `autoqa plan`(探索 + 生成)

```text
cli.newPlanCommand.RunE                       [internal/cli/plan.go]
   └─ runPlan(ctx, url, headless)
        ├─ planSetup                          加载 env/config、解析 baseURL、建 run 目录
        ├─ plan.Explore                       [internal/plan/agent.go]
        │     ├─ browser.NewPage              启动真实浏览器
        │     ├─ tools.Registry.EinoTools     同 run 的工具集
        │     ├─ agent.NewChatModel           豆包 Ark
        │     └─ ReAct 循环:模型 navigate+snapshot 观察页面
        │        最终输出 JSON(url/title/description/elements/links)
        ├─ 写出 .autoqa/runs/<runId>/plan-explore/summary.json
        └─ plan.GenerateSpecs
             ├─ 模型按 summary 生成 Markdown(系统提示规定结构)
             ├─ markdown.Parse 自检           必须能被解析,否则报错不写
             └─ 写入 specs/<sanitized-title>.md
```

`plan-explore` 只跑探索并写 summary;`plan-generate --from <summary.json>` 只从已有 summary 生成。生成的用例可直接 `autoqa run specs/` 跑通。

URL 范围过滤 `internal/plan/url_scope.go::IsURLInScope`(exclude 优先 → include 命中 → site 默认放行)为后续多页探索预留。

## 模型边界与扩展点

`internal/agent/models.go::NewChatModel` 是替换 LLM 的唯一入口:

- `provider=ark`:豆包 Ark Responses API,凭据从 `config.model.apiKey` 或 `ARK_API_KEY` 环境变量读取,模型名 `config.model.model`。
- `provider=eino-script`:离线确定性 `ScriptModel`,不调 LLM,仅用于测试。
- 其他 provider(OpenAI 兼容、Anthropic via eino-ext):在 switch 分支构造对应 `ToolCallingChatModel` 即可,Runner、工具层、IR、导出层无需改动。

## 产物布局

```text
.autoqa/
└─ runs/<runId>/
   ├─ run.log.jsonl                 slog JSON 日志
   ├─ ir.jsonl                      每次工具调用一行的 ActionRecord
   ├─ snapshots/<name>.png          浏览器截图
   └─ plan-explore/summary.json     探索结果
tests/
└─ autoqa/<spec-name>.spec.ts       成功 run 导出的 Playwright 用例
specs/
└─ <generated>.md                   plan 生成的 Markdown 用例(parser 自检通过)
```

`ir.ActionRecord` 字段:`runId / specPath / stepIndex / stepText / toolName / toolInput / outcome / errorCode / pageUrl / element / chosenLocator / timestamp`,可被 `ir.ReadFile` 读取以回放 / 导出。

## 验证

```bash
go vet ./... && go test ./...                      # 静态检查 + 单测
go build -o /tmp/autoqa ./cmd/autoqa               # 构建

# 真实 LLM + Playwright 端到端
export ARK_API_KEY=你的key
python3 -m http.server 18090 --directory testdata/www &
tmp=$(mktemp -d); cd "$tmp"
/tmp/autoqa init --force
cp <repo>/testdata/specs/simple.md specs/simple.md
/tmp/autoqa run specs/simple.md --url http://127.0.0.1:18090 --headless
# 期望: passed=1 failed=0;ir.jsonl 含 navigate/snapshot/assertTextPresent

# plan 自动生成并跑通
/tmp/autoqa plan --url http://127.0.0.1:18090 --headless
/tmp/autoqa run specs/ --url http://127.0.0.1:18090 --headless
```
