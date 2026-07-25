# 第一个插件

本教程带你写一个完整的「掷骰子」插件：群里 @机器人 发送 `/dice` 随机返回 1-6 的点数。几十行代码，覆盖插件开发的核心流程。

## 最终效果

```
你: @AniaBot /dice
AniaBot: 🎲 你掷出了 4 点！
```

## 第一步：创建插件文件

在 `custom/plugins/` 下新建目录（推荐位置，与示例插件同级）：

```
custom/plugins/plugindice/plugin.go
```

## 第二步：定义插件

```go
package plugindice

import (
	"context"
	"math/rand/v2"
	"strconv"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
)

type DicePlugin struct {
	plugin.Meta
}

func NewPlugin() *DicePlugin {
	return &DicePlugin{
		Meta: plugin.Meta{
			Name:      "掷骰子插件",
			HelpWords: "@我发送 /dice 掷一个骰子",
			Order:     plugin.LevelNormal,
			ShowFor:   plugininfo.ShowForGroup,
			Author:    "you",
			Version:   "1.0.0",
		},
	}
}
```

嵌入 `plugin.Meta` 后，插件接口的所有方法都有了默认实现，我们只需重写关心的部分。

## 第三步：响应群消息

```go
func (p *DicePlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	// cmd.Mention: 消息中是否 @ 了机器人
	// cmd.Name:    / 后的命令名
	if !cmd.Mention || cmd.Name != "dice" {
		return true, nil // 不是我们的命令，放行给后续插件
	}

	point := rand.IntN(6) + 1

	chain := msgchain.Builder().Group()
	chain.Mention(msg.Sender.UserId)
	chain.Text(" 🎲 你掷出了 " + strconv.Itoa(point) + " 点！")
	b.SendGroupMsg(msg.GroupId, chain.Build())

	return false, nil // 已处理，阻断传播
}
```

几个要点：

- **`cmd`** 由框架预解析：`/dice 20` → `cmd.Name = "dice"`, `cmd.Args = ["20"]`
- **`msgchain.Builder()`** 是链式消息构造器，`Group()` 构造群消息，`Friend()` 构造私聊消息
- **返回值** `false` 表示这条消息到此为止，不再传给后面的插件（比如 AI 对话）

## 第四步：注册插件

编辑 `cmd/main.go`：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/plugindice"

func main() {
	adapter := napcat.NewNapcatWebSocketAdapter()
	bot := core.NewAniaBot(adapter)

	bot.AddPlugin(pluginsys.NewPluginSys())
	// ... 其他插件
	bot.AddPlugin(plugindice.NewPlugin()) // ← 注册

	bot.Run()
}
```

## 第五步：运行测试

```bash
go run cmd/main.go
```

群里 @机器人 发送 `/dice`，同时私聊发送 `/help` 可以看到你的插件已经出现在列表中。

## 进阶：读取自己的配置

如果插件需要配置项，在 Web 控制面板的「配置管理 → 高级模式 (JSON)」中添加 `plugin.<插件名>.*` 键（配置存于数据库），然后在 `Start` 中读取：

```yaml
plugin:
  dice:
    max_point: 20   # 掷一个 20 面骰
```

```go
type DicePlugin struct {
	plugin.Meta
	maxPoint int
}

func (p *DicePlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.maxPoint = cfg.GetInt("plugin.dice.max_point")
	if p.maxPoint <= 0 {
		p.maxPoint = 6 // 默认值
	}
	p.Logger.Info("骰子插件初始化", "maxPoint", p.maxPoint)
	return nil
}
```

`Start()` 返回非 nil 错误会导致插件初始化失败并在日志中报错。

## 进阶：使用注入的依赖

框架已注入好常用依赖，直接用：

```go
p.Logger.Info("有人掷骰子", "user", msg.Sender.UserId)   // 结构化日志
p.Storage.SetString(ctx, "last", "4", storage.WithTTL(time.Hour)) // 缓存
resp, _ := p.RestyClient.R().Get("https://api.example.com")        // HTTP
if msg.Sender.UserId == p.SystemConfig.AdminId { /* 管理员专属 */ }
```

## 完整代码

项目内置了一个最小示例可直接参考：[`custom/mvp/example.go`](https://github.com/jeanhua/AniaBot/blob/main/custom/mvp/example.go)，以及模板 [`custom/plugins/pluginexample/plugin.go`](https://github.com/jeanhua/AniaBot/blob/main/custom/plugins/pluginexample/plugin.go)。

## 下一步

- [命令解析](/plugin/commands) —— `cmd` 的完整解析规则
- [消息构造器](/plugin/message-builder) —— 图片、视频、合并转发等全部消息类型
- [数据存储](/plugin/storage) —— 让插件记住东西
