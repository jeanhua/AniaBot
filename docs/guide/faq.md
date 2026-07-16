# 常见问题

## 安装与配置

### napcat 连接失败

**症状**：启动后看到 `connection refused` 或反复重连日志。

**排查步骤**：
1. 确认 napcat 已启动且 WebSocket Server 已开启
2. 检查 `config.yaml` 中的 `address` 是否与 napcat 配置一致
3. 如果 napcat 在另一台机器上，确保防火墙放行了对应端口
4. 尝试用浏览器访问 napcat 的 HTTP 管理页面确认服务正常

**常见错误**：
- `ws://localhost:4455` 写成了 `http://localhost:4455`
- napcat 的 WebSocket 端口不是 4455
- napcat 还没启动就读取了配置

### Redis 连接失败

**症状**：启动时 panic 提示 Redis 连接失败。

**解决方案**：
- **方案 A**：确保 Redis 已启动，`config.yaml` 中的 `bot.store.cache.redis` 配置正确
- **方案 B**：无需 Redis 时，设置 `bot.store.cache.driver: memory`，改用进程内内存缓存（重启后清空，适合开发测试）

::: tip 持久化存储默认即开即用
框架默认使用 SQLite 作为持久化层（数据文件位于 `./data/aniabot.db`），无需安装任何外部数据库。Redis 只承担缓存职责，不是持久化存储。
:::

### 配置文件不生效

**原因**：AniaBot 的配置加载顺序是：优先 `config.dev.yaml`，如果不存在才读 `config.yaml`。

**解决方案**：
- 开发时用 `config.dev.yaml`，生产环境用 `config.yaml`
- 确保你编辑的是正确的文件
- YAML 格式对缩进敏感，使用空格而不是 Tab

### Go 版本不满足要求

**症状**：编译报错，提示语法不支持。

**解决方案**：AniaBot 需要 Go 1.26.0 或更高版本。运行 `go version` 检查当前版本。

---

## 插件开发

### 插件消息没有被触发

**排查步骤**：
1. **检查 Order**：是否有更高优先级（更小 Order）的插件返回了 `false` 阻止了消息传递？
2. **检查命令名**：`cmd.Name` 不包含 `/`，`/hello` 的 `cmd.Name` 是 `hello`
3. **检查 ShowFor**：群聊消息需要 `ShowFor` 包含 `ShowForGroup`，私聊需要 `ShowForFriend`
4. **注册了吗？**：确认在 `cmd/main.go` 中调用了 `bot.AddPlugin()`
5. **用 pluginlog 调试**：注册 `pluginlog` 插件，确认消息确实到达了框架

### Storage 数据读不到

**可能原因**：
1. **Name 被修改了**：插件的 `Name` 字段参与存储前缀生成，修改后旧数据无法访问
2. **ctx 已取消**：在后台 goroutine 中使用了已取消的 ctx
3. **类型不匹配**：用 `SetString` 写入的不能用 `Get`（JSON）读取，反之亦然
4. **key 拼写错误**：存储 key 是区分大小写的

### 插件 panic 了

框架会对每个插件调用进行 panic 恢复，不会导致整个程序崩溃。但你需要：
1. 查看控制台日志中的 panic 堆栈信息
2. 检查 `OnPanic` 方法是否有通知逻辑
3. 常见 panic 原因：数组越界（`cmd.Args[0]` 未检查长度）、空指针（未初始化的字段）

### 后台 goroutine 不执行

**排查步骤**：
1. 确认在 `Awake` 中启动了 goroutine，而不是在 `Start` 中（`Start` 时 Bot 还没完全就绪）
2. 使用 `b.Go()` 而不是 `go func()`，后者不会被框架管理和恢复
3. 检查是否有死锁（channel 满了但没人消费）

### 合并转发消息发送失败

**可能原因**：
1. 群聊用 `GroupForward` + `SendGroupForwardMsg`，私聊用 `FriendForward` + `SendFriendForwardMsg`
2. `Message()` 的参数顺序是 `(userId, nickname, content)`
3. napcat 对合并转发消息有长度限制

---

## 性能与运维

### 内存占用持续增长

**可能原因**：
1. **会话未清理**：使用 `map` 存储的会话状态没有定期清理超时条目
2. **消息缓存无限增长**：List 操作没有用 `LTrim` 限制长度
3. **goroutine 泄漏**：后台 goroutine 没有正确退出

**解决方案**：
- 用 `LTrim` 保持列表固定长度
- 定期清理过期的 `map` 条目
- 使用 `context.WithCancel` 控制后台 goroutine 的生命周期

### 消息处理延迟高

**可能原因**：
1. **插件 Order 设置不当**：耗时操作的插件 Order 太小，阻塞了后续插件
2. **同步 HTTP 调用**：在 `OnGroupMsg` 中直接做 HTTP 请求，应该改为 Channel 任务队列模式
3. **插件太多**：每个插件都会被调用，即使不处理也会有开销

**解决方案**：
- 耗时操作用 Channel 任务队列异步处理
- 合理设置 Order，让快速响应的插件先执行
- 不需要的插件不要注册

### Redis 缓存内存占用高

**可能原因**：
1. **没有设置 TTL**：缓存数据永不过期
2. **List 无限增长**：没有用 `LTrim` 限制长度

**解决方案**：
- 为缓存数据设置合理的 TTL
- 用 `RPush` + `LTrim` 保持列表固定长度
- 定期 `ScanKeys` 清理不需要的数据
- 或改用 `bot.store.cache.driver: memory` 改用进程内内存缓存

---

## 部署

### Linux 交叉编译

```bash
# Windows 上编译 Linux 版本
cd cmd && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../build/AniaBot

# 或使用脚本
scripts/build_linux.bat
```

### 后台运行

```bash
# 使用 nohup
nohup ./AniaBot > output.log 2>&1 &

# 使用 screen
screen -S aniabot
./AniaBot
# Ctrl+A, D 分离

# 使用 systemd（推荐）
# 创建 /etc/systemd/system/aniabot.service
```

### 日志管理

- `pluginlog` 插件会将所有消息打印到标准输出
- 生产环境建议将标准输出重定向到日志文件
- 可以用 `logrotate` 管理日志文件大小

---

## 还有问题？

- 查看 [GitHub Issues](https://github.com/jeanhua/AniaBot/issues) 是否有人遇到过类似问题
- 提交新 Issue 时，请附上：错误日志、配置文件（去掉敏感信息）、复现步骤
