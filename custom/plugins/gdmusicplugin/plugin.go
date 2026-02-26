// Package gdmusicplugin 提供基于 GD Music API 的音乐搜索与发送插件。
//
// 支持指令：
//
//	/music [关键词]                搜索音乐（默认5条，第1页）
//	/music [关键词] -n [数量]      每页数量，最多10条
//	/music [关键词] -s [平台]      指定平台
//	/music [关键词] -p [页码]      指定页码
//	/music next                   下一页
//	/music prev                   上一页
//	/music get [序号]              发送当前页指定序号的音频文件
package gdmusicplugin

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/custom/plugins/gdmusicplugin/gdmusic"
	"github.com/spf13/viper"
)

// searchSession 保存某个会话的搜索状态，用于翻页
type searchSession struct {
	keyword string
	source  gdmusic.Source
	count   int // 每页条数
	page    int // 当前页码（从 1 开始）
	results []gdmusic.SearchResult
}

// MusicPlugin 音乐插件
type MusicPlugin struct {
	plugin.Meta
	client      *gdmusic.Client
	groupCache  sync.Map // map[uint]*searchSession
	friendCache sync.Map // map[uint]*searchSession
}

// NewMusicPlugin 创建音乐插件实例
func NewMusicPlugin() *MusicPlugin {
	return &MusicPlugin{
		Meta: plugin.Meta{
			Name:      "GD音乐插件",
			HelpWords: "/music [关键词] 搜索，/music get [序号] 发送，/music next|prev 翻页，/music help 帮助",
		},
	}
}

// Start 插件初始化
func (p *MusicPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	baseURL := cfg.GetString("plugin.gdmusic.base_url")
	if baseURL == "" {
		baseURL = gdmusic.DefaultBaseURL
	}
	p.client = gdmusic.New(gdmusic.WithBaseURL(baseURL))
	log.Println("GD音乐插件初始化完成，API地址:", baseURL)
	return nil
}

// -----------------------------------------------------------------------
// 群聊 / 私聊入口
// -----------------------------------------------------------------------

func (p *MusicPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !cmd.Mention || cmd.Name != "music" {
		return true, nil
	}
	if len(cmd.Args) == 0 || cmd.Args[0] == "help" {
		p.sendHelp(b, msg.GroupId, msg.Sender.UserId, msg.MessageId, true)
		return false, nil
	}
	p.dispatch(ctx, b, cmd.Args, msg.GroupId, msg.Sender.UserId, msg.MessageId, true)
	return false, nil
}

func (p *MusicPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name != "music" {
		return true, nil
	}
	if len(cmd.Args) == 0 || cmd.Args[0] == "help" {
		p.sendHelp(b, 0, msg.Sender.UserId, msg.MessageId, false)
		return false, nil
	}
	p.dispatch(ctx, b, cmd.Args, 0, msg.Sender.UserId, msg.MessageId, false)
	return false, nil
}

// dispatch 根据子命令分发
func (p *MusicPlugin) dispatch(ctx context.Context, b bot.Bot, args []string, groupId, userId, msgId uint, isGroup bool) {
	switch args[0] {
	case "get":
		p.handleGet(ctx, b, args[1:], groupId, userId, msgId, isGroup)
	case "next":
		p.handleTurn(ctx, b, +1, groupId, userId, msgId, isGroup)
	case "prev":
		p.handleTurn(ctx, b, -1, groupId, userId, msgId, isGroup)
	default:
		p.handleSearch(ctx, b, args, groupId, userId, msgId, isGroup)
	}
}

// -----------------------------------------------------------------------
// 搜索
// -----------------------------------------------------------------------

// parseSearchArgs 解析参数：关键词 [-n 数量] [-s 平台] [-p 页码]
func parseSearchArgs(args []string) (keyword string, count, page int, source gdmusic.Source) {
	count = 5
	page = 1
	source = gdmusic.SourceNetease
	var keyParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 && n <= 10 {
					count = n
				}
				i++
			}
		case "-s":
			if i+1 < len(args) {
				source = gdmusic.Source(args[i+1])
				i++
			}
		case "-p":
			if i+1 < len(args) {
				if pg, err := strconv.Atoi(args[i+1]); err == nil && pg > 0 {
					page = pg
				}
				i++
			}
		default:
			keyParts = append(keyParts, args[i])
		}
	}
	keyword = strings.Join(keyParts, " ")
	if source == "" {
		source = gdmusic.SourceNetease
	}
	return
}

func (p *MusicPlugin) handleSearch(ctx context.Context, b bot.Bot, args []string, groupId, userId, msgId uint, isGroup bool) {
	keyword, count, page, source := parseSearchArgs(args)
	if keyword == "" {
		p.reply(b, isGroup, groupId, userId, msgId, "请输入搜索关键词，例如：/music 周杰伦 晴天")
		return
	}

	sess := &searchSession{keyword: keyword, source: source, count: count, page: page}
	if p.fetchPage(ctx, b, sess, groupId, userId, msgId, isGroup) {
		p.storeSession(isGroup, groupId, userId, sess)
		p.sendResults(b, sess, groupId, userId, msgId, isGroup)
	}
}

// handleTurn 翻页：delta = +1 下一页，-1 上一页
func (p *MusicPlugin) handleTurn(ctx context.Context, b bot.Bot, delta int, groupId, userId, msgId uint, isGroup bool) {
	sess := p.loadSession(isGroup, groupId, userId)
	if sess == nil {
		p.reply(b, isGroup, groupId, userId, msgId, "请先搜索音乐，例如：/music 周杰伦 晴天")
		return
	}
	newPage := sess.page + delta
	if newPage < 1 {
		p.reply(b, isGroup, groupId, userId, msgId, "已经是第 1 页了")
		return
	}

	next := &searchSession{keyword: sess.keyword, source: sess.source, count: sess.count, page: newPage}
	if p.fetchPage(ctx, b, next, groupId, userId, msgId, isGroup) {
		p.storeSession(isGroup, groupId, userId, next)
		p.sendResults(b, next, groupId, userId, msgId, isGroup)
	}
}

// fetchPage 请求 API 并填充 sess.results，失败时自动回复错误，返回是否成功
func (p *MusicPlugin) fetchPage(ctx context.Context, b bot.Bot, sess *searchSession, groupId, userId, msgId uint, isGroup bool) bool {
	results, err := p.client.Search(ctx, sess.keyword, &gdmusic.SearchOptions{
		Source: sess.source,
		Count:  sess.count,
		Page:   sess.page,
	})
	if err != nil {
		log.Printf("[GD音乐插件] 搜索失败: %v", err)
		p.reply(b, isGroup, groupId, userId, msgId, "搜索失败，请稍后再试")
		return false
	}
	if len(results) == 0 {
		p.reply(b, isGroup, groupId, userId, msgId,
			fmt.Sprintf("「%s」（平台:%s）第 %d 页没有更多结果了", sess.keyword, sess.source, sess.page))
		return false
	}
	sess.results = results
	return true
}

// sendResults 发送当前页搜索结果列表
func (p *MusicPlugin) sendResults(b bot.Bot, sess *searchSession, groupId, userId, msgId uint, isGroup bool) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎵 「%s」第 %d 页（平台: %s，每页 %d 条）\n",
		sess.keyword, sess.page, sess.source, sess.count))
	for i, r := range sess.results {
		sb.WriteString(fmt.Sprintf("\n%d. %s - %s", i+1, r.Name, r.ArtistName()))
		if r.Album != "" {
			sb.WriteString(fmt.Sprintf("（%s）", r.Album))
		}
	}
	sb.WriteString("\n\n/music get [序号]  获取音乐文件")
	sb.WriteString("\n/music next|prev  翻页")
	p.reply(b, isGroup, groupId, userId, msgId, sb.String())
	log.Printf("[GD音乐插件] 搜索「%s」第%d页，返回 %d 条", sess.keyword, sess.page, len(sess.results))
}

// -----------------------------------------------------------------------
// 发送音乐文件
// -----------------------------------------------------------------------

func (p *MusicPlugin) handleGet(ctx context.Context, b bot.Bot, args []string, groupId, userId, msgId uint, isGroup bool) {
	if len(args) == 0 {
		p.reply(b, isGroup, groupId, userId, msgId, "请提供序号，例如：/music get 1")
		return
	}
	idx, err := strconv.Atoi(args[0])
	if err != nil || idx < 1 {
		p.reply(b, isGroup, groupId, userId, msgId, "序号格式不正确，请输入正整数")
		return
	}

	sess := p.loadSession(isGroup, groupId, userId)
	if sess == nil {
		p.reply(b, isGroup, groupId, userId, msgId, "请先搜索音乐，例如：/music 周杰伦 晴天")
		return
	}
	if idx > len(sess.results) {
		p.reply(b, isGroup, groupId, userId, msgId,
			fmt.Sprintf("序号超出范围，当前页共 %d 条结果", len(sess.results)))
		return
	}
	result := sess.results[idx-1]

	songURL, err := p.client.GetSongURL(ctx, result.URLid, &gdmusic.SongURLOptions{
		Source:  gdmusic.Source(result.Source),
		Quality: gdmusic.Quality320,
	})
	if err != nil {
		log.Printf("[GD音乐插件] 获取播放链接失败 url_id=%s source=%s: %v", result.URLid, result.Source, err)
		p.reply(b, isGroup, groupId, userId, msgId, "该歌曲暂无版权或平台不支持下载，换个平台试试（-s netease / -s kuwo）")
		return
	}
	log.Printf("[GD音乐插件] 获取链接成功: %s - %s  br=%d", result.Name, result.ArtistName(), songURL.BR)

	title := fmt.Sprintf("%s - %s", result.Name, result.ArtistName())
	p.reply(b, isGroup, groupId, userId, msgId, fmt.Sprintf("🎵 正在发送：%s", title))

	fileName := sanitizeFileName(title) + ".mp3"
	if isGroup {
		builder := msgchain.Builder().Group()
		builder.FileUrl(fileName, songURL.URL)
		b.SendGroupMsg(groupId, builder.Build())
	} else {
		builder := msgchain.Builder().Friend()
		builder.FileUrl(fileName, songURL.URL)
		b.SendFriendMsg(userId, builder.Build())
	}
	log.Printf("[GD音乐插件] 发送音乐: %s", title)
}

// -----------------------------------------------------------------------
// 会话存取
// -----------------------------------------------------------------------

func (p *MusicPlugin) storeSession(isGroup bool, groupId, userId uint, sess *searchSession) {
	if isGroup {
		p.groupCache.Store(groupId, sess)
	} else {
		p.friendCache.Store(userId, sess)
	}
}

func (p *MusicPlugin) loadSession(isGroup bool, groupId, userId uint) *searchSession {
	var v interface{}
	var ok bool
	if isGroup {
		v, ok = p.groupCache.Load(groupId)
	} else {
		v, ok = p.friendCache.Load(userId)
	}
	if !ok {
		return nil
	}
	return v.(*searchSession)
}

// -----------------------------------------------------------------------
// 工具函数
// -----------------------------------------------------------------------

func (p *MusicPlugin) reply(b bot.Bot, isGroup bool, groupId, userId, msgId uint, text string) {
	if isGroup {
		builder := msgchain.Builder().Group()
		builder.Reply(msgId).Mention(userId).Text(" " + text)
		b.SendGroupMsg(groupId, builder.Build())
	} else {
		builder := msgchain.Builder().Friend()
		builder.Text(text)
		b.SendFriendMsg(userId, builder.Build())
	}
}

func sanitizeFileName(name string) string {
	return strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "", "\"", "", "<", "", ">", "", "|", "-",
	).Replace(name)
}

func (p *MusicPlugin) sendHelp(b bot.Bot, groupId, userId, msgId uint, isGroup bool) {
	p.reply(b, isGroup, groupId, userId, msgId, helpText)
}

const helpText = `🎵 GD音乐插件

【搜索音乐】
/music [关键词]
/music [关键词] -n [数量]   每页数量，最多10条，默认5条
/music [关键词] -s [平台]   指定平台
/music [关键词] -p [页码]   直接跳转到指定页

【翻页】
/music next   下一页
/music prev   上一页

【发送音乐文件】
/music get [序号]   发送当前页指定序号的歌曲

【支持平台】
netease  网易云（默认，稳定）
kuwo     酷我（稳定）
joox     JOOX（稳定）
tencent  QQ音乐
kugou    酷狗
migu     咪咕

【示例】
/music 周杰伦 晴天
/music 晴天 -n 3 -s kuwo -p 2
/music next
/music get 1`
