package core

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

// TestMsgEventTimeout 消息处理超时的配置读取：nil/未设/非法配置兜底
// MsgEventTimeout，正常配置生效，超大配置先限幅再乘 time.Second 防溢出。
func TestMsgEventTimeout(t *testing.T) {
	t.Run("nil配置兜底默认", func(t *testing.T) {
		ania := &AniaBot{}
		if got := ania.msgEventTimeout(); got != MsgEventTimeout {
			t.Fatalf("got %v, want %v", got, MsgEventTimeout)
		}
	})
	t.Run("未设置兜底默认", func(t *testing.T) {
		ania := &AniaBot{cfg: viper.New()}
		if got := ania.msgEventTimeout(); got != MsgEventTimeout {
			t.Fatalf("got %v, want %v", got, MsgEventTimeout)
		}
	})
	t.Run("配置生效", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("bot.msg_event_timeout_sec", 600)
		ania := &AniaBot{cfg: cfg}
		if got := ania.msgEventTimeout(); got != 10*time.Minute {
			t.Fatalf("got %v, want %v", got, 10*time.Minute)
		}
	})
	t.Run("负数兜底默认", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("bot.msg_event_timeout_sec", -5)
		ania := &AniaBot{cfg: cfg}
		if got := ania.msgEventTimeout(); got != MsgEventTimeout {
			t.Fatalf("got %v, want %v", got, MsgEventTimeout)
		}
	})
	t.Run("超大配置限幅防溢出", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("bot.msg_event_timeout_sec", 10000000000) // 直接乘会 int64 溢出为负值
		ania := &AniaBot{cfg: cfg}
		if got := ania.msgEventTimeout(); got != msgEventMaxTimeoutSec*time.Second {
			t.Fatalf("超大配置应被限幅到 %v, got %v", msgEventMaxTimeoutSec*time.Second, got)
		}
	})
}
