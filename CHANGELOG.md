# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

发布流程：打 `v*.*.*` 标签后，GitHub Release 的正文与 Docker 镜像描述
都会自动从本文件中对应版本段落提取，因此发版前请先更新对应版本的内容。

## [Unreleased]

### 新增

- 定时任务支持委派子代理：任务 AI 可通过 `subagent_run` 把复杂子任务交给一次性子代理在后台并行执行（`subagent_list` / `subagent_cancel` 可查看与取消）；由于子代理是异步的，任务收尾时会自动等待全部子代理返回，把结果回喂给任务 AI 合成最终回复——只有这最后一轮输出才推送给目标，子代理返回前的中间回复不推送；子代理超时按任务剩余预算自动压缩，随 `plugin.ai_chat_bot.subagent.enable` 门控

### 变更

- 面板登录会话改为滑动过期：剩余有效期不足 12 小时时，任意一次请求自动顺延至 24 小时并刷新 Cookie，活跃用户不再被固定 24 小时到期强制下线；闲置满 24 小时仍会过期，改密/重置密码吊销全部会话的行为不变

## [v3.3.0] - 2026-07-27

### 新增

- 面板配置管理新增配置预设功能：可将当前全部配置（含密钥、MCP / Prompt 覆盖）保存为命名快照，支持一键应用切换、同名覆盖更新与删除；应用预设仅覆盖快照中包含的配置键，不影响其他键，重启后生效
- 定时任务执行日志记录完整执行过程：任务内容、LLM 轮数、工具调用明细（名称/参数/结果/耗时）与最终回复，面板「定时任务」页参考 Query 日志展示
- 面板新增「Token 统计」独立页面：按来源（对话/定时任务）、会话类型（群聊/私聊）、执行状态、消耗目标排行（Top 10）、24 小时分布与最近 30 天分来源每日序列等多维度统计 token 用量
- 主对话最大工具调用轮数支持面板配置：新增配置项 `plugin.ai_chat_bot.max_iterations`（默认 20），主对话与定时任务统一生效
- 每日新闻插件新增启用/禁用开关：新增配置项 `plugin.dailynews.enable`（默认 true），关闭后停止定时播报并忽略 `/news` 命令
- 面板概览页新增 AI API 余额卡片：启用配置项 `bot.balance.enable` 后，按声明式配置请求余额接口——`bot.balance.url` / `bot.balance.headers` / `bot.balance.body`（支持 `${base_url}` `${api_key}` `${model}` 占位符，取自 AI 对话插件配置）、`bot.balance.method`（GET/POST），显示模板 `bot.balance.format` 中的 `{gjson 路径}` 会被替换为响应 JSON 中对应字段的值（默认适配 DeepSeek 风格 `/user/balance` 接口）；结果按 `bot.balance.cache_sec`（默认 300 秒）缓存，支持手动强制刷新，配置改动即时生效无需重启

### 变更

- 面板概览页的定时任务管理（新建/编辑/删除/启停）合并到「任务日志」页，侧边栏该项更名为「定时任务」
- 移除 `MAX_ITERATIONS` 环境变量，工具调用轮数统一由 `plugin.ai_chat_bot.max_iterations` 配置项控制
- 面板 NapCat 适配器 HTTP 配置项标注更明确：「HTTP 监听端口」更名「本地监听端口」、「HTTP 目标地址」更名「NapCat HTTP 地址」，帮助文本注明与 NapCat 侧「HTTP 客户端」（上报）和「HTTP 服务器」（调用）配置的对应关系

### 修复

- HTTP 适配器接收 NapCat 事件上报时校验 token：配置 `bot.adapter.token` 后，未携带正确 `Authorization: Bearer <token>` 头（或 `access_token` 查询参数）的上报请求将被拒绝（401），防止伪造事件注入
- HTTP 适配器本地服务器启动失败（如端口被占用）时明确输出错误日志，不再静默失效
- HTTP 适配器上报接口仅接受 POST 请求，其他方法返回 405

## [v3.2.0] - 2026-07-27

### 新增

- Token 消耗监控功能：支持总量、今日及最近 14 天的每日用量统计
- 定时任务执行日志查询功能：支持条件筛选与展示

### 变更

- 移除不必要的 `EstimateTokens` 和 `countRunes` 函数，优化代码结构

## [v3.1.1] - 2026-07-26

### 新增

- 异步子代理功能：支持复杂任务的委派与管理
- 跳过无效用户的子代理消息，优化消息处理逻辑
- 面板首页与插件卡片新增功能展示与图标

### 文档

- 添加容器部署指南并链接至快速开始文档
- 更新 docker.md 中关于源码目录挂载的说明

## [v3.1.0] - 2026-07-26

### 新增

- AI 子代理（subagent）功能：主 AI 可将复杂/耗时任务委派给临时子代理执行

### 文档

- 移除 .gitignore 中对 custom/plugins 的忽略规则

## [v3.0.0] - 2026-07-26

### 新增

- 消息日志改用缓存存储，Redis 驱动下重启后保留
- 请求拦截插件配置更新，添加群号和 QQ 号名单说明
- Web 面板配置说明添加环境变量覆盖提示

### 修复

- OnPanic 用互斥锁保护 lastPanicTime，消除并发 panic 上报时的数据竞争
- HasMention 对 at 段 qq 字段改用 comma-ok 断言，避免异常数据触发 panic
- 更新检查改用完整 commit 哈希比较，避免短哈希长度不一致误报新版本
- get_msg_history 参数改用 desc 标签并将 count 设为可选
- bash 工具自定义环境变量改为追加到继承环境，不再整体替换
- 工具重复注册改为跳过并记录日志，不再 panic
- convertMessage 非图片用户消息拼接全部文本片段，不再只取第一段
- AI 会话状态统一按 g:/f: 前缀键索引，修复同号群聊与私聊互串
- 命令行重置面板密码时同步吊销所有已签发会话
- 内存缓存 matchPattern 支持中间通配段，与 Redis 后端匹配语义对齐
- meme 工具请求传入 ctx 并设置 30s 超时，避免接口挂起时永久阻塞
- bash 工具非零退出码时不再丢弃 stdout/stderr
- 上下文压缩判断改用最后一次调用的 prompt token，避免累加值虚高触发过早压缩
- HTTP 适配器消息过滤规则与 WS 适配器对齐，行为不再随传输方式变化
- HTTP 适配器补齐 API 层状态检查，失败响应不再返回 success=true
- WS 请求 echo 追加原子序号，避免同纳秒并发请求的 echo 碰撞串音
- MCPToolManager.toolCache 加互斥锁，修复并发 AI 会话下的致命 map 并发写
- 内存缓存 ScanKeys 改持写锁，修复 RLock 下删除过期键导致的致命并发冲突
- WS 适配器 ACK 帧改为 readLoop 直接投递，避免 worker 池自饿死
- 字体改为本地打包，移除 Google Fonts CDN 依赖

### 文档

- 修正内置插件文档、架构图、插件开发文档与配置文档中的多处过时内容
- 拦截插件文档补充 whitelist 群聊的 AND 语义说明
- cron.md 示例改用 message.FromUint64 构造 QID
- 修正 README 快速开始中的构建命令

[Unreleased]: https://github.com/jeanhua/AniaBot/compare/v3.1.1...HEAD
[v3.1.1]: https://github.com/jeanhua/AniaBot/compare/v3.1.0...v3.1.1
[v3.1.0]: https://github.com/jeanhua/AniaBot/compare/v3.0.0...v3.1.0
[v3.0.0]: https://github.com/jeanhua/AniaBot/compare/v2.2.2...v3.0.0
