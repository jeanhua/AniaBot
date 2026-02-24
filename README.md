# 群聊部署分支

> 该分支为群聊部署的分支，特性会频繁变动，插件频繁修改，若想体验更多实时有趣的内容可关注本分支，若自行开发需要请切换main分支

### 新增插件

1. 抖音分享解析插件：分享抖音内容可解析视频直链
2. 二次元壁纸图片：可获取随机二次元壁纸
3. waifu.pics插件：获取分类随机二次元图和表情包
4. GithubRepoer插件：给github项目进行质量分析并给出报告
5. 消息拦截器插件：给群聊和用户配置黑白名单，拦截和放行消息事件

![help](./README/help.png)

![AI1](./README/AI1.png)

![AI2](./README/AI2.png)

![AI3](./README/AI3.png)

![acg](./README/acg.png)

![waifu_help](./README/waifu_help.png)

![waifu](./README/waifu.png)

![douyin](./README/douyin.png)

每日新闻插件

![news](./README/news.png)

![news](./README/gr.png)

<details>
  <summary>AniaBot项目质量报告, 点击展开</summary>

（扶了扶眼镜，镜片反射出代码的寒光）好家伙，让我看看这QQ机器人框架……嚯！这目录结构整得跟军事化管理似的，让我这个“代码侦探”来好好盘一盘这位程序员的杰作！

## 🕵️‍♂️ 项目结构锐评

**先扬：**
“组织结构清晰度我给8分！`bot/`、`common/`、`cmd/`、`custom/` 分层明确，一看就是强迫症患者的作品。特别是把 `adapter`、`component`、`plugins` 分开，这波操作我给满分，比那些把所有代码塞进一个 `main.go` 的勇士强多了！”

**后抑：**
“但是！（拍桌子）`bot/utils` 里混了个 `aitooltransmit.go`，这名字起得跟快递公司似的。还有 `common/aniaerror/errors.go` 里定义的 `Timeout = context.DeadlineExceeded` —— 大哥，你这是给标准库起外号吗？直接 `context.DeadlineExceeded` 不香吗？”

## 🔍 代码质量大赏

### 亮点时刻 ✨

1. **插件系统设计优秀**：`common/plugin/plugin.go` 接口定义清晰，`Meta` 结构体嵌入实现继承，Go语言的优雅体现得淋漓尽致！
   
2. **消息链构造器巧妙**：`common/msgchain/builder.go` 的链式API设计，让我想起了Builder模式教科书案例：
   ```go
   msgchain.Builder().Group().Mention(123).Text("你好").Face(14).Build()
   ```
   这流畅度，丝滑得能滑冰！

3. **并发控制到位**：`pluginaichat/plugin.go` 里的 `tryLock/unLock` 机制，防止AI对话被刷爆，考虑周到！

### 槽点集锦 🤦‍♂️

**第一幕：重复代码的狂欢**
```go
// bot/adapter/napcat/wsadapter.go 第128-130行
if msg.MessageType == "group" && n.trigger.OnGroupMsg != nil {
    if msg.RawMessage != "" {
        n.trigger.OnGroupMsg(msg)
    }
}
// 第132-134行（几乎一样的代码）
else if msg.MessageType == "private" && msg.SubType == "friend" && n.trigger.OnFriendMsg != nil {
    if msg.RawMessage != "" {
        n.trigger.OnFriendMsg(msg)
    }
}
```
“这位壮士，`if msg.RawMessage != ""` 检查写两遍，是怕编译器忘记吗？”

**第二幕：错误处理的艺术**
```go
// bot/adapter/napcat/httpadapter.go 第33行
defer func() {
    if err := r.Body.Close(); err != nil {
        log.Printf("关闭HTTP请求体出错: %v", err.Error())
    }
}()
```
“关闭body出错还要log，这严谨程度让我感动……但等等，`err.Error()` 在 `%v` 里？这是要给错误信息套娃吗？”

**第三幕：魔法数字的派对**
```go
// bot/plugins/pluginantiwithdrawal/plugin.go
const (
    ResourceTimeout = 60 * 3 // 时间戳，3分钟
)
```
“60 * 3 = 180秒 = 3分钟，这数学我给满分！但为啥不直接 `3 * time.Minute`？是怕 `time` 包收费吗？”

**第四幕：最骚的操作**
```go
// bot/core/core.go 第86行
encodeName := base64.StdEncoding.EncodeToString([]byte(p.GetMeta().Name))
p.SetStorage(ania.storage.Clone(encodeName))
```
“用base64编码插件名当存储前缀……（扶额）这脑回路，我愿称之为‘防同事看懂大法’！”

## 📦 技术选型点评

**优点：**
- `langchaingo` + `openai`：AI集成方案选型现代
- `redis/go-redis/v9`：存储选型靠谱
- `cron/v3`：定时任务库专业

**疑问：**
“`build_linux.bat` 这文件名……（揉眼睛）在Windows里编译Linux二进制？这波操作我看不懂但大受震撼！”

## 🎯 终极吐槽与抢救建议

### 抢救建议（严肃版）：

1. **统一错误处理**：把 `log.Println` 满天飞的情况整理下，考虑结构化日志
2. **提取公共方法**：HTTP和WebSocket adapter里大量重复逻辑，该抽象了！
3. **配置验证**：`config.yaml` 读取后缺少必要字段验证，容易运行时爆炸
4. **测试覆盖**：这么大个项目，测试文件呢？`_test.go` 文件一个没见着！

### 幽默总结：

“这代码就像一碗螺蛳粉——闻起来有点怪（那些魔法数字和重复代码），但吃起来真香（架构设计和插件系统）！作者在‘工程规范’和‘快速实现’之间反复横跳，最终呈现出一部‘有瑕疵的佳作’。”

**综合评分：7.5/10**

- 架构设计：★★★★☆
- 代码规范：★★★☆☆  
- 可维护性：★★★★☆
- 测试覆盖：★☆☆☆☆（没看到测试文件，扣大分！）
- 幽默指数：★★★★★（无意中创造了很多笑点）

**最后一句：** “继续优化，这框架有潜力成为QQ机器人界的‘瑞士军刀’。但请先把测试补上，不然就是‘没开刃的瑞士军刀’——好看但不好用！” 🔪

</details>