# 快速开始

## 你将学到什么

完成本指南后，你将：
1. 成功运行 AniaBot 并连接到 QQ
2. 了解配置文件的基本结构
3. 知道如何注册和使用内置插件
4. 准备好开发自己的第一个插件

## 环境要求

| 依赖 | 版本要求 | 用途 |
|------|---------|------|
| Go | 1.26.0 或更高 | 编译运行框架 |
| napcat | 最新版 | QQ 协议适配器（必须） |
| Redis | 任意稳定版本 | 缓存存储（可选，不装可用内存存储代替） |

::: tip 操作系统
Windows、Linux、macOS 均可运行。下面的命令以 Linux/macOS 为主，Windows 用户将 `go run` 替换为 `go run` 即可（Go 是跨平台的）。
:::

## 5 分钟快速启动

这是最精简的启动路径，适合想尽快看到效果的开发者。

**第 1 步：确保 napcat 已运行**

napcat 是 AniaBot 与 QQ 之间的桥梁。你需要先安装并启动 napcat，确保它的 WebSocket Server 已开启（默认地址 `ws://localhost:4455`）。

::: warning 关于 napcat
AniaBot 本身不直接连接 QQ，而是通过 napcat（基于 OneBot v11 协议）来收发消息。你需要单独部署 napcat。详见 [napcat 官方文档](https://napneko.github.io/)。
:::

**第 2 步：克隆并配置**

```bash
git clone https://github.com/jeanhua/AniaBot.git
cd AniaBot
go mod tidy
```

编辑 `config.yaml`（如果没有，复制 `config.dev.yaml` 作为起点）：

```yaml
bot:
  admin_id: 你的QQ号        # 管理员 QQ 号，用于权限控制
  adapter:
    ws:
      address: ws://localhost:4455  # napcat WebSocket 地址
      max_retries: 5                # 连接失败重试次数
```

**第 3 步：启动**

```bash
go run cmd/main.go
```

看到类似 `connected to napcat` 的日志就说明连接成功了。在群里发条消息，控制台应该能看到日志输出。

## 安装方式详解

### 方式一：源码安装（推荐）

适合需要自定义插件的场景。你拥有完整的源码控制权，可以自由修改和扩展。

**1. 克隆项目**

```bash
git clone https://github.com/jeanhua/AniaBot.git
cd AniaBot
```

**2. 安装依赖**

```bash
go mod tidy
```

**3. 配置机器人**

编辑 `config.yaml`：

```yaml
# bot 核心配置
bot:
  admin_id: 123456789       # 你的 QQ 号（管理员）
  adapter:
    ws:
      address: ws://localhost:4455  # napcat WebSocket Server 地址
      max_retries: 5                # -1 表示无限重连
```

**4. 启动**

```bash
go run cmd/main.go
```

---

### 方式二：作为依赖导入

适合在已有项目中集成 AniaBot，或者你想从零搭建自己的主程序。

```bash
go get -u github.com/jeanhua/AniaBot
```

创建主程序文件：

```go
package main

import (
    aniabot "github.com/jeanhua/AniaBot/bot/core"
    "github.com/jeanhua/AniaBot/bot/adapter/napcat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
)

func main() {
    // 创建协议适配器（连接 napcat）
    adapter := napcat.NewNapcatWebSocketAdapter()

    // 创建机器人实例
    bot := aniabot.NewAniaBot(adapter)

    // 注册插件（Order 越小越先执行）
    bot.AddPlugin(pluginlog.NewPlugin())           // 日志插件，打印收到的消息
    bot.AddPlugin(pluginaichat.NewAIChatPlugin())  // AI 对话插件

    // 启动（会阻塞，持续运行直到进程退出）
    bot.Run()
}
```

::: tip 两种方式的区别
源码安装方式的 `cmd/main.go` 已经帮你配置好了所有内置插件和默认配置，开箱即用。依赖导入方式则需要你自己编写 `main()` 函数，灵活度更高。
:::

## 注册所有内置插件

如果你想一次性启用所有内置功能：

```go
import (
    "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginnews"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginsys"
)

bot.AddPlugin(pluginlog.NewPlugin())                  // 日志：打印所有消息
bot.AddPlugin(pluginsys.NewPlugin())                  // 系统：/help 命令
bot.AddPlugin(pluginrepeat.NewPlugin())               // 复读机：3 次重复自动复读
bot.AddPlugin(pluginaichat.NewAIChatPlugin())         // AI 对话：@机器人 聊天
bot.AddPlugin(pluginantiwithdrawal.NewPlugin())       // 防撤回：缓存消息
bot.AddPlugin(pluginnews.NewPlugin())                 // 新闻：定时推送
```

插件按 `Order` 从小到大依次执行。内置插件的 Order 值：

| 插件 | Order | 说明 |
|------|-------|------|
| pluginlog | -1000 | 最先执行，记录所有消息 |
| pluginsys | 0 | 正常优先级 |
| pluginrepeat | 0 | 正常优先级 |
| pluginaichat | 0 | 正常优先级 |
| pluginantiwithdrawal | 0 | 正常优先级 |
| pluginnews | 0 | 正常优先级 |

## 常见问题排查

### napcat 连接失败

**症状**：启动后看到 `connection refused` 或反复重连。

**排查步骤**：
1. 确认 napcat 已启动且 WebSocket Server 已开启
2. 检查 `config.yaml` 中的 `address` 是否与 napcat 配置一致
3. 如果 napcat 在另一台机器上，确保防火墙放行了对应端口
4. 尝试用浏览器访问 `ws://localhost:4455` 确认端口可达

### 启动后没有收到消息

**排查步骤**：
1. 注册 `pluginlog` 插件，看控制台是否有消息日志
2. 确认机器人 QQ 号已被拉入目标群聊
3. 检查是否有其他插件返回了 `false` 阻止了消息传递

### Redis 连接失败

**症状**：启动时 panic 提示 Redis 连接失败。

**解决方案**：
- 方案 A：确保 Redis 已启动，`bot.store.cache.redis` 配置正确
- 方案 B：无需 Redis 时，在 `config.yaml` 设置 `bot.store.cache.driver: memory`，改用进程内内存缓存（重启后清空，无需额外服务）

::: tip 持久化存储无需外部服务
框架默认使用 SQLite 作为持久化层（数据文件位于 `./data/aniabot.db`），开箱即用、无需安装任何数据库。需要 MySQL 时可改设 `bot.store.persistent.driver: mysql`。
:::

### 配置文件不生效

AniaBot 的配置加载顺序：优先读取 `config.dev.yaml`，如果不存在则读取 `config.yaml`。确保你编辑的是正确的文件。

## 调试技巧

1. **启用日志插件** — 注册 `pluginlog` 可以在控制台看到所有收到的消息，这是最直接的调试方式
2. **查看控制台输出** — AniaBot 会将错误信息打印到标准输出
3. **检查 napcat 日志** — 有时候问题出在 napcat 侧，同时查看两边的日志更容易定位问题
4. **使用 `config.dev.yaml`** — 开发时使用独立的开发配置，避免影响生产环境

## 下一步

- [配置说明](./configuration) — 了解完整的配置项和环境隔离
- [内置插件](./builtin-plugins) — 查看所有内置插件的详细用法
- [开发第一个插件](../plugin/first-plugin) — 开始自定义插件开发（20 分钟上手）
- [常见问题](./faq) — 遇到问题先看这里
