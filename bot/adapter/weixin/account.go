package weixin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// 账号凭据与轮询游标的本地持久化。
//
// 微信 iLink bot 的 token 由扫码登录获得（非静态密钥），且登录发生在适配器
// 运行期、无法写回配置中心，因此落盘到数据目录（./data/weixin/），与配置中心
// 互不干扰；config 中 bot.weixin.token 优先级更高（手工置入的场景）。

// accountState 落盘的账号凭据。
type accountState struct {
	Token   string `json:"token"`    // bot token（Bearer）
	BaseURL string `json:"base_url"` // 登录确认后服务端下发的 API 地址（可空 → 默认）
	BotID   string `json:"bot_id"`   // 机器人自身 ID（xxx@im.bot）
	UserID  string `json:"user_id"`  // 扫码授权者用户 ID（xxx@im.wechat）
	CDNBase string `json:"cdn_base"` // CDN 地址（可空 → 默认）
}

// stateStore 账号状态存储（单账号：一个机器人一个状态目录）。
type stateStore struct {
	mu  sync.Mutex
	dir string // 如 ./data/weixin
}

func newStateStore(dir string) *stateStore {
	return &stateStore{dir: dir}
}

// load 读取账号凭据；不存在返回 (nil, nil)。
func (s *stateStore) load() (*accountState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.accountFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var st accountState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// save 原子写入账号凭据。
func (s *stateStore) save(st *accountState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.accountFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.accountFile())
}

// clear 清除凭据（保留游标无关）。
func (s *stateStore) clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.accountFile())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *stateStore) accountFile() string { return filepath.Join(s.dir, "account.json") }

// loadUpdatesBuf 读取长轮询游标；不存在返回空串。
func (s *stateStore) loadUpdatesBuf() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(filepath.Join(s.dir, "get_updates_buf"))
	if err != nil {
		return ""
	}
	return string(b)
}

// saveUpdatesBuf 原子写入长轮询游标。
func (s *stateStore) saveUpdatesBuf(buf string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return
	}
	tmp := filepath.Join(s.dir, "get_updates_buf.tmp")
	if os.WriteFile(tmp, []byte(buf), 0o600) != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(s.dir, "get_updates_buf"))
}

// contextTokenStore 按用户缓存最近的 context_token（回复时须带回）。
// 内存为主，落盘以便重启后仍可被动回复最近会话（票据可能过期，失效时服务端
// 会拒绝发送，适配器降级为无票据重发）。
type contextTokenStore struct {
	mu     sync.Mutex
	tokens map[string]string // userRawID -> context_token
	dirty  bool
	path   string
}

func newContextTokenStore(path string) *contextTokenStore {
	s := &contextTokenStore{tokens: map[string]string{}, path: path}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &s.tokens)
	}
	return s
}

// set 记录用户最近一次下发的 context_token。
func (s *contextTokenStore) set(userID, token string) {
	if userID == "" || token == "" {
		return
	}
	s.mu.Lock()
	s.tokens[userID] = token
	s.dirty = true
	s.mu.Unlock()
}

// get 取用户的 context_token；无记录返回空串。
func (s *contextTokenStore) get(userID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokens[userID]
}

// flush 落盘（轮询循环每轮调用；无变更时为空操作）。
func (s *contextTokenStore) flush() {
	s.mu.Lock()
	dirty := s.dirty
	s.dirty = false
	tokens := make(map[string]string, len(s.tokens))
	for k, v := range s.tokens {
		tokens[k] = v
	}
	s.mu.Unlock()
	if !dirty {
		return
	}
	if b, err := json.Marshal(tokens); err == nil {
		_ = os.WriteFile(s.path, b, 0o600)
	}
}
