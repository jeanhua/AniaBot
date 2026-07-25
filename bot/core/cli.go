package core

import (
	"context"
	"errors"

	"github.com/jeanhua/AniaBot/bot/adminpanel"
)

// ResetPanelPassword 通过命令行重置 Web 控制面板密码（忘记密码时使用）。
// 打开与正常运行一致的持久化存储（由环境变量 ANIABOT_STORE_DRIVER /
// ANIABOT_SQLITE_PATH / ANIABOT_MYSQL_DSN 引导），覆盖 __admin 命名空间中的密码哈希。
func ResetPanelPassword(password string) error {
	if password == "" {
		return errors.New("新密码不能为空")
	}
	store, err := newPersistentStorage(context.Background(), Logger().WithGroup("Persistent"))
	if err != nil {
		return err
	}
	if !adminpanel.ResetPassword(store, password) {
		return errors.New("写入密码失败，请检查持久化存储")
	}
	return nil
}
