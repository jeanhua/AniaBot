# bot/component

AI 聊天机器人的核心组件层，分为三个子包。

```
component/
├── aichat/      # 聊天机器人主逻辑
├── functool/    # 内置功能工具
└── llmtool/     # 工具框架与 MCP 集成
```

---

## aichat — 聊天机器人

对外暴露 `ChatBot`，负责协调 LLM 调用、消息构建、工具执行和对话历史。

| 文件 | 职责 |
|------|------|
| `chatbot.go` | 入口，组装各子组件，暴露 `Chat()` / `GetSingleImageDesc()` 等方法（均返回 `TokenUsage`，含当次上下文压缩的消耗） |
| `llmclient.go` | 封装 openai-go SDK，提供 `Generate()` / `GenerateSingle()` / `GenerateSingleWithUsage()` |
| `messagebuilder.go` | 构建每轮请求的消息列表（system prompt + 历史 + 用户输入），支持 Skill 注入 |
| `memorywindow.go` | 对话历史窗口：按 token 预算管理，prompt token 超过 `max_context_tokens` 的 80% 时用 LLM 将旧历史摘要压缩（工具调用细节不进入摘要），并负责历史持久化与回放 |
| `toolexecutor.go` | Agent 循环：LLM → 工具调用 → 结果 → LLM，追踪 `TokenUsage` |

**调用流程：**

```
ChatBot.Chat()
  └─ MessageBuilder.BuildChatMessages()   // 组装消息
  └─ ToolOrchestrator.ExecuteWithTools()  // Agent 循环
       ├─ LLMClient.Generate()            // 调用 LLM
       ├─ ToolExecutor.Execute()          // 执行工具
       └─ MessageBuilder.Build*()         // 构建工具结果消息
  └─ messageWindow.append()              // 保存本轮历史
```

---

## functool — 内置工具

提供开箱即用的工具，以及工具执行器的工厂函数。

| 文件 | 工具 | 说明 |
|------|------|------|
| `time.go` | `time` | 返回当前时间 |
| `jina.go` | `webSearch` / `webExplore` | 基于 Jina API 的网页搜索与浏览 |
| `sendfile.go` | `file` | 向用户发送生成的文件（需启用 file 配置）|
| `bash.go` | `bash` | 在宿主机执行 shell 命令（需启用 bash 配置，支持黑白名单）|
| `loadimages.go` | `load_images` | 加载消息中的图片供模型查看 |
| `localimage.go` | `local_image` | 读取本地图片（需启用 local_image 配置）|
| `msghistory.go` | `get_msg_history` | 获取当前会话历史消息（支持翻页）|
| `privatefile.go` | `get_private_file_url` | 获取私聊文件的下载链接 |
| `tools.go` | — | 工厂函数，按需组合工具执行器 |

**工厂函数（按需选用）：**

```go
// 仅内置工具
CreateDefaultTools(searchToken, bashConfig, fileConfig, localImageConfig)

// 内置工具 + MCP（mcpLazyLoad=true 走工具发现模式，false 全量注册）
CreateToolsWithMCP(searchToken, mcpConfigs, bashConfig, fileConfig, localImageConfig, mcpLazyLoad)

// 内置工具 + MCP + Skill（skills 非空时只加载指定名称的 skill）
CreateToolsWithSkill(searchToken, mcpConfigs, skillsDir, bashConfig, fileConfig, localImageConfig, skills, mcpLazyLoad)
```

三个工厂函数均返回 `error`（`CreateToolsWithSkill` 返回 `(*ToolExecuter, *SkillManager, error)`）。

---

## llmtool — 工具框架

定义工具接口、执行引擎、MCP 集成和 Skill 管理。

| 文件 | 职责 |
|------|------|
| `tool.go` | `Tool` 接口定义，`CallBackFuncs`（SendText / SendImage / SendFile）|
| `toolhelper.go` | 泛型 `BaseTool[T]`，消除工具样板代码 |
| `toolexecuter.go` | `ToolExecuter`（共享工具注册表）+ `SessionToolExecutor`（会话级隔离）|
| `parser.go` | 将 Go struct 或 MCP JSON Schema 转换为 OpenAI tool schema |
| `mcp.go` | MCP 客户端、工具包装、发现/加载模式（支持 stdio / SSE / Streamable HTTP）|
| `skill.go` | `SkillManager`：加载 SKILL.md 文件，注入 skill 列表到 system prompt |

**工具注册两种模式：**

```
传统模式（RegisterMCPWithConfig）
  └─ 一次性注册所有工具 → 工具多时导致上下文爆炸，仅适合工具数 < 5

发现模式（RegisterMCPWithConfigDiscovery）  ← 推荐
  └─ 共享层注册 MCPDiscoveryTool（列出可用工具）
  └─ 会话层注入 MCPLoaderTool（按需加载具体工具）
  └─ 动态加载的工具隔离在 SessionToolExecutor，不跨会话污染
```

**自定义工具示例：**

```go
type MyParams struct {
    Query string `json:"query" desc:"查询内容"`
}

type MyTool struct {
    llmtool.BaseTool[MyParams]
}

func (t *MyTool) Execute(ctx context.Context, params any, cb llmtool.CallBackFuncs) (string, error) {
    p := params.(*MyParams)
    return "result: " + p.Query, nil
}
```
