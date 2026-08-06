package adminpanel

import (
	"net/http"

	"github.com/jeanhua/AniaBot/bot/component/oplog"
)

// ---- config preset handlers（配置预设：保存当前配置为快照，一键切换） ----

// handlePresetList 返回全部配置预设的概要列表。
func (s *Server) handlePresetList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.opt.Config.PresetList())
}

// handlePresetSave 将当前全部配置保存为预设（同名覆盖，保留创建时间）。
func (s *Server) handlePresetSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.opt.Config.SavePreset(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.opt.Logger.Info("配置预设已通过 Web 面板保存", "name", req.Name)
	oplog.Record(oplog.CategoryConfig, "preset_save", "面板保存配置预设: "+req.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handlePresetApply 应用预设：将快照中的配置键写回配置中心（仅覆盖快照包含的键），
// 重启 Bot 后生效。
func (s *Server) handlePresetApply(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	n, err := s.opt.Config.ApplyPreset(name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.opt.Logger.Info("配置预设已通过 Web 面板应用", "name", name, "keys", n)
	oplog.Record(oplog.CategoryConfig, "preset_apply", "面板应用配置预设: "+name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "keys": n, "need_restart": true})
}

// handlePresetDelete 删除预设（只删除快照，不影响当前配置）。
func (s *Server) handlePresetDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !s.opt.Config.DeletePreset(name) {
		writeError(w, http.StatusNotFound, "预设不存在")
		return
	}
	s.opt.Logger.Info("配置预设已通过 Web 面板删除", "name", name)
	oplog.Record(oplog.CategoryConfig, "preset_delete", "面板删除配置预设: "+name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
