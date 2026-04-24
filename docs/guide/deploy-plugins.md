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

**触发方式**：`/acg`

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/acgwallpaper"

bot.AddPlugin(acgwallpaper.NewAcgWallpaperPlugin(5)) // 参数为最大并发工作数
```

---

## waifu.pics (waifupics)

从 [waifu.pics](https://waifu.pics) 获取 waifu / neko 等二次元图片，支持多种类别。

**触发方式**：`@机器人 /waifu [类别]`，发送 `/waifu help` 查看所有类别

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/waifupics"

bot.AddPlugin(waifupics.NewWaifuPlugin(5))
```

---

## GD 音乐 (gdmusicplugin)

音乐搜索与发送，支持关键词搜索、翻页、按序号发送。

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

**触发方式**：发送包含 `https://v.douyin.com/...` 的消息，或 `/douyin [分享内容]`

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/douyinparser"

bot.AddPlugin(douyinparser.Newplugin())
```

---

## GitHub 仓库分析 (githubrepoer)

输入 GitHub 仓库链接，由 AI 自动生成项目分析报告（README 摘要、技术栈、亮点等）。

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

**触发方式**：私聊发送「伪造记录」开始创建流程

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/chatrecordsmaker"

bot.AddPlugin(chatrecordsmaker.NewChatRecordsMakerPlugin())
```

---

## 随机活跃 (activeman)

模拟真人活跃行为，按概率随机在群内点赞、戳一戳、签到，每分钟触发一次检测。

**触发方式**：自动定时触发，无需命令

**注册方式**：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/activeman"

// 三个参数分别为：点赞概率、戳一戳概率、签到概率（0.0~1.0）
bot.AddPlugin(activeman.NewActiveMan(0.1, 0.05, 0.3))
```

---

## 消息拦截器 (interceptor)

通过黑白名单配置，拦截或放行指定群聊/用户的消息，减少无关消息干扰。该插件 `Order` 极低，在其他插件之前执行。

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
    blacklist:
      groups:
        - "123456789"   # 拦截该群的所有消息
      users:
        - "987654321"   # 拦截该用户的所有消息
    whitelist:
      groups:
        - "111111111"   # 仅放行该群（其余全拦截）
      users: []
```

::: tip 黑白名单优先级
黑名单优先于白名单。若同时配置了黑白名单，则按照拦截黑名单->处理白名单->拒绝流程处理。
:::
