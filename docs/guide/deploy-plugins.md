# 部署分支插件

主分支只保留框架与六个内置插件。社区的**示例/生产插件**维护在 [`dev/deploy`](https://github.com/jeanhua/AniaBot/tree/dev/deploy) 分支的 `custom/plugins/` 目录下，每个都是一个完整可运行的插件实现 —— 既是功能，也是最好的学习素材。

## 插件一览

<PluginCards :plugins="[
  { icon: '🖼️', name: '二次元壁纸', desc: '获取随机二次元壁纸图片', cmds: ['/acg'] },
  { icon: '🎲', name: '随机活跃', desc: '随机在群里活跃气氛：点赞、戳一戳、群签到', cmds: [] },
  { icon: '✏️', name: '聊天记录伪造', desc: '按对话流程伪造合并转发聊天记录', cmds: ['伪造记录'] },
  { icon: '🎵', name: '抖音解析', desc: '解析抖音分享链接，提取无水印视频', cmds: ['/douyin [分享内容]'] },
  { icon: '🎧', name: 'GD音乐', desc: '搜索音乐、翻页浏览、发送歌曲', cmds: ['/music [关键词]', '/music get [序号]', '/music next|prev'] },
  { icon: '📊', name: 'GithubRepoer', desc: '输入 GitHub 仓库链接，生成项目分析报告', cmds: ['/gr [项目链接]', '/gr help'] },
  { icon: '📰', name: '群刊', desc: '自动收集群消息并生成有趣的群刊', cmds: ['/gn', '/gn gen'] },
  { icon: '🚧', name: '消息拦截器', desc: '按配置放行/拦截指定群聊和用户，减少干扰', cmds: [] },
  { icon: '🔗', name: 'URL解析', desc: '自动解析群聊中的链接并提取相关信息', cmds: [] },
  { icon: '💠', name: 'waifu.pics', desc: 'waifu.pics 图片获取，支持多种类别', cmds: ['/waifu [类别]', '/waifu help'] },
]" />

## 如何使用

```bash
# 拉取部署分支中的插件目录
git fetch origin dev/deploy
git checkout origin/dev/deploy -- custom/plugins/<插件名>
```

然后在 `cmd/main.go` 中注册：

```go
import "github.com/jeanhua/AniaBot/custom/plugins/waifupics"

bot.AddPlugin(waifupics.NewPlugin())
```

部分插件有自己的配置项，请参考各插件源码中的 `Start()` 方法读取的配置键，在 Web 控制面板的「配置管理 → 高级模式 (JSON)」中以 `plugin.<插件名>.*` 键补充对应配置（重启生效）。

## 学习路径建议

这些示例覆盖了插件开发的主要模式，建议按此顺序阅读源码：

| 顺序 | 插件 | 能学到什么 |
| --- | --- | --- |
| 1 | `waifupics` / `acgwallpaper` | 基础命令响应、HTTP 请求、发图片 |
| 2 | `interceptor` | 中间件思想：用低 Order 拦截消息流 |
| 3 | `douyinparser` / `urlparser` | 被动监听消息内容（非命令触发）、正则提取 |
| 4 | `activeman` | `StartCron` 定时任务、随机行为 |
| 5 | `gdmusicplugin` | 多步交互、会话状态管理、存储使用 |
| 6 | `groupnewsletter` | 消息聚合 + AI 生成 + 定时推送的综合运用 |
| 7 | `chatrecordsmaker` | 合并转发构造、多轮对话状态机 |

::: tip 贡献插件
写好你的插件后，欢迎向 `dev/deploy` 分支提交 PR，让更多人用到！
:::
