// Package adminpanel 提供 AniaBot 的 Web 控制面板。
//
// 面板由后端 API（纯 net/http，零额外依赖）与内嵌的 Vue SPA（go:embed dist）
// 组成，功能包括：配置管理（读取/修改配置中心，重启后生效）、运行状态总览、
// 插件列表、群/好友列表与 AI 定时任务管理（列表 / 启停）与执行日志。
//
// 认证：首次启动生成随机初始密码打印到控制台，SHA-256+salt 哈希存于持久化
// 存储的 __admin 命名空间；登录后签发内存会话（HttpOnly Cookie，24h 过期）。
//
// 注意：使用独立的 http.ServeMux，绝不注册到 http.DefaultServeMux
// （NapCat HTTP 适配器占用了默认 mux 的 / 路由）。
package adminpanel

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/consollog"
	"github.com/jeanhua/AniaBot/bot/component/msglog"
	"github.com/jeanhua/AniaBot/bot/component/querylog"
	"github.com/jeanhua/AniaBot/bot/component/tasklog"
	"github.com/jeanhua/AniaBot/bot/core/configstore"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/storage"
)

//go:embed dist
var distFS embed.FS

// BotInfo 面板需要的 Bot 运行信息（由 *core.AniaBot 实现，避免 import 环）。
type BotInfo interface {
	GetPluginList() []plugininfo.PluginInfo
	GetGroupList() (*[]message.GroupInfo, bool)
	GetFriendList() (*[]message.Friend, bool)
	GoroutineNum() int32
	StartTime() time.Time
}

// TaskLogSource 可选接口：插件实现后，面板「任务日志」页可按条件查询其定时任务
// 执行日志（当前由 AI 对话插件的 clock 功能实现）。
type TaskLogSource interface {
	TaskLogQuery(f tasklog.Filter) []tasklog.Entry
}

// ClockTaskSource 可选接口：插件实现后，面板可对其定时任务做增删改查与启停
// （当前由 AI 对话插件的 clock 功能实现）。
type ClockTaskSource interface {
	ClockTasks() []plugininfo.ClockTaskInfo
	CreateClockTask(t plugininfo.ClockTaskCreate) (string, error)
	UpdateClockTask(id string, f plugininfo.ClockTaskUpdate) error
	DeleteClockTask(id string) error
}

// MsgLogSource 可选接口：插件实现后，面板「消息日志」页可展示其记录的
// 群消息 / 好友消息 / 通知事件（当前由日志打印插件实现，内存环形缓冲）。
// beforeID>0 时仅返回 ID 小于它的更旧日志（滚动分页游标）。
type MsgLogSource interface {
	MsgLogPage(limit int, beforeID uint64) []msglog.Entry
}

// SkillSource 可选接口：插件实现后，面板可对其 AI skill 做列表 / 上传 / 删除
// （当前由 AI 对话插件实现）。上传/删除后插件负责热重载 skill 注册表。
type SkillSource interface {
	// SkillList 返回当前已加载的 skill 列表、skills 目录与白名单（空表示加载全部）
	SkillList() (skills []plugininfo.SkillInfo, dir string, whitelist []string)
	// SkillDelete 按名称删除 skill（同时从磁盘移除）并热重载
	SkillDelete(name string) error
	// SkillUpload 从 zip 压缩包内容安装 skill 并热重载，filename 为原始文件名
	SkillUpload(filename string, data []byte) error
}

// MemorySource 可选接口：插件实现后，面板「记忆管理」页可对其 AI 长期记忆
// 做列表 / 新增 / 编辑 / 删除（当前由 AI 对话插件实现）。改动即时生效，无需重启。
type MemorySource interface {
	// MemoryScopes 返回已有记忆的会话 scope 列表及各自条数
	MemoryScopes() []plugininfo.MemoryScopeInfo
	// MemoryList 返回指定 scope 的全部记忆（新在前）
	MemoryList(scope string) ([]plugininfo.MemoryEntryInfo, error)
	// MemoryCreate 新增一条记忆，返回生成的 ID
	MemoryCreate(up plugininfo.MemoryEntryUpsert) (string, error)
	// MemoryUpdate 按 ID 更新一条记忆
	MemoryUpdate(up plugininfo.MemoryEntryUpsert) error
	// MemoryDelete 按 ID 删除一条记忆
	MemoryDelete(scope, id string) error
}

// QueryLogSource 可选接口：插件实现后，面板「Query 日志」页可展示每次 AI 回复
// 的完整执行记录（耗时、token 用量、工具调用详情）（当前由 AI 对话插件实现）。
type QueryLogSource interface {
	QueryLogRecent(f querylog.Filter) []querylog.Entry
}

// KnowledgeBaseSource 可选接口：插件实现后，面板「知识库」页可对其知识库文档
// 做列表 / 新增 / 编辑 / 删除 / URL 导入（当前由 AI 对话插件实现）。改动即时生效，无需重启。
type KnowledgeBaseSource interface {
	// KnowledgeScopes 返回已有知识库的作用域列表及各自文档条数
	KnowledgeScopes() []plugininfo.KnowledgeScopeInfo
	// KnowledgeList 返回指定 scope 的全部文档（新在前）
	KnowledgeList(scope string) ([]plugininfo.KnowledgeDocInfo, error)
	// KnowledgeCreate 新增一条文档，返回生成的 ID
	KnowledgeCreate(up plugininfo.KnowledgeDocUpsert) (string, error)
	// KnowledgeUpdate 按 ID 更新一条文档
	KnowledgeUpdate(up plugininfo.KnowledgeDocUpsert) error
	// KnowledgeDelete 按 ID 删除一条文档
	KnowledgeDelete(scope, id string) error
	// KnowledgeImportURL 抓取网页正文导入知识库，返回生成的 ID
	KnowledgeImportURL(scope, url string) (string, error)
}

// Options 面板依赖。
type Options struct {
	Listen        string                                             // 监听地址，如 127.0.0.1:7700
	Config        *configstore.Store                                 // 配置中心
	Persistent    storage.PersistentStorage                          // 根持久化存储（__admin 命名空间存密码哈希）
	Bot           BotInfo                                            // 运行信息来源
	Adapter       func() string                                      // 适配器连接状态描述
	AdapterDetail func() string                                      // 适配器状态详情（最近错误/重试次数，可为 nil）
	TaskLogs      func(f tasklog.Filter) []tasklog.Entry             // AI 定时任务执行日志（可为 nil）
	Clocks        ClockTaskSource                                    // AI 定时任务列表与启停（可为 nil）
	MsgLogs       func(limit int, beforeID uint64) []msglog.Entry    // 消息日志（群/好友/通知，可为 nil）
	Skills        SkillSource                                        // AI skill 管理（可为 nil）
	Memories      MemorySource                                       // AI 长期记忆管理（可为 nil）
	Knowledge     KnowledgeBaseSource                                // AI 知识库管理（可为 nil）
	QueryLogs     func(f querylog.Filter) []querylog.Entry           // AI Query 日志（可为 nil）
	ConsoleLogs   func(limit int, beforeID uint64) []consollog.Entry // 控制台日志（slog + log 输出，可为 nil）
	Logger        *slog.Logger
}

// Server 面板 HTTP 服务。
type Server struct {
	opt     Options
	auth    *authManager
	mux     *http.ServeMux
	started time.Time
	balance balanceCache
}

// NewServer 创建面板服务。Options.Listen 为空时默认 127.0.0.1:7700。
func NewServer(opt Options) *Server {
	if opt.Listen == "" {
		opt.Listen = "127.0.0.1:7700"
	}
	if opt.Logger == nil {
		opt.Logger = slog.Default()
	}
	s := &Server{opt: opt, mux: http.NewServeMux(), started: time.Now()}
	s.auth = newAuthManager(opt.Persistent, opt.Logger)
	s.routes()
	startCPUSampler() // 后台持续采样 CPU，为负载图提供服务端缓存的历史曲线
	return s
}

// Run 启动 HTTP 服务（阻塞），通常以 goroutine 调用。
func (s *Server) Run() {
	s.opt.Logger.Info("Web 控制面板已启动", "listen", s.opt.Listen, "url", "http://"+s.opt.Listen)
	srv := &http.Server{
		Addr:              s.opt.Listen,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.opt.Logger.Error("Web 控制面板启动失败", "error", err)
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)
	s.mux.Handle("GET /api/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	s.mux.Handle("POST /api/setup/complete", s.requireAuth(http.HandlerFunc(s.handleSetupComplete)))
	s.mux.Handle("PUT /api/password", s.requireAuth(http.HandlerFunc(s.handleChangePassword)))
	s.mux.Handle("GET /api/config/schema", s.requireAuth(http.HandlerFunc(s.handleConfigSchema)))
	s.mux.Handle("GET /api/config", s.requireAuth(http.HandlerFunc(s.handleConfigGet)))
	s.mux.Handle("PUT /api/config", s.requireAuth(http.HandlerFunc(s.handleConfigPut)))
	s.mux.Handle("GET /api/config/presets", s.requireAuth(http.HandlerFunc(s.handlePresetList)))
	s.mux.Handle("POST /api/config/presets", s.requireAuth(http.HandlerFunc(s.handlePresetSave)))
	s.mux.Handle("POST /api/config/presets/{name}/apply", s.requireAuth(http.HandlerFunc(s.handlePresetApply)))
	s.mux.Handle("DELETE /api/config/presets/{name}", s.requireAuth(http.HandlerFunc(s.handlePresetDelete)))
	s.mux.Handle("GET /api/files/{name}", s.requireAuth(http.HandlerFunc(s.handleFileGet)))
	s.mux.Handle("PUT /api/files/{name}", s.requireAuth(http.HandlerFunc(s.handleFilePut)))
	s.mux.Handle("GET /api/status", s.requireAuth(http.HandlerFunc(s.handleStatus)))
	s.mux.Handle("GET /api/host", s.requireAuth(http.HandlerFunc(s.handleHost)))
	s.mux.Handle("GET /api/plugins", s.requireAuth(http.HandlerFunc(s.handlePlugins)))
	s.mux.Handle("GET /api/groups", s.requireAuth(http.HandlerFunc(s.handleGroups)))
	s.mux.Handle("GET /api/friends", s.requireAuth(http.HandlerFunc(s.handleFriends)))
	s.mux.Handle("GET /api/tasklogs", s.requireAuth(http.HandlerFunc(s.handleTaskLogs)))
	s.mux.Handle("GET /api/msglogs", s.requireAuth(http.HandlerFunc(s.handleMsgLogs)))
	s.mux.Handle("GET /api/querylogs", s.requireAuth(http.HandlerFunc(s.handleQueryLogs)))
	s.mux.Handle("GET /api/consolelogs", s.requireAuth(http.HandlerFunc(s.handleConsoleLogs)))
	s.mux.Handle("GET /api/tokenstats", s.requireAuth(http.HandlerFunc(s.handleTokenStats)))
	s.mux.Handle("GET /api/balance", s.requireAuth(http.HandlerFunc(s.handleBalance)))
	s.mux.Handle("GET /api/tokenstats/detail", s.requireAuth(http.HandlerFunc(s.handleTokenStatsDetail)))
	s.mux.Handle("GET /api/clocks", s.requireAuth(http.HandlerFunc(s.handleClockList)))
	s.mux.Handle("POST /api/clocks", s.requireAuth(http.HandlerFunc(s.handleClockCreate)))
	s.mux.Handle("PUT /api/clocks/{id}", s.requireAuth(http.HandlerFunc(s.handleClockUpdate)))
	s.mux.Handle("DELETE /api/clocks/{id}", s.requireAuth(http.HandlerFunc(s.handleClockDelete)))
	s.mux.Handle("GET /api/skills", s.requireAuth(http.HandlerFunc(s.handleSkillList)))
	s.mux.Handle("POST /api/skills", s.requireAuth(http.HandlerFunc(s.handleSkillUpload)))
	s.mux.Handle("DELETE /api/skills/{name}", s.requireAuth(http.HandlerFunc(s.handleSkillDelete)))
	s.mux.Handle("GET /api/memory/scopes", s.requireAuth(http.HandlerFunc(s.handleMemoryScopes)))
	s.mux.Handle("GET /api/memory/list", s.requireAuth(http.HandlerFunc(s.handleMemoryList)))
	s.mux.Handle("POST /api/memory", s.requireAuth(http.HandlerFunc(s.handleMemoryCreate)))
	s.mux.Handle("PUT /api/memory", s.requireAuth(http.HandlerFunc(s.handleMemoryUpdate)))
	s.mux.Handle("DELETE /api/memory", s.requireAuth(http.HandlerFunc(s.handleMemoryDelete)))
	s.mux.Handle("GET /api/knowledge/scopes", s.requireAuth(http.HandlerFunc(s.handleKnowledgeScopes)))
	s.mux.Handle("GET /api/knowledge/list", s.requireAuth(http.HandlerFunc(s.handleKnowledgeList)))
	s.mux.Handle("POST /api/knowledge", s.requireAuth(http.HandlerFunc(s.handleKnowledgeCreate)))
	s.mux.Handle("PUT /api/knowledge", s.requireAuth(http.HandlerFunc(s.handleKnowledgeUpdate)))
	s.mux.Handle("DELETE /api/knowledge", s.requireAuth(http.HandlerFunc(s.handleKnowledgeDelete)))
	s.mux.Handle("POST /api/knowledge/import-url", s.requireAuth(http.HandlerFunc(s.handleKnowledgeImportURL)))
	s.mux.Handle("POST /api/restart", s.requireAuth(http.HandlerFunc(s.handleRestart)))
	s.mux.Handle("GET /api/update/info", s.requireAuth(http.HandlerFunc(s.handleUpdateInfo)))
	s.mux.Handle("POST /api/update/start", s.requireAuth(http.HandlerFunc(s.handleUpdateStart)))
	s.mux.Handle("GET /api/update/status", s.requireAuth(http.HandlerFunc(s.handleUpdateStatus)))
	s.mux.Handle("/", s.spaHandler())
}

// spaHandler 提供内嵌的前端静态资源，未命中路径回退到 index.html（SPA 路由）。
func (s *Server) spaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// requireAuth 校验会话 Cookie；会话被滑动续期时同步刷新 Cookie 过期时间。
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "未登录或会话已过期")
			return
		}
		valid, renewed := s.auth.ValidSession(cookie.Value)
		if !valid {
			writeError(w, http.StatusUnauthorized, "未登录或会话已过期")
			return
		}
		if renewed {
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    cookie.Value,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Expires:  time.Now().Add(sessionTTL),
			})
		}
		next.ServeHTTP(w, r)
	})
}

// ---- auth handlers ----

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if !s.auth.CheckPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	token := s.auth.NewSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.auth.DropSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"setup_required": s.opt.Config.SetupPending(),
	})
}

// handleSetupComplete 标记首次设置向导完成（或跳过）。
func (s *Server) handleSetupComplete(w http.ResponseWriter, _ *http.Request) {
	s.opt.Config.CompleteSetup()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(req.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "新密码长度至少 6 位")
		return
	}
	if !s.auth.SetPassword(req.NewPassword) {
		writeError(w, http.StatusInternalServerError, "密码保存失败")
		return
	}
	// 修改密码后销毁所有会话（含当前），强制使用新密码重新登录
	s.auth.DropAllSessions()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- config handlers ----

func (s *Server) handleConfigSchema(w http.ResponseWriter, _ *http.Request) {
	// 表单元信息来自配置注册表（框架 + 各插件 ConfigRegistrar 动态注册），
	// 新增/移除插件无需改动面板代码。
	writeJSON(w, http.StatusOK, pluginconfig.Fields())
}

func (s *Server) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	all := s.opt.Config.All()
	for _, f := range pluginconfig.Fields() {
		if !f.Sensitive {
			continue
		}
		if v, ok := all[f.Key]; ok {
			if str, ok2 := v.(string); ok2 && str != "" {
				all[f.Key] = maskPlaceholder
			}
		}
	}
	writeJSON(w, http.StatusOK, all)
}

func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	var updates map[string]any
	if err := readJSON(r, &updates); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误（需要 JSON 对象）")
		return
	}
	for k, v := range updates {
		// 敏感字段传回掩码占位符表示未修改，跳过
		if str, ok := v.(string); ok && str == maskPlaceholder {
			delete(updates, k)
			continue
		}
		// 面板关键配置不允许置空 listen
		if k == "bot.admin_panel.listen" {
			if str, ok := v.(string); !ok || str == "" {
				delete(updates, k)
			}
		}
	}
	if len(updates) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "need_restart": true})
		return
	}
	if err := s.opt.Config.SetMany(updates); err != nil {
		writeError(w, http.StatusInternalServerError, "配置保存失败: "+err.Error())
		return
	}
	s.opt.Logger.Info("配置已通过 Web 面板更新", "keys", len(updates))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "need_restart": true})
}

// ---- file handlers（MCP / Prompt 覆盖 JSON） ----

var fileKeys = map[string]string{
	"mcp":    configstore.KeyMCPJSON,
	"prompt": configstore.KeyPromptJSON,
}

func (s *Server) handleFileGet(w http.ResponseWriter, r *http.Request) {
	key, ok := fileKeys[r.PathValue("name")]
	if !ok {
		writeError(w, http.StatusNotFound, "未知文件")
		return
	}
	v, _ := s.opt.Config.Get(key)
	str, _ := v.(string)
	writeJSON(w, http.StatusOK, map[string]string{"content": str})
}

func (s *Server) handleFilePut(w http.ResponseWriter, r *http.Request) {
	key, ok := fileKeys[r.PathValue("name")]
	if !ok {
		writeError(w, http.StatusNotFound, "未知文件")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	// 非空内容必须是合法 JSON
	if strings.TrimSpace(req.Content) != "" && !json.Valid([]byte(req.Content)) {
		writeError(w, http.StatusBadRequest, "内容不是合法的 JSON")
		return
	}
	if err := s.opt.Config.Set(key, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "need_restart": true})
}

// ---- status / list handlers ----

type pluginDTO struct {
	Name      string `json:"name"`
	HelpWords string `json:"help_words"`
	AdminOnly bool   `json:"admin_only"`
	Author    string `json:"author"`
	Version   string `json:"version"`
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	adapterStatus := "unknown"
	if s.opt.Adapter != nil {
		adapterStatus = s.opt.Adapter()
	}
	adapterDetail := ""
	if s.opt.AdapterDetail != nil {
		adapterDetail = s.opt.AdapterDetail()
	}
	uptime := time.Since(s.opt.Bot.StartTime())
	writeJSON(w, http.StatusOK, map[string]any{
		"uptime_sec":     int64(uptime.Seconds()),
		"started_at":     s.opt.Bot.StartTime().Format(time.RFC3339),
		"goroutines":     s.opt.Bot.GoroutineNum(),
		"adapter_status": adapterStatus,
		"adapter_detail": adapterDetail,
		"plugin_count":   len(s.opt.Bot.GetPluginList()),
	})
}

// handleHost 返回主机硬件配置与运行状态（CPU / 内存占用、系统信息等）。
func (s *Server) handleHost(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, collectHost())
}

func (s *Server) handlePlugins(w http.ResponseWriter, _ *http.Request) {
	list := s.opt.Bot.GetPluginList()
	dtos := make([]pluginDTO, 0, len(list))
	for _, p := range list {
		dtos = append(dtos, pluginDTO{
			Name: p.Name, HelpWords: p.HelpWords,
			AdminOnly: p.AdminOnly, Author: p.Author, Version: p.Version,
		})
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (s *Server) handleGroups(w http.ResponseWriter, _ *http.Request) {
	groups, ok := s.opt.Bot.GetGroupList()
	if !ok || groups == nil {
		writeError(w, http.StatusBadGateway, "获取群列表失败（适配器未连接？）")
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (s *Server) handleFriends(w http.ResponseWriter, _ *http.Request) {
	friends, ok := s.opt.Bot.GetFriendList()
	if !ok || friends == nil {
		writeError(w, http.StatusBadGateway, "获取好友列表失败（适配器未连接？）")
		return
	}
	writeJSON(w, http.StatusOK, friends)
}

// handleTaskLogs 按条件分页查询定时任务执行日志（新在前）。
// 支持查询参数：target_type（group/friend）、target_id（群号/QQ）、task_id（任务 ID）、
// status（running/success/timeout/error）、start / end（RFC3339 或 datetime-local 格式）、
// keyword（匹配任务标题）、limit（每页条数，默认 50，最大 200）、
// before（分页游标：仅返回比该日志 ID 更旧的记录）。
// 响应：{"items": [...], "has_more": bool}。
func (s *Server) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	if s.opt.TaskLogs == nil {
		writePagedLogs(w, nil, false)
		return
	}
	q := r.URL.Query()
	f := tasklog.Filter{
		TargetType: q.Get("target_type"),
		TargetID:   q.Get("target_id"),
		TaskID:     q.Get("task_id"),
		Status:     tasklog.Status(q.Get("status")),
		Keyword:    q.Get("keyword"),
	}
	if f.TargetType != "" && f.TargetType != "group" && f.TargetType != "friend" {
		writeError(w, http.StatusBadRequest, "target_type 仅支持 group / friend")
		return
	}
	switch f.Status {
	case "", tasklog.StatusRunning, tasklog.StatusSuccess, tasklog.StatusTimeout, tasklog.StatusError:
	default:
		writeError(w, http.StatusBadRequest, "status 仅支持 running / success / timeout / error")
		return
	}
	if v := q.Get("start"); v != "" {
		t, err := parseQueryTime(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "start 时间格式错误（支持 RFC3339 或 2006-01-02T15:04）")
			return
		}
		f.Start = t
	}
	if v := q.Get("end"); v != "" {
		t, err := parseQueryTime(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "end 时间格式错误（支持 RFC3339 或 2006-01-02T15:04）")
			return
		}
		f.End = t
	}
	if v := q.Get("before"); v != "" {
		if _, err := strconv.ParseUint(v, 36, 64); err != nil {
			writeError(w, http.StatusBadRequest, "before 游标格式错误（应为日志 ID）")
			return
		}
		f.Before = v
	}
	limit := parsePageLimit(q.Get("limit"))
	// 多取一条判断是否还有下一页
	f.Limit = limit + 1
	items := s.opt.TaskLogs(f)
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	writePagedLogs(w, items, hasMore)
}

// handleMsgLogs 分页返回消息日志（群/好友/通知，新在前）。
// 支持查询参数：limit（每页条数，默认 50，最大 200）、
// before（分页游标：仅返回 ID 小于它的更旧日志）。
// 响应：{"items": [...], "has_more": bool}。
func (s *Server) handleMsgLogs(w http.ResponseWriter, r *http.Request) {
	if s.opt.MsgLogs == nil {
		writePagedLogs(w, nil, false)
		return
	}
	q := r.URL.Query()
	limit := parsePageLimit(q.Get("limit"))
	var before uint64
	if v := q.Get("before"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "before 游标格式错误（应为数字日志 ID）")
			return
		}
		before = n
	}
	items := s.opt.MsgLogs(limit+1, before)
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	writePagedLogs(w, items, hasMore)
}

// handleQueryLogs 按条件分页查询 AI Query 日志（新在前）。
// 支持查询参数：chat_type（group/friend）、target_id（群号/QQ）、sender（触发人 QQ）、
// start / end（RFC3339 或 datetime-local 格式）、keyword（匹配用户输入）、
// limit（每页条数，默认 50，最大 200）、before（分页游标：仅返回比该日志 ID 更旧的记录）。
// 响应：{"items": [...], "has_more": bool}。
func (s *Server) handleQueryLogs(w http.ResponseWriter, r *http.Request) {
	if s.opt.QueryLogs == nil {
		writePagedLogs(w, nil, false)
		return
	}
	q := r.URL.Query()
	f := querylog.Filter{
		ChatType: q.Get("chat_type"),
		TargetID: q.Get("target_id"),
		Sender:   q.Get("sender"),
		Keyword:  q.Get("keyword"),
	}
	if f.ChatType != "" && f.ChatType != "group" && f.ChatType != "friend" {
		writeError(w, http.StatusBadRequest, "chat_type 仅支持 group / friend")
		return
	}
	if v := q.Get("start"); v != "" {
		t, err := parseQueryTime(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "start 时间格式错误（支持 RFC3339 或 2006-01-02T15:04）")
			return
		}
		f.Start = t
	}
	if v := q.Get("end"); v != "" {
		t, err := parseQueryTime(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "end 时间格式错误（支持 RFC3339 或 2006-01-02T15:04）")
			return
		}
		f.End = t
	}
	if v := q.Get("before"); v != "" {
		if _, err := strconv.ParseUint(v, 36, 64); err != nil {
			writeError(w, http.StatusBadRequest, "before 游标格式错误（应为日志 ID）")
			return
		}
		f.Before = v
	}
	limit := parsePageLimit(q.Get("limit"))
	f.Limit = limit + 1
	items := s.opt.QueryLogs(f)
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	writePagedLogs(w, items, hasMore)
}

// handleConsoleLogs 分页返回控制台日志（slog 结构化日志与标准库 log 输出，
// 新在前）。支持查询参数：limit（每页条数，默认 50，最大 200）、
// before（分页游标：仅返回 ID 小于它的更旧日志）。
// 响应：{"items": [...], "has_more": bool}。
func (s *Server) handleConsoleLogs(w http.ResponseWriter, r *http.Request) {
	if s.opt.ConsoleLogs == nil {
		writePagedLogs(w, nil, false)
		return
	}
	q := r.URL.Query()
	limit := parsePageLimit(q.Get("limit"))
	var before uint64
	if v := q.Get("before"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "before 游标格式错误（应为数字日志 ID）")
			return
		}
		before = n
	}
	items := s.opt.ConsoleLogs(limit+1, before)
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	writePagedLogs(w, items, hasMore)
}

// parsePageLimit 解析分页大小：默认 50，最大 200；非法值取默认。
func parsePageLimit(v string) int {
	const (
		def = 50
		max = 200
	)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// writePagedLogs 输出分页日志响应；items 为 nil 时序列化为空数组而非 null。
func writePagedLogs(w http.ResponseWriter, items any, hasMore bool) {
	if items == nil {
		items = []any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":    items,
		"has_more": hasMore,
	})
}

// parseQueryTime 解析面板传来的时间：RFC3339，或 datetime-local 控件的
// "2006-01-02T15:04"（按本地时区解释）。
func parseQueryTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02T15:04", v, time.Local)
}

// handleClockList 返回 AI 定时任务列表（功能未启用时返回空数组）。
func (s *Server) handleClockList(w http.ResponseWriter, _ *http.Request) {
	if s.opt.Clocks == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	tasks := s.opt.Clocks.ClockTasks()
	if tasks == nil {
		tasks = []plugininfo.ClockTaskInfo{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// handleClockCreate 新建定时任务。
func (s *Server) handleClockCreate(w http.ResponseWriter, r *http.Request) {
	if s.opt.Clocks == nil {
		writeError(w, http.StatusNotFound, "定时任务功能未启用")
		return
	}
	var req plugininfo.ClockTaskCreate
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	id, err := s.opt.Clocks.CreateClockTask(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.opt.Logger.Info("定时任务已通过 Web 面板创建", "task", id, "title", req.Title)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// handleClockUpdate 编辑定时任务（仅更新请求体中提供的字段，含启用/停用）。
func (s *Server) handleClockUpdate(w http.ResponseWriter, r *http.Request) {
	if s.opt.Clocks == nil {
		writeError(w, http.StatusNotFound, "定时任务功能未启用")
		return
	}
	var req plugininfo.ClockTaskUpdate
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Cron == nil && req.Title == nil && req.Content == nil && req.Note == nil &&
		req.TimeoutSec == nil && req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "没有需要更新的字段")
		return
	}
	id := r.PathValue("id")
	if err := s.opt.Clocks.UpdateClockTask(id, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.opt.Logger.Info("定时任务已通过 Web 面板更新", "task", id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleClockDelete 删除定时任务。
func (s *Server) handleClockDelete(w http.ResponseWriter, r *http.Request) {
	if s.opt.Clocks == nil {
		writeError(w, http.StatusNotFound, "定时任务功能未启用")
		return
	}
	id := r.PathValue("id")
	if err := s.opt.Clocks.DeleteClockTask(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.opt.Logger.Info("定时任务已通过 Web 面板删除", "task", id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- skill handlers（AI skill 管理） ----

// skillUploadMaxBytes 限制上传的 zip 压缩包大小
const skillUploadMaxBytes = 32 << 20 // 32 MiB

// handleSkillList 返回 skill 列表、skills 目录与白名单（功能未启用时返回空）。
func (s *Server) handleSkillList(w http.ResponseWriter, _ *http.Request) {
	if s.opt.Skills == nil {
		writeJSON(w, http.StatusOK, map[string]any{"skills": []any{}, "dir": "", "whitelist": []string{}})
		return
	}
	skills, dir, whitelist := s.opt.Skills.SkillList()
	if skills == nil {
		skills = []plugininfo.SkillInfo{}
	}
	if whitelist == nil {
		whitelist = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"skills":    skills,
		"dir":       dir,
		"whitelist": whitelist,
	})
}

// handleSkillUpload 接收 multipart 表单中的 zip 压缩包（字段名 file），安装为 skill。
func (s *Server) handleSkillUpload(w http.ResponseWriter, r *http.Request) {
	if s.opt.Skills == nil {
		writeError(w, http.StatusNotFound, "skill 功能未启用")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, skillUploadMaxBytes)
	if err := r.ParseMultipartForm(skillUploadMaxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "解析上传内容失败（文件过大？上限 32MB）")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少上传文件（字段名 file）")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取上传文件失败")
		return
	}
	if err := s.opt.Skills.SkillUpload(header.Filename, data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.opt.Logger.Info("skill 已通过 Web 面板上传", "file", header.Filename)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleSkillDelete 按名称删除 skill（同时从磁盘移除）。
func (s *Server) handleSkillDelete(w http.ResponseWriter, r *http.Request) {
	if s.opt.Skills == nil {
		writeError(w, http.StatusNotFound, "skill 功能未启用")
		return
	}
	name := r.PathValue("name")
	if err := s.opt.Skills.SkillDelete(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.opt.Logger.Info("skill 已通过 Web 面板删除", "skill", name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- memory handlers（AI 长期记忆管理） ----

// handleMemoryScopes 返回已有记忆的会话 scope 列表及条数（功能未启用时返回空数组）。
func (s *Server) handleMemoryScopes(w http.ResponseWriter, _ *http.Request) {
	if s.opt.Memories == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	scopes := s.opt.Memories.MemoryScopes()
	if scopes == nil {
		scopes = []plugininfo.MemoryScopeInfo{}
	}
	writeJSON(w, http.StatusOK, scopes)
}

// handleMemoryList 返回指定 scope（query 参数 scope）的全部记忆。
func (s *Server) handleMemoryList(w http.ResponseWriter, r *http.Request) {
	if s.opt.Memories == nil {
		writeError(w, http.StatusNotFound, "记忆功能未启用")
		return
	}
	entries, err := s.opt.Memories.MemoryList(r.URL.Query().Get("scope"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if entries == nil {
		entries = []plugininfo.MemoryEntryInfo{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// handleMemoryCreate 新增一条记忆。
func (s *Server) handleMemoryCreate(w http.ResponseWriter, r *http.Request) {
	if s.opt.Memories == nil {
		writeError(w, http.StatusNotFound, "记忆功能未启用")
		return
	}
	var req plugininfo.MemoryEntryUpsert
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	id, err := s.opt.Memories.MemoryCreate(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// handleMemoryUpdate 按 ID 更新一条记忆。
func (s *Server) handleMemoryUpdate(w http.ResponseWriter, r *http.Request) {
	if s.opt.Memories == nil {
		writeError(w, http.StatusNotFound, "记忆功能未启用")
		return
	}
	var req plugininfo.MemoryEntryUpsert
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.opt.Memories.MemoryUpdate(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMemoryDelete 按 ID 删除一条记忆（query 参数 scope 与 id）。
func (s *Server) handleMemoryDelete(w http.ResponseWriter, r *http.Request) {
	if s.opt.Memories == nil {
		writeError(w, http.StatusNotFound, "记忆功能未启用")
		return
	}
	q := r.URL.Query()
	if err := s.opt.Memories.MemoryDelete(q.Get("scope"), q.Get("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- knowledge handlers（AI 知识库管理） ----

// handleKnowledgeScopes 返回已有知识库的作用域列表及文档条数（功能未启用时返回空数组）。
func (s *Server) handleKnowledgeScopes(w http.ResponseWriter, _ *http.Request) {
	if s.opt.Knowledge == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	scopes := s.opt.Knowledge.KnowledgeScopes()
	if scopes == nil {
		scopes = []plugininfo.KnowledgeScopeInfo{}
	}
	writeJSON(w, http.StatusOK, scopes)
}

// handleKnowledgeList 返回指定 scope（query 参数 scope）的全部文档。
func (s *Server) handleKnowledgeList(w http.ResponseWriter, r *http.Request) {
	if s.opt.Knowledge == nil {
		writeError(w, http.StatusNotFound, "知识库功能未启用")
		return
	}
	docs, err := s.opt.Knowledge.KnowledgeList(r.URL.Query().Get("scope"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if docs == nil {
		docs = []plugininfo.KnowledgeDocInfo{}
	}
	writeJSON(w, http.StatusOK, docs)
}

// handleKnowledgeCreate 新增一条知识库文档。
func (s *Server) handleKnowledgeCreate(w http.ResponseWriter, r *http.Request) {
	if s.opt.Knowledge == nil {
		writeError(w, http.StatusNotFound, "知识库功能未启用")
		return
	}
	var req plugininfo.KnowledgeDocUpsert
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	id, err := s.opt.Knowledge.KnowledgeCreate(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// handleKnowledgeUpdate 按 ID 更新一条知识库文档。
func (s *Server) handleKnowledgeUpdate(w http.ResponseWriter, r *http.Request) {
	if s.opt.Knowledge == nil {
		writeError(w, http.StatusNotFound, "知识库功能未启用")
		return
	}
	var req plugininfo.KnowledgeDocUpsert
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.opt.Knowledge.KnowledgeUpdate(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleKnowledgeDelete 按 ID 删除一条知识库文档（query 参数 scope 与 id）。
func (s *Server) handleKnowledgeDelete(w http.ResponseWriter, r *http.Request) {
	if s.opt.Knowledge == nil {
		writeError(w, http.StatusNotFound, "知识库功能未启用")
		return
	}
	q := r.URL.Query()
	if err := s.opt.Knowledge.KnowledgeDelete(q.Get("scope"), q.Get("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleKnowledgeImportURL 抓取网页正文导入知识库。
func (s *Server) handleKnowledgeImportURL(w http.ResponseWriter, r *http.Request) {
	if s.opt.Knowledge == nil {
		writeError(w, http.StatusNotFound, "知识库功能未启用")
		return
	}
	var req plugininfo.KnowledgeImportURLRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	id, err := s.opt.Knowledge.KnowledgeImportURL(req.Scope, req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// handleRestart 自重启 Bot：先响应请求，再延迟以相同命令行参数重启进程。
// 配置修改随之生效；面板会话持久化在数据库中，重启后无需重新登录。
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	go func() {
		time.Sleep(500 * time.Millisecond)
		restartSelf(s.opt.Logger)
	}()
}
