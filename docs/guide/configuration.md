# 配置说明

AniaBot 使用 YAML 文件管理所有配置，基于 [Viper](https://github.com/spf13/viper) 加载。你只需要在项目根目录放一个 `config.yaml`，框架启动时会自动读取。

## 配置文件加载机制

AniaBot 支持两个配置文件：

| 文件 | 用途 | 说明 |
|------|------|------|
| `config.yaml` | 生产配置 | 默认配置文件，框架一定会尝试加载 |
| `config.dev.yaml` | 开发配置 | 优先加载，不存在时自动回退到 `config.yaml` |

框架启动时的加载顺序：

1. **优先**尝试读取 `config.dev.yaml`
2. 如果 `config.dev.yaml` **不存在**，自动回退读取 `config.yaml`

::: tip 开发与生产环境分离
推荐在本地开发时使用 `config.dev.yaml`，部署到服务器时只保留 `config.yaml`。这样你可以在本地使用测试 API Key、调试端口等配置，而不用每次切换环境都手动改文件。

`config.dev.yaml` 已被添加到 `.gitignore`（或应被添加），不会被提交到代码仓库，避免泄露密钥。
:::

::: warning 不要同时存在两个文件
如果 `config.dev.yaml` 存在，框架**只会**读取它，不会读取 `config.yaml`。如果你修改了 `config.yaml` 但发现没有生效，检查一下是不是 `config.dev.yaml` 在"插队"。
:::

## 完整配置示例

下面是一份带有详细注释的完整配置文件，涵盖了框架运行所需的所有核心配置：

```yaml
# ============================================
# AniaBot 主配置文件
# ============================================

# bot 核心配置
bot:
  # 管理员 QQ 号，拥有最高权限（如执行 /reload 等管理命令）
  admin_id: 123456789
  # 适配器配置 —— 连接到 NapCat 的方式
  adapter:
    ws:
      # NapCat WebSocket Server 地址
      # 需要与 NapCat 配置中的 WebSocket Server 保持一致
      address: ws://localhost:4455
      # 连接失败后的最大重连次数
      # 设为 -1 表示无限重连（适合生产环境，断线后自动恢复）
      # 设为 0 表示不重连（调试时方便查看错误）
      max_retries: 5

  # 存储配置（缓存 + 持久化两层，独立配置、互不影响）
  store:
    # ---- 缓存存储 ----
    # 易失，支持 TTL 与列表语义，适合热数据/临时会话/分布式锁
    cache:
      # 驱动：redis（默认，需 Redis 服务） | memory（进程内内存，零依赖，重启清空）
      driver: redis
      redis:
        address: "localhost:6379"
        password: ""
        # 使用第几号数据库，0~15
        # 建议不同实例使用不同 db，避免数据冲突
        db: 0
      # memory 引擎无需任何配置

    # ---- 持久化存储 ----
    # 重启不丢失，适合需要长期保存的数据（插件配置/用户数据/历史记录等）
    persistent:
      # 驱动：sqlite（默认，纯 Go 无 CGO，零依赖文件数据库） | mysql
      driver: sqlite
      sqlite:
        # 数据库文件路径，目录不存在会自动创建；支持 ":memory:" 内存数据库
        path: "./data/aniabot.db"
      mysql:
        # 标准 go-sql-driver DSN，charset=utf8mb4 建议保留
        dsn: "root:password@tcp(localhost:3306)/aniabot?charset=utf8mb4&parseTime=true&loc=Local"
        max_open_conns: 20          # 最大连接数，0 表示不限
        max_idle_conns: 5           # 最大空闲连接数
        conn_max_lifetime_sec: 300 # 连接最长存活时间（秒）

# ============================================
# 插件配置区域
# 每个插件的 key 对应该插件 Meta 中的 SystemConfigKey
# ============================================
plugin:
  # ---- AI 对话插件 ----
  # 填入你的 OpenAI 兼容 API 信息即可使用
  ai_chat_bot:
    # API 基础地址，支持任何 OpenAI 兼容接口
    # 例如 OpenAI、DeepSeek、本地 Ollama 等
    base_url: "https://api.openai.com/v1"
    model: "gpt-4o-mini"
    api_key: "sk-your-api-key"
    # 单次回复的最大 token 数，越大回复越长但消耗越多
    max_token: 2048
    # 生成温度：0 表示最确定性，1 表示最随机
    # 日常聊天建议 0.7~0.9，严肃任务建议 0.3~0.5
    temperature: 0.7
    # 系统提示词，定义机器人的"人设"
    prompt: "你是一个有趣的 QQ 机器人助手"
    # 图片识别（OCR）子配置
    ocr:
      enable: false
      base_url: "https://api.openai.com/v1"
      model: "gpt-4o"
      api_key: "sk-your-ocr-api-key"

  # ---- 每日新闻插件 ----
  dailyNews:
    # 新闻图片 API 地址
    api: "https://uapis.cn/api/v1/daily/news-image"
    # Cron 表达式，控制每日推送时间
    # "0 12 * * *" 表示每天中午 12:00
    # "30 8 * * 1-5" 表示工作日早上 8:30
    cron: "0 12 * * *"
    # 接收新闻推送的群号列表
    groups:
      - 123456789
      - 987654321

  # ---- 自定义插件配置示例 ----
  # key 名称需要与你的插件代码中读取的路径一致
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
| `store` | object | 存储配置（缓存 + 持久化两层），详见下方 [`bot.store`](#bot-store) |

### `bot.store`

存储配置分为缓存（cache）与持久化（persistent）两层，二者独立配置、互不影响：

- **cache（缓存层）**：易失存储，支持 TTL 与列表语义，适合热数据、临时会话、分布式锁。
- **persistent（持久化层）**：重启不丢失，适合需要长期保存的数据（插件配置、用户数据、历史记录等）。

::: warning 破坏性变更
旧版本的 `bot.store.redis`（字符串地址）、`bot.store.password`、`bot.store.db` 三个字段已废弃。请迁移到下方新的 `bot.store.cache` / `bot.store.persistent` 结构。
:::

#### `bot.store.cache`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `driver` | string | `redis` | 缓存驱动：`redis`（需 Redis 服务）或 `memory`（进程内内存，零依赖，重启清空） |
| `redis.address` | string | `localhost:6379` | Redis 地址（driver 为 `redis` 时生效） |
| `redis.password` | string | `""` | Redis 密码 |
| `redis.db` | int | `0` | Redis 数据库编号，0~15 |

::: warning 缓存默认不会自动降级
默认使用 Redis 作为缓存层。若 Redis 不可达，框架会记录错误并退出（**不会**自动降级为内存存储）。如需零依赖、免安装的本地缓存，显式设置 `bot.store.cache.driver: memory`，但重启后缓存数据会清空，且多实例之间不共享。
:::

#### `bot.store.persistent`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `driver` | string | `sqlite` | 持久化驱动：`sqlite`（纯 Go 无 CGO）或 `mysql` |
| `sqlite.path` | string | `./data/aniabot.db` | SQLite 数据库文件路径，目录不存在会自动创建；支持 `:memory:` 内存数据库 |
| `mysql.dsn` | string | — | 标准 go-sql-driver DSN，如 `root:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=true&loc=Local` |
| `mysql.max_open_conns` | int | `20` | 最大连接数，0 表示不限 |
| `mysql.max_idle_conns` | int | `5` | 最大空闲连接数 |
| `mysql.conn_max_lifetime_sec` | int | `300` | 连接最长存活时间（秒） |

::: tip 零依赖持久化
默认使用 SQLite 作为持久化层，纯 Go 驱动 `modernc.org/sqlite` 无需 CGO、无需额外安装数据库服务，开箱即用。需要多实例共享或更高吞吐时，切换 `driver: mysql` 即可。
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

插件通过 `Start` 生命周期方法接收全局 `*viper.Viper` 实例。框架在调用 `Start` 之前已经完成了配置文件的加载，你只需要用对应的 key 去读取即可。

```go
func (p *YourPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    // 用 GetString、GetInt、GetBool 等方法读取配置
    // 路径格式统一为: plugin.<你的插件key>.<字段名>
    p.apiKey = cfg.GetString("plugin.yourplugin.api_key")
    p.timeout = cfg.GetInt("plugin.yourplugin.timeout")
    return nil
}
```

::: tip 配置路径规则
配置路径的格式是 `plugin.<插件key>.<字段名>`，其中 `<插件key>` 就是 YAML 文件中 `plugin:` 下面那一层的 key 名称。比如 YAML 中写的 `plugin.ai_chat_bot`，读取时就用 `cfg.GetString("plugin.ai_chat_bot.model")`。
:::

### 实战示例：拦截器插件

下面是一个真实案例 —— 自定义的拦截器插件（`custom/plugins/interceptor`），它通过配置文件管理黑名单和白名单：

**YAML 配置：**

```yaml
plugin:
  interceptor:
    # 黑名单模式：拦截指定群/用户，其余放行
    # 适用于：想屏蔽少数吵闹的群或用户
    blacklist:
      groups:
        - "111111111"   # 某个吵闹的群
        - "222222222"
      users:
        - "333333333"   # 某个刷屏的用户
    # 白名单模式：只放行指定群/用户，其余全部拦截
    # 适用于：只想让机器人在特定群/用户中工作
    # 一般不与黑名单同时使用
    whitelist:
      groups: []
      users: []
```

**Go 代码读取：**

```go
func (p *InterceptorPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    // 读取黑名单中的群号列表
    p.interceptGroup = cfg.GetStringSlice("plugin.interceptor.blacklist.groups")
    // 读取黑名单中的用户列表
    p.interceptUser = cfg.GetStringSlice("plugin.interceptor.blacklist.users")

    // 读取白名单中的群号列表
    p.permitGroup = cfg.GetStringSlice("plugin.interceptor.whitelist.groups")
    // 读取白名单中的用户列表
    p.permitUser = cfg.GetStringSlice("plugin.interceptor.whitelist.users")

    return nil
}
```

::: warning 黑白名单是两种模式
拦截器的默认行为是**拦截所有消息**。黑白名单是两种互斥的使用模式：

- **黑名单模式**：`blacklist` 填入要屏蔽的群/用户，`whitelist` 留空 → 被列出的群/用户被拦截，其余放行
- **白名单模式**：`whitelist` 填入要放行的群/用户，`blacklist` 留空 → 被列出的群/用户被放行，其余全部拦截

一般**不同时使用**。如果同时配置，黑名单先生效（命中即拦截），白名单仅对未命中黑名单的条目生效。
:::

::: info 常用的 Viper 读取方法
| 方法 | 返回类型 | 适用场景 |
|------|----------|----------|
| `cfg.GetString(key)` | string | 字符串配置 |
| `cfg.GetInt(key)` | int | 数字配置 |
| `cfg.GetBool(key)` | bool | 开关配置 |
| `cfg.GetStringSlice(key)` | []string | 字符串列表（如群号列表） |
| `cfg.GetIntSlice(key)` | []int | 数字列表 |
| `cfg.GetDuration(key)` | time.Duration | 时间间隔（如 `"30s"`） |
:::

## 环境配置建议

### 本地开发

创建 `config.dev.yaml`，填入测试用的 API Key 和本地服务地址：

```yaml
bot:
  admin_id: 987654321
  adapter:
    ws:
      address: ws://localhost:4455
      max_retries: 0   # 开发时不重连，方便看到错误
  store:
    cache:
      driver: memory   # 本地开发免装 Redis，用进程内内存（重启清空）
    persistent:
      driver: sqlite   # SQLite 零依赖，文件落盘

plugin:
  ai_chat_bot:
    base_url: "http://localhost:11434/v1"  # 本地 Ollama
    model: "qwen2.5"
    api_key: "ollama"
    temperature: 0.9
    prompt: "你是一个测试用的机器人"
```

### 生产部署

使用 `config.yaml`，填入正式的 API Key，开启重连，并配置生产级存储（Redis 缓存 + SQLite/MySQL 持久化）：

```yaml
bot:
  admin_id: 123456789
  adapter:
    ws:
      address: ws://localhost:4455
      max_retries: -1  # 生产环境无限重连
  store:
    cache:
      driver: redis   # 生产环境使用 Redis 作为缓存
      redis:
        address: "localhost:6379"
        password: "your-redis-password"
        db: 0
    persistent:
      driver: sqlite  # 持久化层（生产高吞吐可切换 mysql）
      sqlite:
        path: "./data/aniabot.db"

plugin:
  ai_chat_bot:
    base_url: "https://api.openai.com/v1"
    model: "gpt-4o-mini"
    api_key: "sk-production-key"
    temperature: 0.7
    prompt: "你是一个有用的 QQ 机器人助手"
```

::: warning 保护你的配置文件
`config.yaml` 和 `config.dev.yaml` 包含 API 密钥等敏感信息，**不要**提交到 Git 仓库。框架已内置保护机制，禁止通过机器人发送这两个文件。请确保 `.gitignore` 中包含这两个文件。
:::
