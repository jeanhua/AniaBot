# 内置插件

AniaBot 内置了多个开箱即用的插件，涵盖日志、AI 对话、消息管理等常见场景。

## pluginlog — 日志打印

在控制台打印所有收到的消息，便于开发调试。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"

bot.AddPlugin(pluginlog.NewPlugin())
```

**特性**：显示发送者昵称、用户 ID、消息内容。

---

## pluginsys — 系统帮助

提供 `/help` 命令，列出所有已注册插件的说明信息。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginsys"

bot.AddPlugin(pluginsys.NewPlugin())
```

**命令**：`/help` — 查看所有插件帮助

::: tip
`AdminOnly: true` 的插件对非管理员不可见。
:::

---

## pluginaichat — AI 对话

集成 AI 聊天功能，支持文本对话和图片内容理解（OCR）。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"

bot.AddPlugin(pluginaichat.NewAIChatPlugin())
```

**触发方式**：在群聊中 @ 机器人，或直接私聊发送消息即可开始对话。

### 用户命令

| 命令 | 说明 |
|------|------|
| `@机器人 <消息>` | 发起 / 继续对话 |
| `@机器人 <消息> #新对话` | 清空上下文，开启新一轮对话，同时清理动态加载的 MCP 工具 |
| `@机器人 /stop` | 立即中止当前 AI 响应 |

### 完整配置

```yaml
plugin:
  ai_chat_bot:
    # 必填
    base_url: "https://api.openai.com/v1"   # OpenAI 兼容接口地址
    model: "gpt-4o-mini"
    api_key: "sk-your-key"

    # 对话参数（均可选）
    prompt: "你是一个有趣的 QQ 机器人助手"
    max_token: 2048
    temperature: 0.7
    top_p: 0.9
    top_k: 50
    rate_limit: 2                            # 最大并发对话数，默认 2

    # 深度思考模式（需模型支持，如 claude-3-7-sonnet）
    thinking:
      enable: false
      mode: "auto"                           # none / low / medium / high / auto

    # 网页搜索 / 浏览（基于 Jina AI）
    search:
      token: "your-jina-token"              # 留空则禁用搜索和网页浏览工具

    # OCR 图片识别（可使用独立的视觉模型）
    ocr:
      enable: false
      base_url: "https://api.openai.com/v1"
      model: "gpt-4o"
      api_key: "sk-your-key"
      prompt: "描述图片内容"
      max_token: 1024
      temperature: 0.5
      top_p: 0.9
      top_k: 50

    # Skills 目录（默认 ./skills）
    skills_dir: "./skills"

component:
  md2img:
    apipoint: ""   # Markdown 转图片 API 地址，留空则禁用 shareimg 工具
```

### 内置工具（Tool Use）

AI 对话插件内置了一套工具，模型可在对话中自主调用：

| 工具 | 说明 | 依赖 |
|------|------|------|
| `time` | 获取当前时间 | 无 |
| `webSearch` | 互联网搜索（支持翻页） | Jina Token |
| `webExplore` | 浏览指定 URL 的页面内容 | Jina Token |
| `meme` | 根据文字描述发送表情包 | 无 |
| `file` | 生成并发送文本文件 | 无 |
| `get_msg_history` | 读取当前会话的历史消息（支持翻页） | 无 |
| `shareimg` | 将 Markdown 内容渲染为图片发送 | md2img API |
| `skill_read` | 读取指定 Skill 的详细指令 | Skills 目录 |

### MCP 工具扩展

通过 `aniabot.mcp.json` 文件可接入任意 MCP 服务器，AI 会自动发现并调用其工具。

**stdio 模式**（本地进程）：

```json
{
  "servers": [
    {
      "name": "my-server",
      "transport": "stdio",
      "command": "python",
      "args": ["-m", "mcp_server"],
      "env": { "API_KEY": "xxx" },
      "timeout": 30
    }
  ]
}
```

**HTTP 模式**（远程服务，支持 `sse` / `streamable-http`）：

```json
{
  "servers": [
    {
      "name": "remote-server",
      "transport": "sse",
      "endpoint": "http://localhost:3000",
      "headers": { "Authorization": "Bearer token123" },
      "timeout": 30
    }
  ]
}
```

MCP 工具在每次 `#新对话` 或超过 30 条未 @ 消息后自动清理，避免工具列表膨胀。

### Skills 系统

Skills 是写在 Markdown 文件中的「技能指令」，注入到 AI 的 system prompt 里，引导模型在特定场景下按预定流程行事。

**目录结构**：

```
skills/
├── translate/
│   └── SKILL.md
└── summarize/
    └── SKILL.md
```

**SKILL.md 格式**：

```markdown
---
name: translate
description: 将用户输入的文本翻译成目标语言
---

## 指令

1. 识别用户输入的语言
2. 询问目标语言（如未指定）
3. 调用翻译并返回结果
```

启动时框架自动扫描 `skills_dir` 目录，将所有 Skill 的名称和描述注入 system prompt。当模型判断需要使用某个 Skill 时，会先调用 `skill_read` 工具读取完整指令，再按指令执行。

### 其他特性

- **并发控制**：同一会话同时只处理一个请求，重复 @ 会提示等待
- **自动清理上下文**：群内超过 30 条消息未 @ 机器人时，自动清空该群对话历史
- **Token 统计**：每次对话结束后在日志中输出 prompt / completion / total token 消耗

---

## pluginrepeat — 复读机

当群内同一条消息连续出现 3 次时自动复读，增加群聊趣味性。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"

bot.AddPlugin(pluginrepeat.NewPlugin())
```

**管理命令**（需 @ 机器人）：

| 命令 | 说明 |
|------|------|
| `/close repeat` | 关闭复读机（管理员） |
| `/enable repeat` | 开启复读机（管理员） |

---

## pluginantiwithdrawal — 防撤回

缓存群聊消息，防止消息被撤回后无法查看，支持消息回顾。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"

bot.AddPlugin(pluginantiwithdrawal.NewPlugin())
```

**命令**：

| 场景 | 命令 | 说明 |
|------|------|------|
| 群聊 | `@机器人 /explore [数量]` | 查看本群最近消息（默认 50，最多 100） |
| 私聊（仅管理员） | `/explore [群号] [数量]` | 查看指定群最近消息 |

**特性**：每个群缓存最近 100 条消息，消息被撤回后仍可通过命令查看。

---

## pluginnews — 新闻推送

定期抓取新闻资讯并推送到指定群聊，支持 Cron 定时配置。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginnews"

bot.AddPlugin(pluginnews.NewPlugin())
```

**配置示例**：

```yaml
plugin:
  dailyNews:
    api: "https://uapis.cn/api/v1/daily/news-image"
    cron: "0 12 * * *"   # 每天 12:00 推送
    groups:
      - 123456789
```

---

## 完整注册示例

```go
package main

import (
    aniabot "github.com/jeanhua/AniaBot/bot/core"
    "github.com/jeanhua/AniaBot/bot/adapter/napcat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginsys"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginnews"
)

func main() {
    adapter := napcat.NewNapcatWebSocketAdapter()
    bot := aniabot.NewAniaBot(adapter)

    bot.AddPlugin(pluginlog.NewPlugin())
    bot.AddPlugin(pluginsys.NewPlugin())
    bot.AddPlugin(pluginrepeat.NewPlugin())
    bot.AddPlugin(pluginaichat.NewAIChatPlugin())
    bot.AddPlugin(pluginantiwithdrawal.NewPlugin())
    bot.AddPlugin(pluginnews.NewPlugin())

    bot.Run()
}
```
