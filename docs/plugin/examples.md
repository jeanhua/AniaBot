# 完整示例

## 天气查询插件

一个完整的天气查询插件，展示命令解析、配置读取、消息发送的综合用法。

```go
package pluginweather

import (
    "context"
    "fmt"

    "github.com/jeanhua/AniaBot/common/bot"
    "github.com/jeanhua/AniaBot/common/model/command"
    "github.com/jeanhua/AniaBot/common/model/message"
    "github.com/jeanhua/AniaBot/common/msgchain"
    "github.com/jeanhua/AniaBot/common/plugin"
    "github.com/jeanhua/AniaBot/common/plugininfo"
    "github.com/spf13/viper"
)

type WeatherPlugin struct {
    plugin.Meta
    apiKey string
}

func NewPlugin() *WeatherPlugin {
    return &WeatherPlugin{
        Meta: plugin.Meta{
            Name:      "天气查询",
            HelpWords: "查询天气，用法：/weather <城市>",
            AdminOnly: false,
            ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
            Author:    "jeanhua",
            Version:   "1.0.0",
            Order:     20,
        },
    }
}

func (p *WeatherPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    p.apiKey = cfg.GetString("plugin.weather.api_key")
    return nil
}

func (p *WeatherPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "weather" {
        return true, nil
    }

    if len(cmd.Args) == 0 {
        builder := msgchain.Builder().Group()
        builder.Reply(msg.MessageId)
        builder.Text("请指定城市，例如：/weather 北京")
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil
    }

    city := cmd.Args[0]
    info, err := p.queryWeather(city)
    if err != nil {
        builder := msgchain.Builder().Group()
        builder.Text(fmt.Sprintf("查询失败：%v", err))
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil
    }

    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)
    builder.Text(fmt.Sprintf("📍 %s 天气\n%s", city, info))
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil
}

func (p *WeatherPlugin) queryWeather(city string) (string, error) {
    // 调用天气 API...
    return "晴，25°C，东南风 3 级", nil
}
```

**config.yaml**：

```yaml
plugin:
  weather:
    api_key: "your-weather-api-key"
```

---

## 积分系统插件

展示存储接口的综合用法：读写用户积分、排行榜查询。

```go
package pluginscore

import (
    "context"
    "fmt"
    "sort"

    "github.com/jeanhua/AniaBot/common/bot"
    "github.com/jeanhua/AniaBot/common/model/command"
    "github.com/jeanhua/AniaBot/common/model/message"
    "github.com/jeanhua/AniaBot/common/msgchain"
    "github.com/jeanhua/AniaBot/common/plugin"
    "github.com/jeanhua/AniaBot/common/plugininfo"
)

type ScorePlugin struct {
    plugin.Meta
}

func NewPlugin() *ScorePlugin {
    return &ScorePlugin{
        Meta: plugin.Meta{
            Name:    "积分系统",
            HelpWords: "/score 查看积分 | /rank 查看排行",
            ShowFor: plugininfo.ShowForGroup,
            Order:   30,
        },
    }
}

func (p *ScorePlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    key := fmt.Sprintf("score:%d:%d", msg.GroupId, msg.Sender.UserId)

    switch cmd.Name {
    case "score":
        val, _ := p.Storage.GetString(ctx, key)
        if val == "" {
            val = "0"
        }
        builder := msgchain.Builder().Group()
        builder.Mention(msg.Sender.UserId)
        builder.Text(fmt.Sprintf(" 你的积分：%s", val))
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil

    case "checkin":
        // 每日签到加积分（WithCheckExist 防止重复签到）
        dailyKey := fmt.Sprintf("checkin:%d:%d", msg.GroupId, msg.Sender.UserId)
        if !p.Storage.SetString(ctx, dailyKey, "1",
            storage.WithTTL(24*time.Hour),
            storage.WithCheckExist(),
        ) {
            builder := msgchain.Builder().Group()
            builder.Text("今天已经签到过了！")
            b.SendGroupMsg(msg.GroupId, builder.Build())
            return false, nil
        }
        // 加 10 分逻辑...
        return false, nil
    }

    return true, nil
}
```

---

## 更多示例

查看 [dev/deploy 分支](https://github.com/jeanhua/AniaBot/tree/dev/deploy) 获取更多由社区开发的插件示例，包括：

- 音乐点歌
- 图片生成
- 群管理工具
- 游戏互动
- 以及更多...
