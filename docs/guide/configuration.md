# 配置说明

AniaBot 使用 `config.yaml` 作为统一配置文件，基于 [Viper](https://github.com/spf13/viper) 管理。

## 完整配置示例

```yaml
# bot 核心配置
bot:
  admin_id: 123456789       # 管理员 QQ 号
  adapter:
    ws:
      address: ws://localhost:4455  # napcat WebSocket Server 地址
      max_retries: 5                # 连接失败最大重连次数

# Redis 配置（用于插件持久化存储，可选）
redis:
  addr: "localhost:6379"
  password: ""
  db: 0

# 插件配置
plugin:
  # AI 对话插件
  ai_chat_bot:
    base_url: "https://api.openai.com/v1"
    model: "gpt-4o-mini"
    api_key: "sk-your-api-key"
    max_token: 2048
    temperature: 0.7
    prompt: "你是一个有趣的 QQ 机器人助手"
    ocr:
      enable: false
      base_url: "https://api.openai.com/v1"
      model: "gpt-4o"
      api_key: "sk-your-ocr-api-key"

  # 每日新闻插件
  dailyNews:
    api: "https://uapis.cn/api/v1/daily/news-image"
    cron: "0 12 * * *"   # 每天 12:00 推送
    groups:
      - 123456789
      - 987654321

  # 自定义插件配置（示例）
  yourplugin:
    api_key: "your-key"
    timeout: 30
```

## 配置项说明

### `bot`

| 字段 | 类型 | 说明 |
|------|------|------|
| `admin_id` | int | 管理员 QQ 号，用于权限控制 |
| `adapter.ws.address` | string | napcat WebSocket Server 地址 |
| `adapter.ws.max_retries` | int | 断线重连最大次数，`-1` 为无限重连 |

### `redis`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `addr` | string | `localhost:6379` | Redis 地址 |
| `password` | string | `""` | Redis 密码 |
| `db` | int | `0` | Redis 数据库编号 |

::: info 不配置 Redis
若不配置 Redis，框架会自动使用内存存储引擎（重启后数据清空）。
:::

### `plugin.ai_chat_bot`

| 字段 | 类型 | 说明 |
|------|------|------|
| `base_url` | string | OpenAI 兼容接口地址 |
| `model` | string | 使用的模型名称 |
| `api_key` | string | API 密钥 |
| `max_token` | int | 单次最大 token 数 |
| `temperature` | float | 生成温度，0~1 |
| `prompt` | string | 系统提示词 |
| `ocr.enable` | bool | 是否启用图片识别 |

### `plugin.dailyNews`

| 字段 | 类型 | 说明 |
|------|------|------|
| `api` | string | 新闻图片 API 地址 |
| `cron` | string | Cron 表达式，控制推送时间 |
| `groups` | []int | 接收推送的群号列表 |

## 在插件中读取配置

插件通过 `Start` 生命周期方法接收全局 `*viper.Viper` 实例：

```go
func (p *YourPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    p.apiKey = cfg.GetString("plugin.yourplugin.api_key")
    p.timeout = cfg.GetInt("plugin.yourplugin.timeout")
    return nil
}
```
