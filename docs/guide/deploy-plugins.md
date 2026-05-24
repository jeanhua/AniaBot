# 部署分支插件

[dev/deploy 分支](https://github.com/jeanhua/AniaBot/tree/dev/deploy) 包含一系列由作者开发的插件示例，可直接用于生产部署，也可作为自定义插件的参考实现。

## 插件列表

| 插件 | 命令 | 场景 |
|------|------|------|
| [二次元壁纸](#二次元壁纸-acgwallpaper) | `/acg` | 群聊 / 私聊 |
| [waifu.pics](#waifupics-waifupics) | `/waifu` | 群聊 / 私聊 |
| [GD 音乐](#gd-音乐-gdmusicplugin) | `/music` | 群聊 / 私聊 |
| [抖音解析](#抖音视频解析-douyinparser) | `/douyin` 或自动识别 | 群聊 / 私聊 |
| [GitHub 仓库分析](#github-仓库分析-githubrepoer) | `@机器人 /gr` | 群聊 / 私聊 |
| [URL 解析](#url-解析-urlparser) | 自动识别 URL | 群聊 |
| [群刊](#群刊-groupnewsletter) | `/gn` | 群聊 |
| [聊天记录伪造](#聊天记录伪造-chatrecordsmaker) | `伪造记录` | 私聊 |
| [随机活跃](#随机活跃-activeman) | 自动触发 | 群聊 |
| [消息拦截器](#消息拦截器-interceptor) | 配置驱动 | 全局 |

---

## 二次元壁纸 (acgwallpaper)

随机获取二次元壁纸图片，支持群聊和私聊。

**设计模式**：该插件采用 **channel 任务队列** 模式——使用带缓冲的 channel 作为工作队列，用户请求入队后由后台 goroutine 池消费处理。构造函数接收的并发参数控制 worker 数量，避免同时发起过多 HTTP 请求。这种模式适合「请求量大但处理耗时」的场景，既限流又不阻塞消息处理。

**触发方式**：`/acg`

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/acgwallpaper"

bot.AddPlugin(acgwallpaper.NewAcgWallpaperPlugin(5)) // 参数为最大并发工作数
```

---

## waifu.pics (waifupics)

从 [waifu.pics](https://waifu.pics) 获取 waifu / neko 等二次元图片，支持 **31 种类别**（如 waifu、neko、shinobu、megumin、bully、cuddle、cry、hug、awoo、kiss、lick、pat、smug、bonk、yeet、blush、smile、wave、highfive、handhold、nom、bite、glomp、slap、kill、kick、happy、wink、poke、dance、cringe 等），发送 `/waifu help` 可查看完整列表。

**设计模式**：与 acgwallpaper 相同，采用 **channel 任务队列** 模式，后台 goroutine 池消费请求队列，确保并发可控。

**触发方式**：`@机器人 /waifu [类别]`，发送 `/waifu help` 查看所有类别

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/waifupics"

bot.AddPlugin(waifupics.NewWaifuPlugin(5))
```

---

## GD 音乐 (gdmusicplugin)

音乐搜索与发送，支持关键词搜索、翻页、按序号发送。

**设计模式**：该插件的翻页功能基于 **`sync.Map` 的分群会话缓存**——每个群维护独立的搜索结果缓存和当前页码，用户发送 `/music next` 或 `/music prev` 时从缓存中读取对应页数据，无需重复请求 API。此外，API 客户端层采用 **functional options 模式** 构建，通过 `WithXxx()` 函数链式配置 base URL、超时等参数，对外保持简洁的构造接口。

**命令**：

| 命令 | 说明 |
|------|------|
| `/music [关键词]` | 搜索音乐 |
| `/music get [序号]` | 发送指定序号的歌曲 |
| `/music next` / `/music prev` | 翻页 |
| `/music help` | 查看帮助 |

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/gdmusicplugin"

bot.AddPlugin(gdmusicplugin.NewMusicPlugin())
```

**配置**（可选，有默认值）：

```yaml
plugin:
  gdmusic:
    base_url: "https://example.com/api"  # 自定义音乐 API 地址
```

---

## 抖音视频解析 (douyinparser)

自动识别消息中的抖音分享链接，解析并发送无水印视频。

**设计模式**：该插件使用 **正则表达式提取 URL** 模式——通过预编译的正则从用户消息中匹配 `v.douyin.com` 短链接，匹配成功后走解析流程，否则直接跳过。正则在包级别预编译（`regexp.MustCompile`），避免每次消息到来时重复编译的开销。

**触发方式**：发送包含 `https://v.douyin.com/...` 的消息，或 `/douyin [分享内容]`

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/douyinparser"

bot.AddPlugin(douyinparser.Newplugin())
```

---

## GitHub 仓库分析 (githubrepoer)

输入 GitHub 仓库链接，由 AI 自动生成项目分析报告（README 摘要、技术栈、亮点等）。

**设计模式**：该插件展示了 **LLM 集成** 的典型流程——接收用户输入，构造 prompt（可自定义），调用 OpenAI 兼容 API 获取分析结果。同时实现了 **Markdown 转图片** 功能：将 LLM 返回的 Markdown 格式报告渲染为图片后发送，避免 QQ 对纯文本排版的限制。这种「LLM 生成 + 格式转换 + 消息发送」的三段式架构在内容生成类插件中非常实用。

**触发方式**：`@机器人 /gr [GitHub 仓库链接]`，发送 `/gr help` 查看参数详情

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/githubrepoer"

bot.AddPlugin(githubrepoer.NewGithubRepoer(3))
```

**配置**：

```yaml
plugin:
  github_repoer:
    model:
      base_url: "https://api.openai.com/v1"
      api_key: "sk-your-key"
      model: "gpt-4o-mini"
      prompt: ""  # 留空使用默认 prompt
```

---

## URL 解析 (urlparser)

自动识别群聊中的 URL，调用 Jina Reader 抓取页面内容，再由 AI 提炼摘要发送到群内。

**设计模式**：该插件集成了 **Jina Reader** 作为网页内容提取服务（`https://r.jina.ai/[url]`），将任意网页转为结构化文本。同时使用 **Redis 缓存** 模式——对已解析的 URL 结果设置 TTL 缓存，相同链接在缓存有效期内直接返回缓存结果，避免重复抓取和 LLM 调用。缓存 key 基于 URL 生成，TTL 通过配置文件控制。

**触发方式**：自动识别，无需命令

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/urlparser"

bot.AddPlugin(urlparser.NewURLParserPlugin(5))
```

**配置**：

```yaml
plugin:
  url_parser:
    token: "your-jina-token"   # Jina Reader API Token（必填）
    cache_ttl: 60              # 相同 URL 缓存时间（秒），默认 60
    llm:
      base_url: "https://api.openai.com/v1"
      api_key: "sk-your-key"
      model: "gpt-4o-mini"
```

---

## 群刊 (groupnewsletter)

自动收集群内消息，达到阈值后由 AI 生成一期「群刊」——以幽默风格总结群内热点话题和精彩发言。

**设计模式**：这是部署分支中 **最复杂的插件**，包含约 8 个源文件，展示了完整的「数据收集 - 持久化 - 内容生成 - 触发控制」架构：

- **消息收集**：每条群消息经过滤后追加到内存 buffer
- **异步 Redis 持久化**：buffer 定期刷新到 Redis，防止进程重启丢失数据，写入操作在独立 goroutine 中异步执行，不阻塞消息处理
- **LLM 内容生成**：达到消息阈值后，将收集的消息作为上下文交给 LLM 生成群刊内容
- **阈值触发机制**：通过 `msg_threshold` 控制自动触发时机，也可通过 `/gn gen` 手动触发

如果你需要实现「数据采集 + 周期性内容生成」类插件，群刊是最佳参考。

**命令**：

| 命令 | 说明 |
|------|------|
| `/gn` | 查看当前消息收集状态 |
| `/gn gen` | 立即生成本期群刊 |

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/groupnewsletter"

bot.AddPlugin(groupnewsletter.NewGroupNewsletterPlugin())
```

**配置**：

```yaml
plugin:
  group_newsletter:
    enabled_groups:
      - 123456789     # 启用群刊的群号
    msg_threshold: 100  # 触发自动生成的消息数，默认 100
    max_messages: 500   # buffer 最多保留消息数，默认 500
    fmt: "md"           # 输出格式
    model:
      base_url: "https://api.openai.com/v1"
      api_key: "sk-your-key"
      model: "gpt-4o"
      prompt: ""        # 留空使用内置 prompt
```

---

## 聊天记录伪造 (chatrecordsmaker)

通过私聊交互，伪造合并转发格式的聊天记录。

**设计模式**：该插件实现了 **交互式多步骤会话** 模式——使用 `map[QID]*session` 为每个用户维护独立的创建会话状态。用户发送「伪造记录」后进入多轮对话流程（设定头像、昵称、消息内容等），每步输入更新会话状态，最终组装为合并转发消息。这种「状态机 + 用户会话映射」的模式适合所有需要多轮交互引导的插件场景。

**触发方式**：私聊发送「伪造记录」开始创建流程

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/chatrecordsmaker"

bot.AddPlugin(chatrecordsmaker.NewChatRecordsMakerPlugin())
```

---

## 随机活跃 (activeman)

模拟真人活跃行为，按概率随机在群内点赞、戳一戳、签到，每分钟触发一次检测。

**设计模式**：该插件结合了 **cron 定时任务** 和 **概率触发** 模式——通过框架的 cron 机制每分钟执行一次检测，每次检测时按配置的概率（0.0~1.0）决定是否执行对应动作。使用 `atomic.Int64` 实现 **原子计数器限流**，防止短时间内频繁执行同类操作。三个概率参数独立控制点赞、戳一戳、签到的触发频率，模拟自然的活跃行为。

**触发方式**：自动定时触发，无需命令

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/activeman"

// 三个参数分别为：点赞概率、戳一戳概率、签到概率（0.0~1.0）
bot.AddPlugin(activeman.NewActiveMan(0.1, 0.05, 0.3))
```

---

## 消息拦截器 (interceptor)

通过黑白名单配置，拦截或放行指定群聊/用户的消息，减少无关消息干扰。

**设计模式**：该插件的关键特征是 **高优先级 Order 设置**——使用 `LevelLog + 1`（即 -999）作为 Order 值，使其在所有业务插件之前执行。在 `OnGroupMsg` 中返回 `(false, nil)` 即可中断后续插件链的传播。这是实现「前置过滤器」的标准模式：越早执行的插件 Order 越小，用于做全局性的拦截、日志记录、速率限制等前置处理。

**触发方式**：配置驱动，自动生效，无命令

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/interceptor"

bot.AddPlugin(interceptor.NewInterceptorPlugin())
```

**配置**：

```yaml
plugin:
  interceptor:
    # 黑名单模式：屏蔽指定群/用户，其余放行
    blacklist:
      groups:
        - "123456789"   # 拦截该群的所有消息
      users:
        - "987654321"   # 拦截该用户的所有消息
    # 白名单模式：只放行指定群/用户，其余全部拦截
    # 一般不与黑名单同时使用
    whitelist:
      groups: []
      users: []
```

::: tip 两种使用模式
拦截器的默认行为是拦截所有消息。黑白名单是两种互斥的使用模式：

- **黑名单模式**：`blacklist` 填入要屏蔽的群/用户，`whitelist` 留空 → 被列出的被拦截，其余放行
- **白名单模式**：`whitelist` 填入要放行的群/用户，`blacklist` 留空 → 被列出的被放行，其余全部拦截

如果同时配置，黑名单先生效（命中即拦截），白名单仅对未命中黑名单的条目生效。
:::

---

## 从这些插件中学什么

下表将上述插件中使用的核心设计模式映射到对应的文档页面，方便按需查阅：

| 设计模式 | 示例插件 | 文档 |
|----------|----------|------|
| Channel 任务队列（带缓冲 channel + goroutine 池） | acgwallpaper、waifupics | [常见模式](/plugin/patterns) |
| LLM 集成（OpenAI 兼容 API 调用、prompt 构造） | githubrepoer、urlparser、groupnewsletter | [常见模式](/plugin/patterns) |
| Redis 缓存（TTL 缓存、异步持久化） | urlparser、groupnewsletter | [数据存储](/plugin/storage) |
| Cron 定时任务（周期性触发、概率执行） | activeman | [定时任务](/plugin/cron) |
| 交互式多步骤会话（用户状态机） | chatrecordsmaker | [常见模式](/plugin/patterns) |
| 高优先级 Order（前置过滤器） | interceptor | [插件概览](/plugin/overview) |
| sync.Map 分群缓存（翻页会话管理） | gdmusicplugin | [常见模式](/plugin/patterns) |
| Functional Options 模式 | gdmusicplugin | [常见模式](/plugin/patterns) |
| 正则提取 + 预编译 | douyinparser | [常见模式](/plugin/patterns) |
| Markdown 转图片 | githubrepoer | [常见模式](/plugin/patterns) |
