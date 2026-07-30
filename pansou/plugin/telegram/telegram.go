package telegram

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"pansou/config"
	"pansou/model"
	"pansou/plugin"
	"pansou/util"

	"gopkg.in/yaml.v3"
)

// TelegramChannelConfig 单个频道配置
type TelegramChannelConfig struct {
	Channel    string            `yaml:"channel"`    // 频道用户名（可带或不带 @）
	CloudType  string            `yaml:"cloud_type"` // 该频道主要分享的网盘类型，用于标签/过滤
	KeywordMap map[string]string `yaml:"keyword_map"` // 可选：类别关键词映射（仅记录，不参与硬过滤）
	Enabled    bool              `yaml:"enabled"`
}

// telegramConfig 配置文件整体结构
type telegramConfig struct {
	API struct {
		SessionPath string `yaml:"session_path"`
	} `yaml:"api"`
	Channels []TelegramChannelConfig `yaml:"channels"`
}

// TelegramChannelPlugin 通用 Telegram 频道适配器（内嵌 BaseAsyncPlugin）
//
// 当前为「降级模式」实现：不引入 github.com/gotd/td，client 以 interface{} 占位。
// 当 TG 凭据（TG_API_ID / TG_API_HASH / TG_SESSION_PATH）缺失时，client 保持 nil，
// 搜索/探针会返回明确错误并被标记为 fail，不影响其余源与启动。
// 接入真实 MTProto 客户端（gotd/td）时：在 Initialize 内创建 *tg.Client 并赋值 p.client，
// 并在 searchImpl 的 client != nil 分支实现频道消息读取与解析。
type TelegramChannelPlugin struct {
	*plugin.BaseAsyncPlugin
	channels   []TelegramChannelConfig
	mu         sync.RWMutex
	client     interface{} // 接入 gotd/td 后为 *tg.Client；降级模式为 nil
	configured bool
	initOnce   sync.Once
}

// init 仅注册一次
func init() {
	plugin.RegisterGlobalPlugin(NewTelegramPlugin())
}

// NewTelegramPlugin 创建插件实例（优先级 3）
func NewTelegramPlugin() *TelegramChannelPlugin {
	return &TelegramChannelPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("telegram", 3),
	}
}

// Name 返回插件名称（与包/目录 id 一致）
func (p *TelegramChannelPlugin) Name() string {
	return "telegram"
}

// ensureInitialized 惰性初始化（保证 config.Init 之后才读取配置）
func (p *TelegramChannelPlugin) ensureInitialized() {
	p.initOnce.Do(func() {
		_ = p.Initialize()
	})
}

// Initialize 加载频道配置与 TG 凭据；任何缺失都仅标记未配置，绝不 panic/阻断启动
func (p *TelegramChannelPlugin) Initialize() error {
	cfgPath := config.AppConfig.TGConfigPath
	if cfgPath == "" {
		cfgPath = "plugin/telegram/config.yaml"
	}

	data, err := os.ReadFile(cfgPath)
	if err == nil {
		var cfg telegramConfig
		if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr == nil {
			for i := range cfg.Channels {
				cfg.Channels[i].Channel = normalizeChannel(cfg.Channels[i].Channel)
			}
			p.mu.Lock()
			p.channels = cfg.Channels
			p.mu.Unlock()
			if cfg.API.SessionPath != "" && config.AppConfig.TGSessionPath == "" {
				// 仅当环境变量未设置时，用配置文件里的 session_path 兜底
				_ = cfg.API.SessionPath
			}
		}
	}

	apiID := config.AppConfig.TGAPIID
	apiHash := config.AppConfig.TGAPIHash
	sessionPath := config.AppConfig.TGSessionPath

	// 降级模式：不引入 gotd/td，凭据缺失即保持未配置
	if apiID == "" || apiHash == "" || sessionPath == "" {
		p.configured = false
		p.client = nil
		return nil
	}

	// TODO(gotd/td): 在此用 apiID/apiHash/sessionPath 创建 *tg.Client 并赋值 p.client
	// 例如：
	//   dispatcher := tg.NewUpdateDispatcher()
	//   client := tg.NewClient(tg.ClientConfig{...})
	//   p.client = client
	// 当前降级模式下保持 nil，交由探针标记为 fail。
	p.configured = false
	p.client = nil
	return nil
}

// Search 兼容性方法
func (p *TelegramChannelPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 返回带 IsFinal 标记的结果
func (p *TelegramChannelPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实际搜索逻辑
func (p *TelegramChannelPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	p.ensureInitialized()

	if p.client == nil {
		return nil, fmt.Errorf("[%s] 未配置凭据：需 TG_API_ID/TG_API_HASH/TG_SESSION_PATH，参见 plugin/telegram/README.md", p.Name())
	}

	// 以下为真实客户端接入后的处理逻辑骨架（降级模式下不会执行）：
	// 对每个 Enabled 频道并发搜索关键词，解析消息文本中的网盘链接，
	// 构造 model.SearchResult{UniqueID:"telegram-<channel>-<msgID>", Title:..., Channel:"@"+channel, work_title:..., Links:[]model.Link{...}}。
	p.mu.RLock()
	channels := make([]TelegramChannelConfig, len(p.channels))
	copy(channels, p.channels)
	p.mu.RUnlock()

	var results []model.SearchResult
	// sem := make(chan struct{}, 4)  // 真实实现中用于并发限流
	_ = channels
	// TODO(gotd/td): 真实实现时在此并发读取频道消息并解析。
	_ = util.DefaultUserAgent
	return results, nil
}

// normalizeChannel 规范化频道名（去掉前导 @，保留小写）
func normalizeChannel(ch string) string {
	ch = strings.TrimSpace(ch)
	ch = strings.TrimPrefix(ch, "@")
	return ch
}

// 网盘链接识别正则（baidu/aliyun/quark/uc/tianyi/115/123/magnet 等）
var (
	urlRegex   = regexp.MustCompile(`(?:https?://[^\s"'<>]+|magnet:\?[^\s"'<>]+)`)
	baiduRe    = regexp.MustCompile(`pan\.baidu\.com`)
	aliyunRe   = regexp.MustCompile(`(?i)aliyundrive|alipan\.com`)
	quarkRe    = regexp.MustCompile(`(?i)quark\.cn|quark\.aliyundrive`)
	ucRe       = regexp.MustCompile(`(?i)uc\.douyin\.com|weiyun\.com`)
	tianyiRe   = regexp.MustCompile(`(?i)cloud\.189\.cn|tianyi`)
	c115Re     = regexp.MustCompile(`(?i)115\.com|115cdn`)
	c123Re     = regexp.MustCompile(`(?i)123pan\.com|123\.com`)
	magnetRe   = regexp.MustCompile(`^magnet:`)
)

// classifyLinkType 根据 URL 推断网盘类型
func classifyLinkType(rawURL string) string {
	switch {
	case magnetRe.MatchString(rawURL):
		return "magnet"
	case baiduRe.MatchString(rawURL):
		return "baidu"
	case aliyunRe.MatchString(rawURL):
		return "aliyun"
	case quarkRe.MatchString(rawURL):
		return "quark"
	case ucRe.MatchString(rawURL):
		return "uc"
	case tianyiRe.MatchString(rawURL):
		return "tianyi"
	case c115Re.MatchString(rawURL):
		return "115"
	case c123Re.MatchString(rawURL):
		return "123"
	default:
		return "others"
	}
}

// extractPassword 从 URL 或邻近文本中提取提取码/密码
func extractPassword(rawURL, text string) string {
	if u, err := url.Parse(rawURL); err == nil {
		if pwd := u.Query().Get("pwd"); pwd != "" {
			return pwd
		}
	}
	// 形如 "提取码: xxxx" / "密码: xxxx"
	re := regexp.MustCompile(`(?:提取码|密码|访问码)[\s:：]*([A-Za-z0-9]{4,8})`)
	if m := re.FindStringSubmatch(text); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// extractLinksFromText 从消息文本中解析网盘链接，返回标准化 Link 列表
func extractLinksFromText(text string) []model.Link {
	matches := urlRegex.FindAllString(text, -1)
	links := make([]model.Link, 0, len(matches))
	seen := make(map[string]bool)
	for _, raw := range matches {
		if seen[raw] {
			continue
		}
		seen[raw] = true
		links = append(links, model.Link{
			Type:     classifyLinkType(raw),
			URL:      raw,
			Password: extractPassword(raw, text),
		})
	}
	return links
}

// 确保 time 被使用（Initialize 未来接入 gotd 时会用到）；此处显式引用避免未使用导入告警
var _ = time.Now
