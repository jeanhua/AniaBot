package adminpanel

import (
	"encoding/json"
	"net/http"
)

// QRLoginSource 可选接口：平台适配器实现后，面板可发起扫码登录
// （当前由微信适配器实现——iLink bot 无静态 Token，凭据须扫码获取）。
// 适配器负责会话状态机与凭据落盘；面板仅负责展示二维码、轮询状态、提交配对码。
type QRLoginSource interface {
	// QRLoginStart 发起（或复用进行中的）扫码登录，返回二维码 PNG 的 data URL
	QRLoginStart() (qrDataURL string, err error)
	// QRLoginStatus 返回当前登录流程状态（idle/pending/scaned/need_verify/connected/failed）
	// 与提示详情；进行中的会话同时返回最新二维码（可为空串）
	QRLoginStatus() (state, detail, qrDataURL string)
	// QRLoginSubmitVerify 提交手机上显示的配对数字（need_verify 状态时调用）
	QRLoginSubmitVerify(code string) error
}

// QRLoginChannel 支持扫码登录的平台（core 收集适配器可选能力时填充）。
type QRLoginChannel struct {
	Name     string // 适配器名
	Platform string // 平台标识
	Source   QRLoginSource
}

func (s *Server) qrLoginOf(platform string) (QRLoginSource, bool) {
	for _, ch := range s.opt.QRLogins {
		if ch.Platform == platform {
			return ch.Source, true
		}
	}
	return nil, false
}

// handleQRLoginSources GET /api/qrlogin/sources：支持扫码登录的平台列表。
func (s *Server) handleQRLoginSources(w http.ResponseWriter, r *http.Request) {
	out := make([]map[string]string, 0, len(s.opt.QRLogins))
	for _, ch := range s.opt.QRLogins {
		out = append(out, map[string]string{"name": ch.Name, "platform": ch.Platform})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

// handleQRLoginStart POST /api/qrlogin/{platform}/start：发起扫码登录。
func (s *Server) handleQRLoginStart(w http.ResponseWriter, r *http.Request) {
	src, ok := s.qrLoginOf(r.PathValue("platform"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "该平台不支持扫码登录"})
		return
	}
	qr, err := src.QRLoginStart()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"qr_data_url": qr})
}

// handleQRLoginStatus GET /api/qrlogin/{platform}/status：轮询登录状态。
func (s *Server) handleQRLoginStatus(w http.ResponseWriter, r *http.Request) {
	src, ok := s.qrLoginOf(r.PathValue("platform"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "该平台不支持扫码登录"})
		return
	}
	state, detail, qr := src.QRLoginStatus()
	writeJSON(w, http.StatusOK, map[string]string{"state": state, "detail": detail, "qr_data_url": qr})
}

// handleQRLoginVerify POST /api/qrlogin/{platform}/verify：提交配对码。
func (s *Server) handleQRLoginVerify(w http.ResponseWriter, r *http.Request) {
	src, ok := s.qrLoginOf(r.PathValue("platform"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "该平台不支持扫码登录"})
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少配对码"})
		return
	}
	if err := src.QRLoginSubmitVerify(body.Code); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
