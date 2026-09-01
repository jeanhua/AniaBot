<div align="center">
  <img src="./README/logo.png" width="200" alt="AniaBot Logo"/>
  <h1>AniaBot</h1>
  <p>一个插件驱动型多平台机器人框架</p>
  <a href="https://jeanhua.github.io/AniaBot/">📖 文档</a> |
  <a href="https://github.com/jeanhua/AniaBot">GitHub</a>
</div>

## 项目介绍

**AniaBot** 是一个基于 Go 语言开发的高性能、插件驱动型**多平台**机器人框架，采用模块化设计，让开发者能够快速构建功能强大的机器人应用。

- **高性能**：基于 Go 语言，充分利用并发特性
- **插件驱动**：功能模块化，易于扩展和维护
- **多平台**：QQ（NapCat WebSocket/HTTP，OneBot v11）+ 飞书（官方 SDK 长连接/Webhook）+ Telegram（Bot API 长轮询，无需公网地址）+ Discord（discordgo，Gateway WebSocket，无需公网地址）等；新增平台只需实现一个适配器并注册，框架核心零改动
- **开箱即用**：内置 AI 对话（含 MCP/Skills/Tool Use）、防撤回、复读机等插件
- **插件市场**：独立插件仓库（AniaBot-Plugins）支持 PR 提交插件，面板「插件市场」可浏览插件介绍、在线安装/升级/卸载并自动编译重启（容器内开箱即用）

![framework](./README/framework.png)

![pannel](./README/pannel0801.png)

## 快速开始

```bash
git clone https://github.com/jeanhua/AniaBot.git
cd AniaBot
go mod tidy
cd web && npm ci && npm run build
```

直接运行，首次启动会自动写入默认配置，并在控制台打印 Web 控制面板的随机初始密码：

```bash
go run cmd/main.go
```

交叉编译

```bash
make linux
make windows
```

登录 `http://127.0.0.1:7700`，按设置向导填写平台连接（NapCat 地址 / 飞书 App ID、Secret / Telegram Bot Token / Discord Bot Token）、管理员 ID 与 AI 模型配置即可。默认启用 QQ 平台；在「配置 → 平台适配器」中勾选要启用的平台并填写对应连接信息后重启，即可多平台同时在线（如 QQ + 飞书 + Telegram + Discord）。

二进制部署时，可在面板「自动更新」页一键从 git 拉取最新代码、重新编译并自动重启（需配置源码目录，详见[文档](https://jeanhua.github.io/AniaBot/guide/web-panel#自动更新)）。

## Docker 部署

镜像内置 Go / Node.js / git 工具链，面板的「自动更新」在容器内可直接使用。

使用 Docker Compose：

```bash
git clone https://github.com/jeanhua/AniaBot.git && cd ./AniaBot
docker compose up -d
```

或直接使用镜像：

```bash
cd ~/Bot/
docker run -d --name aniabot \
  --net=host \
  -v ./data:/app/aniabot/data \
  -v ./skills:/app/aniabot/skills \
  jeanhua/aniabot:latest
```

详细配置和插件开发教程请查阅 **[文档站点](https://jeanhua.github.io/AniaBot/)**。

## 许可证

本项目采用 MIT 许可证，详见 [LICENSE](./LICENSE) 文件。
