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

Requires Go 1.25+ (see `go.mod`).

```bash
make linux     # → build/AniaBot     (GOOS=linux GOARCH=amd64)
make windows   # → build/AniaBot.exe
make web       # rebuild admin panel frontend (cd web && npm ci && npm run build → bot/adminpanel/dist)
make clean     # remove build/

# Manual
cd cmd && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../build/AniaBot
```

The panel frontend's built `dist/` is gitignored (build artifact). `bot/adminpanel` embeds it via `go:embed`, so run `make web` (or `cd web && npm run build`) at least once before `go build` on a fresh clone.

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

All configuration lives in the **database** (the persistent storage's `ania_kv` table), managed via the built-in **Web admin panel** (`bot/adminpanel`, Vue 3 + Tailwind frontend in `web/`, embedded via `go:embed`). There is no `config.yaml` anymore.

- `bot/core/configstore/` — config center. Keys are dot-paths identical to the historical viper keys (`plugin.ai_chat_bot.base_url`, ...), values JSON-encoded. `Init()` seeds framework defaults from the embedded `config_tmpl.yaml` on first run and flags the first-run setup wizard. `ToViper()` builds an in-memory viper so plugin `Start(ctx, *viper.Viper)` semantics are unchanged. `EnsureDefaults()` fills registered defaults for missing keys (never overwrites).
- `common/pluginconfig/` — config field registry + struct binding. Plugins implement the optional `plugin.ConfigSchemaProvider` interface (`ConfigSchema() any`) returning a pointer to a config struct with `cfg` tags (key/label/type/group/help/sensitive/default as struct tags; type inferred from the Go field type, slices comma-separated defaults, pointer scalars = optional params that stay nil when unset). Core reflects the struct before `ToViper()` (`RegisterStruct` — registers panel fields and seeds defaults), then fills it before the plugin's `Start` (`Load`) — plugins read typed struct fields instead of hand-writing `cfg.Get*` calls. The lower-level `plugin.ConfigRegistrar` interface (`ConfigFields() []pluginconfig.Field`) is kept for framework fields (`bot/core/configfields.go`) and dynamic declarations; both share one registry, same-key later registration overwrites in place. The admin panel renders its form dynamically from this registry — adding/removing plugins requires no panel code changes. Called before DI, so implementations must not use injected fields and must return the same pointer on every call.
- **Bootstrap env vars** (needed before the DB opens): `ANIABOT_STORE_DRIVER` (sqlite|mysql), `ANIABOT_SQLITE_PATH` (default `./data/aniabot.db`), `ANIABOT_MYSQL_DSN`.
- MCP servers and per-group/friend prompt overrides are config keys `files.mcp_json` / `files.prompt_json` (raw JSON text), edited graphically in the panel's 扩展配置 (Files) page with an optional raw-JSON source mode.
- Panel auth: random initial password printed to console on first run, SHA-256+salt hash in the `__admin` namespace; config changes take effect after restart.

Config is assembled in `bot/core/core.go` `Run()`: persistent storage (env) → configstore init → collect field registrations + `RegisterStruct` plugin schemas → `EnsureDefaults` → `ToViper()` → create adapter via `WithAdapterFactory` (NapCat adapter picked by `bot.adapter.mode`: ws|http) → `Load` each plugin schema struct → cache storage → plugins. Plugins declaring `ConfigSchema()` read typed struct fields in `Start()`; framework-level shared keys (e.g. `files.mcp_json`) are still read from the whole viper passed to `Start()`.

## Architecture

### Layered Structure

```
cmd/main.go              Entry point: registers plugins, runs bot (adapter chosen at runtime via bot.adapter.mode)
common/                  Shared interfaces and models (adapter, plugin, storage, bot, msgchain)
bot/core/                AniaBot orchestrator: plugin lifecycle, event dispatch, DI, storage impls
  configstore/             DB-backed config center (seed/migrate/ToViper)
bot/adminpanel/          Web admin panel: config/status APIs + embedded SPA (dist/)
bot/adapter/napcat/      NapCat protocol adapters (WebSocket and HTTP)
bot/component/           AI chat engine
  aichat/                  ChatBot, LLMClient, MessageBuilder, ToolOrchestrator, messageWindow
  llmtool/                 Tool interface, ToolExecuter, MCP client, SkillManager, schema parser
  functool/                Built-in tools (time, web search, meme, file, msg history, image loading)
bot/plugins/             Seven built-in plugins (sys, log, repeat, antiwithdrawal, interceptor, aichat, news)
bot/utils/               Command parsing, message extraction, URL helpers, time formatting
custom/                  User-created plugin examples and templates
web/                     Admin panel frontend (Vite + Vue 3 + Tailwind v4, builds into bot/adminpanel/dist)
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
- **AI scheduled tasks (clock)** — `pluginaichat/clock.go`'s `clockManager` owns its **own `*cron.Cron`** (separate from the framework's `StartCron` cron) and persists dynamic, AI/user-managed tasks to `PersistentStorage` (`clock:` namespace). On trigger it runs a **fresh ephemeral ChatBot** (nil history store → discarded after) with full tools, under a per-task timeout, logged via `bot/component/tasklog`. Trigger prompt = `【定时任务】<title>\n<content>`. On trigger execution, the `SendText` callback **discards** intermediate-round text (logs only); only the **final reply** is sent to the trigger target, while tools' own image/file sends (`SendImage`/`SendFile`, e.g. `meme`/`file`) still go through. Users manage via `/clock`; AI manages via `clock_create/list/update/delete/log` tools (registered per session in `getChat`). Gated by `plugin.ai_chat_bot.clock.enable`.
- **AI long-term memory** — `pluginaichat/memory.go`'s `memoryManager` persists AI-managed long-term memories to `PersistentStorage` (`memory:` namespace), one JSON array per session scope (`g:<group>` / `f:<qq>`, whole-array read/write fits KV semantics; hundreds of entries per scope). The AI manages memories itself via `memory_save`/`memory_search`/`memory_forget` tools (registered per session in `getChat` with the scope bound at registration, so memories can never leak across groups/chats). `memory_search` returns all entries in scope when no query is given (newest first); with a query it keyword-scores entries (tag hits weigh more than content hits), drops zero-score entries, and sorts by score. Identical content is deduped; `memory.max_entries` (default 200) caps per-scope count. Gated by `plugin.ai_chat_bot.memory.enable`. The plugin also implements the panel's optional `adminpanel.MemorySource` interface (`memorypanel.go`), so the 记忆管理 page can list scopes and create/update/delete entries with live effect (scope input is validated against `^[gf]:\d+$` to stay inside the `memory:` namespace).
- **AI subagents** — the `subagent_run` session tool (registered per session in `getChat`, gated by `plugin.ai_chat_bot.subagent.enable`) lets the main AI delegate a complex/time-consuming subtask to an **ephemeral sub-agent**: a fresh `ChatBot` (nil history store → discarded after) with its own `SessionToolExecutor`, running synchronously inside the parent's tool call. The sub-agent gets the same session-scoped tools (clock/memory via the shared `registerScopedTools` helper) but **not** `subagent_run` itself (no recursive delegation). Its intermediate-round `SendText` is discarded (logged only) while image/file sends and history reads pass through to the current session; the image-loading callbacks are NOT inherited (they close over per-request `loadedImages`/`loaded` state and the two tool loops would drain the same queue) — following the `makeClockCallback` pattern, the sub-agent gets its own image queue for `LoadLocalImage` and a stub `LoadImages` deferring to the main AI. Only the final reply returns to the parent as the tool result, prefixed with run metadata (duration, LLM iterations, tokens) and rune-truncated at `subagent.max_result_len` (default 4000) to protect the main context. The sub-agent's tool loop is capped at `subagent.max_iterations` (default 10) via `ChatBot.SetMaxIterations`. Timeout resolution (`resolveSubagentTimeout`): `subagent.timeout_sec` default 300s, per-call override clamped as int **before** multiplying by `time.Second` (1800s cap, overflow-proof), then shrunk to fit the framework's per-message budget (`core.MsgEventTimeout`, 5min) minus a 30s reserve so the sub-agent's own deadline fires first and the timeout surfaces as a graceful tool result instead of killing the whole request; `/stop` still cancels it through the parent request context.

### Message Chain Builder

Fluent builder pattern for constructing bot responses:

```go
msgchain.Builder().Group(target).Text("hello").Mention(qid).Build()
msgchain.Builder().GroupForward(target).Node(sender, content).Build()
```

### Storage

AniaBot exposes two interface-adapted storage layers, both with Clone-based `base64(pluginName)` namespacing injected per plugin:

- **CACHE** (`common/storage.Storage`, injected as `p.Storage`): volatile; supports TTL + Redis-list semantics. Backends: `memory` (default, process-local) | `redis` (opt-in, shared across instances).
- **PERSISTENT** (`common/storage.PersistentStorage`, injected as `p.PersistentStorage`): durable KV/document store, survives restart, no TTL/list semantics. Backends: `sqlite` (default) | `mysql` (opt-in).

All SQL backends use pure-Go drivers (`modernc.org/sqlite`, `github.com/go-sql-driver/mysql`) — no CGO, cross-compile friendly. Cache config lives under `bot.store.cache` in the DB config; the persistent backend itself is bootstrapped via env vars (`ANIABOT_STORE_DRIVER` / `ANIABOT_SQLITE_PATH` / `ANIABOT_MYSQL_DSN`); factories in `bot/core/storage_factory.go`.

## Key Conventions

- **Language**: Code comments and user-facing strings are in Chinese
- **Logging**: Mixed `log/slog` (core/plugins) and `log.Printf` (adapter/tools). Use `slog` for new code.
- **Error handling**: Adapter/storage methods return `(value, bool)` not `(value, error)`. Use `fmt.Errorf` with `%w` wrapping in component-layer code.
- **Package naming**: Lowercase, concatenated (e.g., `pluginaichat`, `llmtool`, `functool`, `aniaerror`)
- **No linting config**: CI runs `go vet ./...` only. No golangci-lint.
- **Generics**: Used for `BaseTool[T]`, `MessageQueue[T]`, `safeExecuteWithReturn[T]`
- **Functional options**: `Option func(*AniaBot)` pattern for bot configuration
- **OneBot v11**: All QQ message types use the OneBot v11 segment format (`OB11Segment`)
- **Changelog**: Every code change (feature, fix, refactor) must be recorded in `CHANGELOG.md` as part of the same change — add entries under the current unreleased/next version section, following the existing format

## CI/CD

Four GitHub Actions workflows in `.github/workflows/`:

- **test.yaml** — runs on push/PR to `main`: `go vet`, `go test -v -race -coverprofile=coverage.out ./...`
- **release.yaml** — runs on version tags (`v*.*.*`): tests, then creates the GitHub Release with the body extracted from the tag's section in `CHANGELOG.md` (falls back to conventional-commit generation if the section is missing) — update `CHANGELOG.md` before tagging
- **docs.yaml** — builds VitePress docs and deploys to GitHub Pages on `main` push
- **docker.yaml** — builds and pushes the AniaBot Docker image: version + short-sha tags on `v*.*.*` tag pushes, and `latest` on manual `workflow_dispatch`; the image's `org.opencontainers.image.description` label is taken from the corresponding `CHANGELOG.md` section

## External Dependencies

| Dependency                       | Purpose                                    |
| -------------------------------- | ------------------------------------------ |
| `openai-go/v3`                   | OpenAI-compatible LLM API client           |
| `modelcontextprotocol/go-sdk`    | MCP protocol client                        |
| `gorilla/websocket`              | WebSocket for NapCat adapter               |
| `go-resty/resty/v2`              | HTTP client                                |
| `redis/go-redis/v9`              | Redis cache storage backend                |
| `modernc.org/sqlite`             | Pure-Go SQLite, persistent storage default |
| `github.com/go-sql-driver/mysql` | Pure-Go MySQL, persistent storage opt-in   |
| `robfig/cron/v3`                 | Cron scheduling for timed plugins          |
| `spf13/viper`                    | In-memory config view for plugins           |
| `lmittmann/tint`                 | Colored slog handler                       |