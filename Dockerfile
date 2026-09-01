# syntax=docker/dockerfile:1

# AniaBot 一体化镜像：编译产物 + 自动更新所需的工具链（git / go / node / npm）。
# 最终镜像基于 golang alpine，保证面板「自动更新」流水线（拉代码 → go mod →
# npm 构建前端 → go build → 替换二进制重启）在容器内可直接执行。

# ---------- 阶段 1：构建前端（产物嵌入 Go 二进制） ----------
FROM node:22-alpine AS web-builder
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# vite.config 的 outDir 指向 ../bot/adminpanel/dist（go:embed 目录）
RUN npm run build

# ---------- 阶段 2：编译 Go ----------
FROM golang:1.25-alpine AS go-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY custom/ ./custom/
COPY bot/ ./bot/
COPY common/ ./common/
# 前端产物（.dockerignore 已排除仓库内 dist，此处用阶段 1 的新鲜产物）
COPY --from=web-builder /build/bot/adminpanel/dist ./bot/adminpanel/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/AniaBot ./cmd/

# ---------- 阶段 3：运行时 ----------
# 保留 golang 基础镜像：自动更新需要在容器内执行 go build
FROM golang:1.25-alpine
RUN apk add --no-cache ca-certificates tzdata git nodejs npm bash

WORKDIR /app/aniabot
COPY --from=go-builder /out/AniaBot ./AniaBot

ENV ANIA_BOT_ADMIN_PANEL_LISTEN=0.0.0.0:7700 \
    TZ=Asia/Shanghai

# 只需持久化两类数据，其余（含自动更新的源码目录，放容器内任意路径即可，
# 重建后重新克隆）都是一次性的：
#   ./data    —— SQLite 数据库等持久化数据（ANIABOT_SQLITE_PATH 默认 ./data/aniabot.db）
#   ./skills  —— AI skills（plugin.ai_chat_bot.skills_dir 默认 ./skills）
VOLUME ["/app/aniabot/data", "/app/aniabot/skills"]

EXPOSE 7700
CMD ["./AniaBot"]
