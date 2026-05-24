# 内置插件

AniaBot 内置了多个开箱即用的插件，涵盖日志、AI 对话、消息管理等常见场景。每个插件都实现了 `common/plugin.Plugin` 接口，按 `Order` 从小到大依次执行。

::: tip 插件开发
如果你想了解如何编写自己的插件，请参阅[插件开发概览](../plugin/overview.md)和[第一个插件](../plugin/first-plugin.md)教程。
:::

---

## pluginlog — 日志打印

在控制台打印所有收到的消息，便于开发调试。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"

bot.AddPlugin(pluginlog.NewPlugin())
```

**特性**：显示发送者昵称、用户 ID、消息内容。

### 什么时候需要它？

- **开发阶段**：你在编写新插件时，需要确认消息的结构、字段内容是否符合预期，开启日志插件可以在控制台实时看到每条消息的详细信息。
- **排查问题**：机器人在线上运行时出现异常行为，查看日志可以快速定位是哪条消息触发了问题。
- **监控运行**：简单确认机器人是否正常接收消息。

::: warning 生产环境建议
日志插件会打印所有消息内容，在流量较大的群中可能产生大量输出。生产环境建议根据需要决定是否启用。
:::

---

## pluginsys — 系统帮助

提供 `/help` 命令，列出所有已注册插件的说明信息。

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginsys"

bot.AddPlugin(pluginsys.NewPlugin())
```

**命令**：`/help` — 查看所有插件帮助

### /help 的工作原理

当用户发送 `/help` 时，插件系统会遍历所有已注册的插件，读取每个插件的 `Meta` 信息，然后按以下规则筛选和展示：

1. **AdminOnly 过滤**：`AdminOnly: true` 的插件对非管理员完全不可见，帮助信息中不会出现。
2. **ShowFor 过滤**：根据消息来源（群聊或私聊）匹配插件的 `ShowFor` 标志位，只展示适用场景的插件。
3. **拼接展示**：将每个插件的 `Name` 和 `HelpWords` 拼接为帮助列表返回给用户。

::: tip
`AdminOnly: true` 的插件对非管理员不可见。
:::

如果你想开发自己的插件并让它出现在 `/help` 中，只需在 `Meta` 中设置好 `HelpWords` 字段即可。详见[插件开发概览](../plugin/overview.md)中的 Meta 字段说明。

---

## pluginaichat — AI 对话

集成 AI 聊天功能，支持文本对话和图片内容理解（OCR）。这是 AniaBot 最核心的插件，内置了丰富的工具系统、MCP 扩展能力和 Skills 技能框架。

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

### 典型使用场景

以下是一个群聊中的真实对话流程：

```
用户: @机器人 帮我查一下今天的天气，北京的
机器人: [调用 webSearch 工具] 北京今天晴，气温 18-28°C，微风...

用户: @机器人 帮我把这段话翻译成英文：今天天气真好，适合出去玩
机器人: [识别翻译意图，调用 translate Skill] The weather is really nice today, perfect for going out.

用户: @机器人 生成一个 Python 冒泡排序的代码，保存为文件发给我
机器人: [调用 file 工具，生成 .py 文件并发送]

用户: @机器人 新对话
机器人: 已清空上下文，开始新一轮对话。

用户: @机器人 这张图片里写了什么？[附带图片]
机器人: [调用 OCR 模型识别图片] 图片中显示的是一张表格，包含...
```

::: tip 多轮对话与上下文
AI 对话插件会维护每个群/用户的对话历史。如果群内超过 30 条消息没有 @ 机器人，上下文会自动清空，避免无关消息干扰对话。你也可以手动发送 `#新对话` 来重置。
:::

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

### System Prompt 配置技巧

`prompt` 字段是 AI 的「人设」，写好它可以显著提升对话质量：

- **明确角色定位**：不要只写"你是一个助手"，而是写清楚它的职责边界，例如"你是一个 QQ 群聊助手，擅长回答技术问题，用轻松幽默的语气回复，回复尽量简短"。
- **约束输出格式**：如果你希望 AI 的回复适合 QQ 阅读，可以在 prompt 中加入"回复不要太长，避免使用大段代码块，如果需要展示代码请用文件工具发送"。
- **设定行为规则**：例如"如果用户问的问题你不确定，请说明你不确定，不要编造答案"。
- **注入上下文信息**：可以写入群的用途、常见问题等，让 AI 更好地服务特定群聊。

::: warning prompt 越长，每次对话消耗的 token 越多
system prompt 会在每次对话中发送给模型。过长的 prompt 会增加 token 消耗和响应延迟。建议保持在 500 字以内，将详细的指令放到 Skills 中按需加载。
:::

### 使用不同模型处理不同任务

AI 对话插件支持为 OCR（图片识别）配置独立的模型，这是一个实用的省钱策略：

- **对话模型**：选择速度快、性价比高的模型（如 `gpt-4o-mini`、`deepseek-chat`），用于日常文字对话。
- **OCR 模型**：选择视觉能力强的模型（如 `gpt-4o`、`claude-3-5-sonnet`），仅在用户发送图片时调用。

```yaml
# 示例：用便宜模型聊天，用强模型看图
plugin:
  ai_chat_bot:
    model: "deepseek-chat"           # 日常对话用 DeepSeek
    ocr:
      enable: true
      model: "gpt-4o"                # 看图用 GPT-4o
```

::: tip
OCR 配置后，只有当用户发送图片时才会调用视觉模型，文字对话仍然使用主模型，不会额外消耗视觉模型的 token。
:::

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

::: tip 工具是自动调用的
你不需要手动指定使用哪个工具。当你发送"帮我搜一下 XXX"时，模型会自动判断需要调用 `webSearch`；发送"把这段话保存成文件"时，会自动调用 `file` 工具。
:::

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

Skills 是写在 Markdown 文件中的「技能指令」，注入到 AI 的 system prompt 里，引导模型在特定场景下按预定流程行事。与内置工具不同，Skills 不是可执行的代码，而是给模型阅读的「操作手册」。

**为什么需要 Skills？**

内置工具解决的是"能不能做"的问题（搜索、发文件），而 Skills 解决的是"怎么做好"的问题。当你希望 AI 在某个场景下有稳定的、可复现的表现时，用 Skill 来规范它的行为。

**目录结构**：

```
skills/
├── translate/
│   └── SKILL.md
├── summarize/
│   └── SKILL.md
└── code-review/
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
4. 如果原文包含专业术语，在翻译后附上术语解释
```

**工作流程**：

1. 启动时框架自动扫描 `skills_dir` 目录，将所有 Skill 的**名称和描述**注入 system prompt。
2. 当模型判断需要使用某个 Skill 时，会先调用 `skill_read` 工具读取完整指令。
3. 模型按照读取到的指令执行任务。

::: tip Skills 的优势
Skills 采用两阶段加载：启动时只注入名称和描述（节省 token），需要时才加载完整指令。这意味着你可以放很多 Skills 而不用担心占用太多上下文窗口。
:::

**一个更完整的 Skill 示例** — 代码审查：

```markdown
---
name: code-review
description: 对用户发送的代码片段进行审查，指出潜在问题
---

## 指令

1. 确认代码的编程语言
2. 检查以下方面：
   - 语法错误
   - 潜在的 bug 或逻辑错误
   - 性能问题
   - 代码风格和可读性
3. 对每个发现的问题，给出：
   - 问题所在行号
   - 问题描述
   - 修改建议（附代码）
4. 最后给出总体评价（1-10 分）
```

有了这个 Skill，用户只需要 @ 机器人并粘贴代码，AI 就会按照规范的流程进行代码审查，而不是随意发挥。

### 其他特性

- **并发控制**：同一会话同时只处理一个请求，重复 @ 会提示等待
- **自动清理上下文**：群内超过 30 条消息未 @ 机器人时，自动清空该群对话历史
- **Token 统计**：每次对话结束后在日志中输出 prompt / completion / total token 消耗

::: tip 开发自定义工具
如果你需要让 AI 调用自定义的功能（比如查数据库、调用内部 API），可以编写自定义工具并注册到工具系统中。详见[插件开发概览](../plugin/overview.md)。
:::

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

### 有趣的使用场景

复读机是群聊中的「气氛组」。当有人发了一个搞笑的表情包或者一句话戳中了大家的笑点，群友们会不自觉地重复发送同一条消息。当连续出现 3 次时，机器人也会跟着"复读"，形成一种"连机器人也忍不住了"的效果。

常见的玩法：
- 某人发了"哈哈哈"，第二个人也发"哈哈哈"，第三个人再发，机器人也来一句"哈哈哈" —— 氛围瞬间拉满。
- 节日时大家排队发"新年快乐"，机器人混入其中一起祝福。
- 有人发了一个梗图链接，大家纷纷转发，机器人也来凑热闹。

::: tip
复读机的触发阈值是 3 次相同消息连续出现。如果中间穿插了别的消息，计数会重新开始。
:::

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

### 内部工作原理

防撤回的核心思路很简单：**先存下来，撤回了也能看**。

1. **缓存消息**：插件为每个群维护一个容量为 100 条消息的环形队列（`MessageQueue`）。每收到一条群消息，就将其完整内容（包括发送者、时间戳、消息段列表）存入队列。这个操作在 `OnGroupMsg` 的末尾执行，不区分命令和普通消息 —— 所有消息都会被缓存。
2. **撤回时发生了什么**：当有人撤回消息时，QQ 会发送一个撤回通知事件。但此时消息早已被缓存，所以用户仍然可以通过 `/explore` 命令查看到被撤回的内容。
3. **查看历史**：用户发送 `@机器人 /explore` 时，插件从缓存队列中取出最近 N 条消息，以合并转发的形式展示。展示时会根据消息类型做特殊处理：文本、表情、@ 等直接展示；图片和文件会检查链接有效期（3 分钟）；语音消息显示为占位文本。

::: warning 图片和文件的时效性
图片和文件消息有 3 分钟的链接有效期。超过 3 分钟后，即使消息被缓存，图片/文件链接也可能失效，查看时会显示过期提示。
:::

::: tip 内存使用
每个群最多缓存 100 条消息，使用 `sync.Map` 存储，线程安全。消息在内存中不会持久化，机器人重启后缓存清空。
:::

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

**用户命令**：

| 命令 | 说明 |
|------|------|
| `@机器人 /news` | 立即获取新闻（群聊） |
| `/news force` | 管理员强制触发推送（私聊） |

### Cron 表达式速查

AniaBot 使用标准 5 字段 Cron 表达式，格式为 `分 时 日 月 周`：

| 位置 | 字段 | 范围 | 说明 |
|:----:|------|------|------|
| 第 1 位 | 分钟 | 0-59 | 几分执行 |
| 第 2 位 | 小时 | 0-23 | 几点执行 |
| 第 3 位 | 日 | 1-31 | 几号执行 |
| 第 4 位 | 月 | 1-12 | 几月执行 |
| 第 5 位 | 星期 | 0-6 | 周几执行（0=周日） |

常用表达式示例：

| 表达式 | 说明 |
|--------|------|
| `0 8 * * *` | 每天早上 8:00 |
| `0 12 * * *` | 每天中午 12:00 |
| `0 22 * * 1-5` | 工作日晚上 22:00 |
| `30 7 * * 1` | 每周一早上 7:30 |
| `0 9,18 * * *` | 每天 9:00 和 18:00（早晚各一次） |
| `0 12 * * 1,3,5` | 每周一三五中午 12:00 |

::: tip
如果想在一天内多次推送（比如早报和晚报），可以注册两个 `pluginnews` 实例，分别配置不同的 cron 和 groups。或者编写自定义插件，在 `StartCron` 中注册多个定时任务，详见[定时任务文档](../plugin/cron.md)。
:::

::: warning 注意时区
Cron 表达式使用的是服务器的本地时区。如果服务器部署在海外，注意调整时间以匹配北京时间。
:::

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

::: tip 下一步
想编写自己的插件？从[插件开发概览](../plugin/overview.md)开始，或直接跟着[第一个插件](../plugin/first-plugin.md)教程动手实践。
:::
