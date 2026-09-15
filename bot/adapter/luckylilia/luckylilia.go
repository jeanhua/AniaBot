package luckylilia

import (
	"log/slog"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/spf13/viper"
)

// NewAdapter 按配置 bot.luckylilia.mode 创建适配器：ws（默认）或 http。
// 无法识别的取值回退为 ws 并记录警告。
func NewAdapter(cfg *viper.Viper) adapter.Adapter {
	switch cfg.GetString("bot.luckylilia.mode") {
	case "http":
		slog.Info("使用 HTTP 适配器连接 Luckylilia(LLBot)")
		return NewLuckyliliaHttpAdapter()
	case "ws", "":
		slog.Info("使用 WebSocket 适配器连接 Luckylilia(LLBot)")
		return NewLuckyliliaWebSocketAdapter()
	default:
		slog.Warn("未知的适配器模式，回退为 WebSocket", "mode", cfg.GetString("bot.luckylilia.mode"))
		return NewLuckyliliaWebSocketAdapter()
	}
}

func NewLuckyliliaHttpAdapter() adapter.Adapter {
	return &luckyliliaHttpAdapter{
		// httpClient 必须在构造时就绪（与 ws 适配器预建 ackMng 同理）：
		// 首次启动等待设置向导时 Serve 尚未运行，插件仍可能调用发送接口，
		// 此时应优雅失败（请求出错）而非 nil 指针 panic。
		httpClient: resty.New(),
	}
}

func NewLuckyliliaWebSocketAdapter() adapter.Adapter {
	return &luckyliliaWebSocketAdapter{
		// ackMng 必须在构造时就绪：首次启动等待设置向导时 Serve 尚未运行，
		// 插件（如系统插件 Awake 通知）仍可能调用发送接口，此时应优雅失败而非 panic。
		ackMng: &ackManager{timeout: time.Second * 10},
	}
}
