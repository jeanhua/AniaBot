# 快速开始

## 环境要求

| 依赖 | 版本要求 |
|------|---------|
| Go | 1.21.0 或更高 |
| Redis | 任意稳定版本（可选，用于持久化存储） |
| napcat | 最新版（QQ 协议适配器） |

::: tip 操作系统
Windows、Linux、macOS 均可运行。
:::

## 安装方式

### 方式一：源码安装（推荐）

适合需要自定义插件的场景。

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
# bot 配置
bot:
  admin_id: 123456789   # 管理员 QQ 号
  adapter:
    ws:
      address: ws://localhost:4455  # napcat WebSocket 地址
      max_retries: 5                # 连接失败最大重连次数
```

**4. 启动**

```bash
go run cmd/main.go
```

---

### 方式二：作为依赖导入

适合在已有项目中集成 AniaBot。

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
    // 创建协议适配器
    adapter := napcat.NewNapcatWebSocketAdapter()

    // 创建机器人实例
    bot := aniabot.NewAniaBot(adapter)

    // 注册插件（Order 越小越先执行）
    bot.AddPlugin(pluginlog.NewPlugin())           // 日志插件
    bot.AddPlugin(pluginaichat.NewAIChatPlugin())  // AI 对话插件

    // 启动
    bot.Run()
}
```

## 注册所有内置插件

```go
import (
    "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginnews"
    "github.com/jeanhua/AniaBot/bot/plugins/pluginsys"
)

bot.AddPlugin(pluginlog.NewPlugin())
bot.AddPlugin(pluginsys.NewPlugin())
bot.AddPlugin(pluginrepeat.NewPlugin())
bot.AddPlugin(pluginaichat.NewAIChatPlugin())
bot.AddPlugin(pluginantiwithdrawal.NewPlugin())
bot.AddPlugin(pluginnews.NewPlugin())
```

## 调试技巧

1. **启用日志插件** — 注册 `pluginlog` 可以在控制台看到所有收到的消息
2. **检查 napcat 连接** — 确认 napcat 的 WebSocket Server 已启动且地址配置正确
3. **查看控制台输出** — AniaBot 会将错误信息打印到标准输出

## 下一步

- [配置说明](./configuration) — 了解完整的配置项
- [内置插件](./builtin-plugins) — 查看所有内置插件的用法
- [开发第一个插件](../plugin/first-plugin) — 开始自定义插件开发
