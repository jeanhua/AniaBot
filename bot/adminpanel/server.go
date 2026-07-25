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
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/msglog"
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

// TaskLogSource 可选接口：插件实现后，其定时任务执行日志会展示在面板上
// （当前由 AI 对话插件的 clock 功能实现）。
type TaskLogSource interface {
	TaskLogRecent(limit int) []tasklog.Entry
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
type MsgLogSource interface {
	MsgLogRecent(limit int) []msglog.Entry
}

// Options 面板依赖。
type Options struct {
	Listen        string                          // 监听地址，如 127.0.0.1:7700
	Config        *configstore.Store              // 配置中心
	Persistent    storage.PersistentStorage       // 根持久化存储（__admin 命名空间存密码哈希）
	Bot           BotInfo                         // 运行信息来源
	Adapter       func() string                   // 适配器连接状态描述
	AdapterDetail func() string                   // 适配器状态详情（最近错误/重试次数，可为 nil）
	TaskLogs      func(limit int) []tasklog.Entry // AI 定时任务执行日志（可为 nil）
	Clocks        ClockTaskSource                 // AI 定时任务列表与启停（可为 nil）
	MsgLogs       func(limit int) []msglog.Entry  // 消息日志（群/好友/通知，可为 nil）
	Logger        *slog.Logger
}

// Server 面板 HTTP 服务。
type Server struct {
	opt     Options
	auth    *authManager
	mux     *http.ServeMux
	started time.Time
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
	s.mux.Handle("GET /api/files/{name}", s.requireAuth(http.HandlerFunc(s.handleFileGet)))
	s.mux.Handle("PUT /api/files/{name}", s.requireAuth(http.HandlerFunc(s.handleFilePut)))
	s.mux.Handle("GET /api/status", s.requireAuth(http.HandlerFunc(s.handleStatus)))
	s.mux.Handle("GET /api/plugins", s.requireAuth(http.HandlerFunc(s.handlePlugins)))
	s.mux.Handle("GET /api/groups", s.requireAuth(http.HandlerFunc(s.handleGroups)))
	s.mux.Handle("GET /api/friends", s.requireAuth(http.HandlerFunc(s.handleFriends)))
	s.mux.Handle("GET /api/tasklogs", s.requireAuth(http.HandlerFunc(s.handleTaskLogs)))
	s.mux.Handle("GET /api/msglogs", s.requireAuth(http.HandlerFunc(s.handleMsgLogs)))
	s.mux.Handle("GET /api/clocks", s.requireAuth(http.HandlerFunc(s.handleClockList)))
	s.mux.Handle("POST /api/clocks", s.requireAuth(http.HandlerFunc(s.handleClockCreate)))
	s.mux.Handle("PUT /api/clocks/{id}", s.requireAuth(http.HandlerFunc(s.handleClockUpdate)))
	s.mux.Handle("DELETE /api/clocks/{id}", s.requireAuth(http.HandlerFunc(s.handleClockDelete)))
	s.mux.Handle("POST /api/restart", s.requireAuth(http.HandlerFunc(s.handleRestart)))
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

// requireAuth 校验会话 Cookie。
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !s.auth.ValidSession(cookie.Value) {
			writeError(w, http.StatusUnauthorized, "未登录或会话已过期")
			return
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
		OldPassword string `json:"old_password"`
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
	if !s.auth.CheckPassword(req.OldPassword) {
		writeError(w, http.StatusUnauthorized, "原密码错误")
		return
	}
	if !s.auth.SetPassword(req.NewPassword) {
		writeError(w, http.StatusInternalServerError, "密码保存失败")
		return
	}
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

func (s *Server) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	if s.opt.TaskLogs == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, s.opt.TaskLogs(200))
}

// handleMsgLogs 返回最近的消息日志（群/好友/通知，新在前）。
func (s *Server) handleMsgLogs(w http.ResponseWriter, r *http.Request) {
	if s.opt.MsgLogs == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, s.opt.MsgLogs(300))
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

// handleRestart 自重启 Bot：先响应请求，再延迟以相同命令行参数重启进程。
// 配置修改随之生效；面板会话持久化在数据库中，重启后无需重新登录。
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	go func() {
		time.Sleep(500 * time.Millisecond)
		restartSelf(s.opt.Logger)
	}()
}
