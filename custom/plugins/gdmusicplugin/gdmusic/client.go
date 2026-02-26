package gdmusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://music-api.gdstudio.xyz/api.php"
	DefaultTimeout = 10 * time.Second
)

// Source 音乐平台
type Source string

const (
	SourceNetease Source = "netease" // 网易云（稳定）
	SourceKuwo    Source = "kuwo"    // 酷我（稳定）
	SourceJoox    Source = "joox"    // JOOX（稳定）
	SourceTencent Source = "tencent" // QQ音乐
	SourceKugou   Source = "kugou"   // 酷狗
	SourceMigu    Source = "migu"    // 咪咕
)

// Quality 音质
type Quality string

const (
	Quality128      Quality = "128"
	Quality320      Quality = "320"
	QualityLossless Quality = "999"
)

// Client GD Music API 客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

func New(opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) get(ctx context.Context, params url.Values, v interface{}) error {
	reqURL := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("gdmusic: build request: %w", err)
	}
	req.Header.Set("User-Agent", "gdmusic-go-sdk/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gdmusic: http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("gdmusic: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gdmusic: server returned %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("gdmusic: decode response: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------
// 数据模型
// -----------------------------------------------------------------------

// SearchResult 单条搜索结果
type SearchResult struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Artist  []string `json:"artist"` // API 返回数组
	Album   string   `json:"album"`
	PicID   string   `json:"pic_id"`
	URLid   string   `json:"url_id"`
	LyricID string   `json:"lyric_id"`
	Source  string   `json:"source"`
	From    string   `json:"from"`
}

// ArtistName 返回拼接后的歌手名，自动去除末尾 "-"
func (r SearchResult) ArtistName() string {
	parts := make([]string, 0, len(r.Artist))
	for _, a := range r.Artist {
		a = strings.TrimRight(strings.TrimSpace(a), "-")
		if a != "" {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " / ")
}

// SongURL 播放链接信息
type SongURL struct {
	URL    string `json:"url"`
	BR     int    `json:"br"`
	Size   int    `json:"size"`
	Source string `json:"source"`
}

// AlbumPic 专辑封面
type AlbumPic struct {
	URL string `json:"url"`
}

// -----------------------------------------------------------------------
// 搜索
// -----------------------------------------------------------------------

type SearchOptions struct {
	Source Source
	Count  int
	Page   int
}

func (c *Client) Search(ctx context.Context, keyword string, opts *SearchOptions) ([]SearchResult, error) {
	if keyword == "" {
		return nil, fmt.Errorf("gdmusic: keyword must not be empty")
	}

	count := 10
	page := 1
	var source Source
	if opts != nil {
		if opts.Count > 0 {
			count = opts.Count
		}
		if opts.Page > 1 {
			page = opts.Page
		}
		source = opts.Source
	}

	params := url.Values{}
	params.Set("types", "search")
	params.Set("name", keyword)
	params.Set("count", strconv.Itoa(count))
	params.Set("pages", strconv.Itoa(page))
	if source != "" {
		params.Set("source", string(source))
	}

	var results []SearchResult
	if err := c.get(ctx, params, &results); err != nil {
		return nil, err
	}
	// 部分平台忽略 pages 参数会返回第1页，客户端按 count 截断即可
	if len(results) > count {
		results = results[:count]
	}
	return results, nil
}

// -----------------------------------------------------------------------
// 获取播放链接
// -----------------------------------------------------------------------

type SongURLOptions struct {
	Source  Source
	Quality Quality
}

func (c *Client) GetSongURL(ctx context.Context, trackID string, opts *SongURLOptions) (*SongURL, error) {
	if trackID == "" {
		return nil, fmt.Errorf("gdmusic: trackID must not be empty")
	}
	params := url.Values{}
	params.Set("types", "url")
	params.Set("id", trackID)
	if opts != nil {
		if opts.Source != "" {
			params.Set("source", string(opts.Source))
		}
		if opts.Quality != "" {
			params.Set("br", string(opts.Quality))
		}
	}
	// 先拿原始 JSON，便于 URL 为空时打印诊断信息
	var raw json.RawMessage
	if err := c.get(ctx, params, &raw); err != nil {
		return nil, err
	}
	var result SongURL
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("gdmusic: decode song url: %w (raw: %s)", err, raw)
	}
	if result.URL == "" {
		return &result, fmt.Errorf("gdmusic: empty url (raw: %s)", raw)
	}
	return &result, nil
}

// -----------------------------------------------------------------------
// 获取专辑封面
// -----------------------------------------------------------------------

type PicOptions struct {
	Source Source
	Size   string // "300" or "500"
}

func (c *Client) GetPic(ctx context.Context, picID string, opts *PicOptions) (*AlbumPic, error) {
	if picID == "" {
		return nil, fmt.Errorf("gdmusic: picID must not be empty")
	}
	params := url.Values{}
	params.Set("types", "pic")
	params.Set("id", picID)
	if opts != nil {
		if opts.Source != "" {
			params.Set("source", string(opts.Source))
		}
		if opts.Size != "" {
			params.Set("size", opts.Size)
		}
	}
	var result AlbumPic
	if err := c.get(ctx, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
