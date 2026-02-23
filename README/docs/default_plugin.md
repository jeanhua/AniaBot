## 系统内置插件介绍

**AniaBot** 内置了多个实用的系统插件，开箱即用，为开发者提供了丰富的功能参考。

### 1. 日志打印插件 (pluginlog)

**功能描述**：在控制台打印所有接收到的消息，便于调试和监控。

**主要特性**：
- 群聊和私聊消息日志记录
- 显示发送者昵称和用户ID

**使用方式**：

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"

bot.AddPlugin(pluginlog.NewPlugin())
```

### 2. 复读机插件 (pluginrepeat)

**功能描述**：自动复读群聊中重复出现的消息，增加群聊趣味性。

**主要特性**：
- 当同一条消息连续出现3次时自动复读
- 支持管理员控制开关
- 基于群聊的独立计数

**控制命令**：
- `@机器人 /close repeat` - 关闭复读机（管理员）
- `@机器人 /enable repeat` - 开启复读机（管理员）

**使用方式**：
```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"

bot.AddPlugin(pluginrepeat.NewPlugin())
```

### 3. AI对话插件 (pluginaichat)

**功能描述**：集成AI聊天功能，支持文本和图片内容理解。

**主要特性**：
- 支持多种AI模型接口（OpenAI兼容）
- 图片OCR识别功能
- 上下文记忆和并发控制
- 超时处理和错误恢复

**触发方式**：
- 在群聊或私聊中@机器人即可开始对话

**配置示例**（在config.yaml中）：
```yaml
plugin:
  ai_chat_bot:
    base_url: "https://api.openai.com/v1"
    model: "gpt-3.5-turbo"
    api_key: "your-api-key"
    max_token: 2048
    temperature: 0.7
    prompt: "你是一个AI助手"
    ocr:
      enable: true
      base_url: "https://api.openai.com/v1"
      model: "gpt-4-vision-preview"
      api_key: "your-ocr-api-key"
```

**使用方式**：
```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"

bot.AddPlugin(pluginaichat.NewAIChatPlugin())
```

### 4. 防撤回插件 (pluginantiwithdrawal)

**功能描述**：记录群聊消息，防止消息被撤回后无法查看。

**主要特性**：
- 缓存最近的100条群聊消息
- 支持消息回顾功能
- 转发消息格式优化

**使用命令**：

在群聊中

- `@机器人 /explore [数量]` - 查看最近的消息（默认50条，最多100条）

在私聊中(仅管理员)

- `@机器人 /explore [群号] [数量]` - 查看某个群最近的消息（默认50条，最多100条）

**使用方式**：

```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"

bot.AddPlugin(pluginantiwithdrawal.NewPlugin())
```

### 5. 新闻推送插件 (pluginnews)

**功能描述**：定期抓取并推送来自配置来源的新闻、头条相关资讯到群聊，支持订阅/退订和关键词过滤。

**配置示例**（在 `config.yaml` 中）：

```yaml
plugin:
  # 每日新闻插件
  dailyNews:
    api: "https://uapis.cn/api/v1/daily/news-image" # api端点
    cron: "0 12 * * *" # cron表达式, 每天12点触发
    groups: # 指定需要定时播报的群聊
      - 123456
      - 7891011
```

**使用方式**：
```go
import "github.com/jeanhua/AniaBot/bot/plugins/pluginnews"

bot.AddPlugin(pluginnews.NewPlugin())
```

### 系统插件注册示例

在 `cmd/main.go` 中注册所有系统插件：

```go
import (
    "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"
)

func main() {
    adapter := napcat.NewNapcatWebSocketAdapter()
    bot := aniabot.NewAniaBot(adapter)
    
    // 注册系统插件
    bot.AddPlugin(pluginlog.NewPlugin())           // 日志插件
    bot.AddPlugin(pluginrepeat.NewPlugin())        // 复读机插件
    bot.AddPlugin(pluginaichat.NewAIChatPlugin())  // AI对话插件
    bot.AddPlugin(pluginantiwithdrawal.NewPlugin()) // 防撤回插件
    bot.AddPlugin(pluginnews.NewPlugin())			// 每日新闻插件
    
    bot.Run()
}
```
