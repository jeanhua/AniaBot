package groupnewsletter

import (
	"github.com/jeanhua/AniaBot/common/aniaerror"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms/openai"
)

// llmClient 抽象 LLM 接口，方便测试替换
type llmClient = *openai.LLM

type newsletterConfig struct {
	baseUrl  string
	apiKey   string
	model    string
	prompt   string
	maxToken int

	// msgThreshold 达到多少条消息后触发自动生成
	msgThreshold int
	// maxMessages buffer 中最多保留多少条消息
	maxMessages int

	enabledGroups []uint
}

func loadConfig(cfg *viper.Viper) newsletterConfig {
	c := newsletterConfig{
		baseUrl:      cfg.GetString("plugin.group_newsletter.model.base_url"),
		apiKey:       cfg.GetString("plugin.group_newsletter.model.api_key"),
		model:        cfg.GetString("plugin.group_newsletter.model.model"),
		prompt:       cfg.GetString("plugin.group_newsletter.model.prompt"),
		maxToken:     cfg.GetInt("plugin.group_newsletter.max_token"),
		msgThreshold: cfg.GetInt("plugin.group_newsletter.msg_threshold"),
		maxMessages:  cfg.GetInt("plugin.group_newsletter.max_messages"),
	}

	for _, id := range cfg.GetIntSlice("plugin.group_newsletter.enabled_groups") {
		if id > 0 {
			c.enabledGroups = append(c.enabledGroups, uint(id))
		}
	}

	if c.msgThreshold <= 0 {
		c.msgThreshold = 100
	}
	if c.maxMessages <= 0 {
		c.maxMessages = 500
	}
	if c.prompt == "" {
		c.prompt = defaultPrompt
	}

	return c
}

// initLLM 根据配置初始化 LLM 客户端
func (p *GroupNewsletter) initLLM() error {
	if p.config.baseUrl == "" || p.config.apiKey == "" || p.config.model == "" {
		return aniaerror.ParameterInitializeError
	}

	llm, err := openai.New(
		openai.WithBaseURL(p.config.baseUrl),
		openai.WithToken(p.config.apiKey),
		openai.WithModel(p.config.model),
	)
	if err != nil {
		return err
	}
	p.llm = llm
	return nil
}
