# 常见问题

## 连接与启动

### 启动后机器人收不到消息？

按以下顺序排查：

1. **NapCat 是否已登录**：确认 NapCat 端 QQ 登录成功且在线
2. **适配器类型是否匹配**：`bot.adapter.mode` 为 `ws`（WebSocket）时，NapCat 就必须开启 WebSocket 服务端；为 `http` 时则对应 HTTP 服务端 + 客户端上报
3. **地址是否正确**：默认 `ws://localhost:4455`，Docker 环境不能用 `localhost`，改用内网 IP
4. **access token**：NapCat 端设置了 token 时，在面板「配置管理」中填写 `bot.adapter.token`（重启生效）

### WebSocket 一直重连失败？

- 连接失败会**无限重试**（指数退避，封顶 30 秒），连接状态与最近错误可在 Web 面板查看；修改连接配置后重启生效
- 检查 NapCat 的 WebSocket 服务端是否监听在配置的端口
- Windows 上检查防火墙是否拦截

### 提示「配置读取错误」或插件初始化失败？

配置存于数据库，通过面板修改后**需要重启才生效**——确认改完后重启过 Bot。插件读取不到自己的配置键时会在 `Start()` 返回错误并记录日志；可在面板「配置管理 → 高级模式 (JSON)」中检查键名是否正确（点分路径，大小写不敏感）。

## AI 对话

### @机器人没有反应？

1. 确认 `plugin.ai_chat_bot.api_key` 已填写且有效
2. 查看控制台日志，是否有 API 请求错误（余额不足、模型名错误等）
3. AI 插件 Order = 1000，最后执行 —— 如果某个前置插件返回了 `false`（阻断传播），AI 就收不到消息
4. 同一会话已有请求在进行时，新消息会进入排队队列，待当前响应结束后自动合并回复

### AI 不认识图片？

- 主模型支持多模态：将 `multimodal: true`，AI 会通过 `load_images` 工具按需加载图片
- 主模型不支持多模态：配置 `ocr` 备用视觉模型，图片会先转述为文字
- 两者都没配置时，AI 会明确告知无法查看图片

### 对话历史能保存多久？

历史持久化在 `PersistentStorage`（默认 SQLite），重启不丢。超过 `max_context_tokens` 的 80% 时会自动让 LLM 总结压缩。发送带 `#新对话` 的消息可手动清空。

### AI 创建的定时任务存在哪？

clock 任务持久化在 `PersistentStorage` 的 `clock:` 命名空间下，重启后自动恢复调度。用 `/clock list` 查看，`/clock del <id>` 删除。

## 存储

### 不想装 Redis 可以吗？

可以，默认就是。缓存驱动默认为 `memory`（进程内内存，零依赖）。需要多实例共享缓存时再把 `bot.store.cache.driver` 改为 `redis`。

### 数据文件在哪？

默认 SQLite 持久化数据在 `./data/aniabot.db`（环境变量 `ANIABOT_SQLITE_PATH` 可改）。目录不存在会自动创建。

### 换 MySQL 怎么配？

通过环境变量切换持久化驱动（详见 [配置详解](/guide/configuration#引导配置-环境变量)）：

```bash
export ANIABOT_STORE_DRIVER=mysql
export ANIABOT_MYSQL_DSN="root:password@tcp(host:3306)/aniabot?charset=utf8mb4&parseTime=true&loc=Local"
```

## 插件开发

### 我的插件不触发？

1. 确认在 `cmd/main.go` 中 `bot.AddPlugin()` 注册过
2. 检查是否有高优先级（Order 更小）插件返回了 `false` 阻断传播
3. 命令必须**以 `/` 开头**才会进入 `cmd.Name` 解析；群聊命令还需 @ 机器人（`cmd.Mention == true`）

### 插件 panic 会拖垮整个机器人吗？

不会。框架对每个插件调用都有 `safeExecute` 包裹，panic 会被捕获并触发所有插件的 `OnPanic` 回调（系统插件默认私聊通知管理员）。通过 `bot.Go(name, f)` 启动的协程同样有崩溃恢复。

### 如何调试插件？

保留 `pluginlog` 日志插件，控制台会打印每条收发消息；单元测试参考 `bot/utils` 与各插件的 `_test.go`：

```bash
go test ./...
go test -v -race ./...
```

## 部署

### 交叉编译报错 CGO？

AniaBot 全部存储后端均为纯 Go 驱动（`modernc.org/sqlite`、`go-sql-driver/mysql`），默认无 CGO 依赖。直接用 Makefile：

```bash
make linux     # GOOS=linux GOARCH=amd64
make windows
```

### 如何更新文档？

文档源码在 `docs/`（VitePress），推送到 `main` 分支后 GitHub Actions 自动构建发布到 `https://jeanhua.github.io/AniaBot/`。本地预览：

```bash
cd docs
npm install
npm run docs:dev
```
