package weixin

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// 扫码登录（对齐 openclaw-weixin 的 CLI 登录）：
// get_bot_qrcode → 展示二维码 → 长轮询 get_qrcode_status →
// confirmed 时取回 bot_token / baseurl / ilink_bot_id / ilink_user_id。
//
// 同一套会话状态机服务两个入口：
//   - 控制台（Bot 启动时无凭据/凭据失效）：阻塞运行，配对码从 stdin 读入，进度打印到控制台
//   - Web 面板：后台运行，配对码经面板 API 提交，状态经 API 轮询
//
// 两个入口并发时以「凭据落盘」为准：任一入口成功保存后关闭适配器的
// superseded 信号，另一入口的会话立即中止，避免两个二维码互相干扰。
//
// 状态机要点：
//   - need_verifycode：手机上显示数字，要求用户输入后继续轮询
//   - verify_code_blocked / expired：配对码多次输错或二维码过期，刷新（最多 3 次）
//   - scaned_but_redirect：IDC 重定向，切换轮询主机
//   - binded_redirect：该 bot 已绑定到其他实例（本机无凭据时无法接管）

const (
	qrLoginTimeout    = 8 * time.Minute  // 整个登录流程的等待上限
	qrLongPollTimeout = 40 * time.Second // 单次状态长轮询客户端超时（服务端约挂起 35s）
	qrMaxRefresh      = 3                // 二维码最大刷新次数
	qrPollInterval    = time.Second      // 状态轮询间隔
	qrPNGSize         = 256              // 面板二维码图片边长（像素）
)

// 登录会话状态（面板展示）。
const (
	LoginStateIdle       = "idle"        // 未发起
	LoginStatePending    = "pending"     // 等待扫码
	LoginStateScaned     = "scaned"      // 已扫码，等待手机确认
	LoginStateNeedVerify = "need_verify" // 需要输入手机显示的配对数字
	LoginStateConnected  = "connected"   // 登录成功，凭据已保存
	LoginStateFailed     = "failed"      // 失败/超时/被替代（detail 说明原因）
)

// errLoginSuperseded 凭据已被其他入口更新，本会话中止（调用方应重新解析本地凭据）。
var errLoginSuperseded = errors.New("weixin: 凭据已在其他入口更新")

// loginSession 一次扫码登录会话（状态机 + 面板可见状态）。
type loginSession struct {
	// 配置（创建后不变）
	apiBase     string
	cdnBase     string
	botType     string
	localTokens []string
	// console 控制台模式：配对码从 stdin 读入、进度打印到控制台；面板模式为 false
	console bool
	// superseded 凭据被其他入口更新时关闭的信号（可为 nil）
	superseded <-chan struct{}
	// onSave 登录成功后落盘凭据（返回错误则视为登录失败）
	onSave func(*accountState) error

	mu            sync.Mutex
	state         string
	detail        string
	qrPNG         []byte // 最新二维码 PNG（面板展示）
	qrCode        string // 当前二维码票据
	qrURL         string // 二维码内容 URL
	pollBase      string
	refreshCount  int
	pendingVerify string
	pollCancel    context.CancelFunc // 面板提交配对码时打断在途轮询，立即带码重发
	finished      bool
}

func (s *loginSession) setState(state, detail string) {
	s.mu.Lock()
	s.state = state
	s.detail = detail
	s.mu.Unlock()
}

// snapshot 面板可见状态。
func (s *loginSession) snapshot() (state, detail string, qrDataURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.qrPNG) > 0 {
		qrDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(s.qrPNG)
	}
	return s.state, s.detail, qrDataURL
}

// submitVerify 提交配对码并打断在途轮询（立即带码重发）。
func (s *loginSession) submitVerify(code string) {
	s.mu.Lock()
	s.pendingVerify = strings.TrimSpace(code)
	cancel := s.pollCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel() // 打断在途长轮询，下一轮立即携带配对码
	}
}

// supersededOrDone 会话是否应中止（外部完成登录或父 ctx 结束）。
func (s *loginSession) supersededOrDone(ctx context.Context) bool {
	select {
	case <-s.superseded:
		return true
	default:
	}
	return ctx.Err() != nil
}

// prepare 获取二维码并渲染（同步调用，保证发起方立即可见二维码）。
func (s *loginSession) prepare(ctx context.Context) error {
	c := newClient("", s.apiBase, s.cdnBase)
	c.http.SetTimeout(qrLongPollTimeout)

	qr, err := c.getBotQRCode(ctx, s.botType, s.localTokens)
	if err != nil {
		s.setState(LoginStateFailed, "获取登录二维码失败: "+err.Error())
		return fmt.Errorf("获取登录二维码失败: %w", err)
	}
	png, _ := qrcode.Encode(qr.QrcodeImgContent, qrcode.Medium, qrPNGSize)
	s.mu.Lock()
	s.qrCode = qr.Qrcode
	s.qrURL = qr.QrcodeImgContent
	s.qrPNG = png
	s.pollBase = s.apiBase
	s.mu.Unlock()
	s.setState(LoginStatePending, "等待扫码")
	if s.console {
		s.consolePrint("\n========== 微信扫码登录 ==========")
		printQRTerminal(qr.QrcodeImgContent)
		s.consolePrint("若二维码无法显示或扫描，可在手机微信中打开以下链接完成授权：")
		s.consolePrint(qr.QrcodeImgContent)
		s.consolePrint("请用手机微信扫描二维码，并在手机上确认授权。")
	}
	return nil
}

// run 运行会话（阻塞）直至：登录成功（已落盘）、失败/超时、或被其他入口替代。
// 返回保存后的凭据；失败时返回原因描述；被替代时返回 errLoginSuperseded。
func (s *loginSession) run(ctx context.Context) (*accountState, error) {
	if err := s.prepare(ctx); err != nil {
		return nil, err
	}
	return s.loop(ctx)
}

// loop 扫码状态轮询循环（prepare 之后调用）。
func (s *loginSession) loop(ctx context.Context) (*accountState, error) {
	c := newClient("", s.apiBase, s.cdnBase)
	c.http.SetTimeout(qrLongPollTimeout)

	deadline := time.Now().Add(qrLoginTimeout)
	for time.Now().Before(deadline) {
		if s.supersededOrDone(ctx) {
			// 父 ctx 结束不视为 superseded（进程退出场景）
			if s.superseded != nil {
				select {
				case <-s.superseded:
					s.setState(LoginStateFailed, "登录已在其他入口完成，此二维码已失效")
					return nil, errLoginSuperseded
				default:
				}
			}
			s.setState(LoginStateFailed, "登录中止")
			return nil, ctx.Err()
		}

		pollBase := s.pollBase
		pollClient := c
		if pollBase != s.apiBase {
			pollClient = newClient("", pollBase, s.cdnBase)
			pollClient.http.SetTimeout(qrLongPollTimeout)
		}
		s.mu.Lock()
		pending := s.pendingVerify
		s.mu.Unlock()
		pollCtx, pollCancel := context.WithCancel(ctx)
		s.mu.Lock()
		s.pollCancel = pollCancel
		s.mu.Unlock()
		status, err := pollClient.getQRCodeStatus(pollCtx, s.qrCode, pending)
		pollCancel()
		if err != nil {
			if ctx.Err() != nil {
				s.setState(LoginStateFailed, "登录中止")
				return nil, ctx.Err()
			}
			// 网关超时/网络抖动/配对码提交打断在途轮询：继续等待
			time.Sleep(qrPollInterval)
			continue
		}

		switch status.Status {
		case QRStatusWait:
			s.setState(LoginStatePending, "等待扫码")
		case QRStatusScaned:
			// 配对码已被服务端接受（或无需配对码）：清除本地暂存
			s.mu.Lock()
			s.pendingVerify = ""
			s.mu.Unlock()
			s.setState(LoginStateScaned, "已扫码，请在手机上确认")
			if s.console {
				s.consolePrint("已扫码，正在验证……")
			}
		case QRStatusNeedVerifyCode:
			s.setState(LoginStateNeedVerify, "请在手机微信上查看显示的数字并输入")
			if s.console {
				// 控制台：阻塞读取配对码后立即带码重轮询
				code := readLine("请在手机微信上查看显示的数字并在此输入: ")
				s.mu.Lock()
				s.pendingVerify = code
				s.mu.Unlock()
				continue
			}
			// 面板：保持 need_verify 状态等待 API 提交；继续轮询（提交时会打断在途请求）
		case QRStatusVerifyCodeBlocked:
			if !s.refreshQR(ctx, c, "配对码多次输错") {
				return nil, fmt.Errorf("配对码多次输错，登录流程已停止")
			}
		case QRStatusExpired:
			if !s.refreshQR(ctx, c, "二维码已过期") {
				return nil, fmt.Errorf("二维码多次过期，登录流程已停止")
			}
		case QRStatusScanedButRedir:
			if host := strings.TrimSpace(status.RedirectHost); host != "" {
				s.mu.Lock()
				s.pollBase = "https://" + host
				s.mu.Unlock()
				if s.console {
					s.consolePrint("服务端重定向，切换登录主机: " + host)
				}
			}
		case QRStatusBindedRedirect:
			msg := "该微信 bot 已绑定到其他实例（本机未保存其凭据）。如需接管，请清除状态目录中的 account.json 后重新扫码"
			s.setState(LoginStateFailed, msg)
			return nil, fmt.Errorf("%s", msg)
		case QRStatusConfirmed:
			if status.IlinkBotID == "" {
				s.setState(LoginStateFailed, "登录失败：服务端未返回 ilink_bot_id")
				return nil, fmt.Errorf("登录失败：服务端未返回 ilink_bot_id")
			}
			st := &accountState{
				Token:   status.BotToken,
				BaseURL: status.Baseurl,
				BotID:   status.IlinkBotID,
				UserID:  status.IlinkUserID,
				CDNBase: s.cdnBase,
			}
			if s.onSave != nil {
				if err := s.onSave(st); err != nil {
					s.setState(LoginStateFailed, "保存登录凭据失败: "+err.Error())
					return nil, fmt.Errorf("保存登录凭据失败: %w", err)
				}
			}
			s.setState(LoginStateConnected, "登录成功（bot_id="+status.IlinkBotID+"）")
			return st, nil
		}
		time.Sleep(qrPollInterval)
	}
	s.setState(LoginStateFailed, "登录超时，请重新发起")
	return nil, fmt.Errorf("登录超时，请重试")
}

// refreshQR 刷新二维码（达到次数上限返回 false）。
func (s *loginSession) refreshQR(ctx context.Context, c *client, reason string) bool {
	s.mu.Lock()
	s.refreshCount++
	count := s.refreshCount
	s.mu.Unlock()
	if count > qrMaxRefresh {
		s.setState(LoginStateFailed, reason+"，登录流程已停止，请稍后重试")
		return false
	}
	s.setState(LoginStatePending, fmt.Sprintf("%s，正在刷新二维码（%d/%d）……", reason, count, qrMaxRefresh))
	qr, err := c.getBotQRCode(ctx, s.botType, s.localTokens)
	if err != nil {
		s.setState(LoginStateFailed, "刷新登录二维码失败: "+err.Error())
		return false
	}
	png, _ := qrcode.Encode(qr.QrcodeImgContent, qrcode.Medium, qrPNGSize)
	s.mu.Lock()
	s.qrCode = qr.Qrcode
	s.qrURL = qr.QrcodeImgContent
	s.qrPNG = png
	s.pollBase = s.apiBase
	s.mu.Unlock()
	if s.console {
		s.consolePrint(fmt.Sprintf("%s（%d/%d），二维码已更新，请重新扫描。", reason, count, qrMaxRefresh))
		printQRTerminal(qr.QrcodeImgContent)
	}
	return true
}

func (s *loginSession) consolePrint(msg string) {
	fmt.Println(msg)
}

// printQRTerminal 在终端以半块字符渲染二维码（无法渲染时静默降级为链接提示）。
func printQRTerminal(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return
	}
	fmt.Println(qr.ToSmallString(false))
}

// readLine 从 stdin 读取一行（控制台配对码输入；无法读取时返回空串）。
func readLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64), 256)
	if !scanner.Scan() {
		fmt.Println()
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}
