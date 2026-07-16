# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AniaBot is a plugin-driven QQ bot framework built with Go. It connects to QQ via NapCat (WebSocket or HTTP adapter using the OneBot v11 protocol) and features an AI chat engine powered by OpenAI-compatible LLM APIs with tool calling, MCP (Model Context Protocol) integration, and a skill system.

## Commands

### Run
```bash
go run cmd/main.go
```

### Build (cross-compile)
Requires Go 1.26+ (see `go.mod`).

```bash
make linux     # → build/AniaBot     (GOOS=linux GOARCH=amd64)
make windows   # → build/AniaBot.exe
make clean     # remove build/

# Manual
cd cmd && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../build/AniaBot
```

### Test
```bash
go test ./...                           # all tests
go test -v -race ./...                  # verbose with race detector
go test ./bot/utils/...                 # specific package
go test -run TestParseCommand ./...     # single test by name
```

### Vet & Verify
```bash
go vet ./...
go mod verify
```

## Configuration

- `config.yaml` — production config (loaded via Viper)
- `config.dev.yaml` — development config (tried first, falls back to `config.yaml`)
- `aniabot.mcp.json` — MCP server definitions (stdio / Streamable HTTP / SSE transports)

Config is loaded in `bot/core/core.go`. Plugins receive their config section via `SetConfig(*viper.Viper)` during DI injection — each plugin gets the `plugin.<key>` sub-tree matching its `Meta.SystemConfigKey`.

## Architecture

### Layered Structure

```
cmd/main.go              Entry point: creates adapter, registers plugins, runs bot
common/                  Shared interfaces and models (adapter, plugin, storage, bot, msgchain)
bot/core/                AniaBot orchestrator: plugin lifecycle, event dispatch, DI, storage impls
bot/adapter/napcat/      NapCat protocol adapters (WebSocket and HTTP)
bot/component/           AI chat engine
  aichat/                  ChatBot, LLMClient, MessageBuilder, ToolOrchestrator, messageWindow
  llmtool/                 Tool interface, ToolExecuter, MCP client, SkillManager, schema parser
  functool/                Built-in tools (time, web search, meme, file, msg history, image loading)
bot/plugins/             Six built-in plugins (sys, log, repeat, antiwithdrawal, aichat, news)
bot/utils/               Command parsing, message extraction, URL helpers, time formatting
custom/                  User-created plugin examples and templates
docs/                    VitePress documentation site
```

### Dependency Flow (strictly top-down)

```
cmd/main.go → bot/core, bot/adapter/napcat, bot/plugins/*
bot/core → common/*, bot/utils
bot/adapter/napcat → common/adapter, common/model/message, common/msgchain
bot/plugins/* → common/plugin, common/bot, common/storage, bot/component/*
bot/component/aichat → bot/component/llmtool
bot/component/functool → bot/component/llmtool, bot/utils
bot/component/llmtool → external (openai-go, MCP SDK)
common/* → leaf packages (no upward dependencies)
```

### Plugin System

Plugins implement `common/plugin.Plugin` by embedding `plugin.Meta` and overriding only needed methods. Key mechanics:

- **Ordered execution**: `LevelLog = -1000`, `LevelNormal = 0`, `LevelPostHandle = 1000`. Plugins sorted by `Order` at startup.
- **Middleware chain**: `OnGroupMsg`/`OnFriendMsg` return `(bool, error)`. Return `false` to stop propagation to subsequent plugins.
- **Broadcast notices**: All 14 notice event types (recall, poke, ban, etc.) are delivered to every plugin — no short-circuit.
- **DI injection**: Core injects `Storage` (cache) and `PersistentStorage` alongside `RestyClient`, `Logger`, and `SystemConfig` before calling `Start()`.
- **Lifecycle**: `Start()` → `StartCron()` → `Awake()` → message/notice events → `OnPanic()`
- **Panic recovery**: Every plugin call is wrapped in `safeExecute`; goroutines spawned via `bot.Go()` have crash recovery that notifies all plugins.

### LLM Tool System

Tools are defined as structs embedding `llmtool.BaseTool[ParamsType]`. Parameter structs use `json` tags for names and `desc` tags for descriptions. The `parser.go` reflection engine auto-generates OpenAI-compatible function schemas from these structs — no manual JSON schema needed.

Registration hierarchy:
1. `functool.CreateDefaultTools()` — registers built-in tools. Always on: `time`, `web_search`, `web_explore` (both via Jina), `meme`, `msg_history`, `private_file`, `load_images` (LLM-invoked, on-demand loading of images in the user's current/quoted message; recognition via the multimodal model or OCR fallback in the callback). Opt-in (gated behind config flags for safety): `bash` (executes on the host with whitelist/blacklist regex), `file`/`send_file`, and `local_image` (reads host-local image files for the LLM to view; served as a data URI to the multimodal model or OCR fallback in the callback).
2. `functool.CreateToolsWithMCP()` — adds MCP discovery tools
3. `functool.CreateToolsWithSkill()` — adds `skill_read` tool

Each user session gets a `SessionToolExecutor` with isolated dynamic tools. MCP tools use a two-phase lazy loading pattern (discover → load per session) to avoid context window explosion.

### AI Chat Component

`ChatBot` orchestrates per-session conversations:
- `LLMClient` wraps the OpenAI Go SDK (`openai-go/v3`), supporting reasoning content (DeepSeek-style `reasoning_content`)
- `MessageBuilder` constructs message arrays with system prompt, skill registry, chat history, tool results
- `messageWindow` (in `memorywindow.go`) is a token-budget context window, not a fixed-turn slider: it records prompt-token usage and, once that exceeds 80% of `max_context_tokens`, compresses prior history via an LLM summarizer (`MaybeCompress` / `NewContextCompressor`). History is persisted across restarts via an injected `HistoryStore` (backed by `PersistentStorage`, namespaced per group/friend): `append`/`MaybeCompress`/`clear` all sync to disk with a background context; `ChatBot.LoadHistory` replays on session creation. On replay, remote http(s) image URLs (QQ temp links that expire) are degraded to a text marker, while `data:` URIs (local images) are preserved.
- `ToolOrchestrator` runs the multi-turn agent loop: LLM → tool call → result → LLM (up to `MAX_ITERATIONS` iterations, default 20, overridable via the `MAX_ITERATIONS` env var)
- `CallBackFuncs` bridges tool execution back to QQ messaging (send text, image, file)
- **AI scheduled tasks (clock)** — `pluginaichat/clock.go`'s `clockManager` owns its **own `*cron.Cron`** (separate from the framework's `StartCron` cron) and persists dynamic, AI/user-managed tasks to `PersistentStorage` (`clock:` namespace). On trigger it runs a **fresh ephemeral ChatBot** (nil history store → discarded after) with full tools, under a per-task timeout, logged via `bot/component/tasklog`. Trigger prompt = `【定时任务】<title>\n<content>`. Users manage via `/clock`; AI manages via `clock_create/list/update/delete/log` tools (registered per session in `getChat`); triggered tasks additionally get a `send_message` tool to message any group/friend. Gated by `plugin.ai_chat_bot.clock.enable`.

### Message Chain Builder

Fluent builder pattern for constructing bot responses:
```go
msgchain.Builder().Group(target).Text("hello").Mention(qid).Build()
msgchain.Builder().GroupForward(target).Node(sender, content).Build()
```

### Storage

AniaBot exposes two interface-adapted storage layers, both with Clone-based `base64(pluginName)` namespacing injected per plugin:

- **CACHE** (`common/storage.Storage`, injected as `p.Storage`): volatile; supports TTL + Redis-list semantics. Backends: `redis` (default) | `memory` (opt-in, process-local).
- **PERSISTENT** (`common/storage.PersistentStorage`, injected as `p.PersistentStorage`): durable KV/document store, survives restart, no TTL/list semantics. Backends: `sqlite` (default) | `mysql` (opt-in).

All SQL backends use pure-Go drivers (`modernc.org/sqlite`, `github.com/go-sql-driver/mysql`) — no CGO, cross-compile friendly. Config lives under `bot.store.cache` / `bot.store.persistent`; factories in `bot/core/storage_factory.go`.

## Key Conventions

- **Language**: Code comments and user-facing strings are in Chinese
- **Logging**: Mixed `log/slog` (core/plugins) and `log.Printf` (adapter/tools). Use `slog` for new code.
- **Error handling**: Adapter/storage methods return `(value, bool)` not `(value, error)`. Use `fmt.Errorf` with `%w` wrapping in component-layer code.
- **Package naming**: Lowercase, concatenated (e.g., `pluginaichat`, `llmtool`, `functool`, `aniaerror`)
- **No linting config**: CI runs `go vet ./...` only. No golangci-lint.
- **Generics**: Used for `BaseTool[T]`, `MessageQueue[T]`, `safeExecuteWithReturn[T]`
- **Functional options**: `Option func(*AniaBot)` pattern for bot configuration
- **OneBot v11**: All QQ message types use the OneBot v11 segment format (`OB11Segment`)

## CI/CD

Three GitHub Actions workflows in `.github/workflows/`:
- **test.yaml** — runs on push/PR to `main` and `dev/deploy`: `go vet`, `go test -v -race -coverprofile=coverage.out ./...`
- **release.yaml** — runs on version tags (`v*.*.*`): tests then auto-generates changelog from conventional commits
- **docs.yaml** — builds VitePress docs and deploys to GitHub Pages on `main` push

## External Dependencies

| Dependency | Purpose |
|---|---|
| `openai-go/v3` | OpenAI-compatible LLM API client |
| `modelcontextprotocol/go-sdk` | MCP protocol client |
| `gorilla/websocket` | WebSocket for NapCat adapter |
| `go-resty/resty/v2` | HTTP client |
| `redis/go-redis/v9` | Redis cache storage backend |
| `modernc.org/sqlite` | Pure-Go SQLite, persistent storage default |
| `github.com/go-sql-driver/mysql` | Pure-Go MySQL, persistent storage opt-in |
| `robfig/cron/v3` | Cron scheduling for timed plugins |
| `spf13/viper` | YAML configuration loading |
| `lmittmann/tint` | Colored slog handler |
