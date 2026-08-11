package mipan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"pansou/config"
	"pansou/model"
	"pansou/plugin"
)

const (
	SearchURL = "https://www.mipan.so/api/search"
	TokenURL  = "https://www.mipan.so/api/token"
	Source    = "mipan"
	Priority  = 8
)

// mipan.so 于 2026-08 改版：/api/search 需要 X-Mipan-Token 头鉴权。
// 先 GET /api/token 拿到 {token, ttl}，再带 X-Mipan-Token 头搜索。
// token 按 ttl 缓存（留 300s 缓冲），命中 403 则强制刷新重试。
var (
	tokenMu     sync.Mutex
	cachedToken string
	tokenExp    int64 // unix 秒，过期时间
)

// obtainToken 获取（缓存的）token；force=true 时忽略缓存强制刷新。
func (p *MipanPlugin) obtainToken(client *http.Client, force bool) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if !force && cachedToken != "" && time.Now().Unix() < tokenExp {
		return cachedToken, nil
	}
	return fetchTokenLocked(client)
}

// fetchTokenLocked 必须在持有 tokenMu 时调用。
func fetchTokenLocked(client *http.Client) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", TokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("[%s] 创建token请求失败: %w", Source, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.mipan.so/")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[%s] 请求token失败: %w", Source, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("[%s] token接口状态码 %d: %s", Source, resp.StatusCode, string(body))
	}

	var tr struct {
		Token string `json:"token"`
		TTL   int    `json:"ttl"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("[%s] token解析失败: %w (body: %s)", Source, err, string(body))
	}
	if tr.Token == "" {
		return "", fmt.Errorf("[%s] token为空: %s", Source, string(body))
	}

	cachedToken = tr.Token
	ttl := tr.TTL
	if ttl <= 0 {
		ttl = 7200
	}
	tokenExp = time.Now().Unix() + int64(ttl) - 300 // 留 300s 缓冲，与前端一致
	fmt.Printf("[%s] 获取token成功, ttl=%d\n", Source, ttl)
	return cachedToken, nil
}

// invalidateToken 清除缓存的 token（命中 403 时调用）。
func invalidateToken() {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	cachedToken = ""
	tokenExp = 0
}

// 在init函数中注册插件
func init() {
	plugin.RegisterGlobalPlugin(NewMipanPlugin())
}

// MipanPlugin mipan 聚合搜索插件
type MipanPlugin struct {
	*plugin.BaseAsyncPlugin
}

// NewMipanPlugin 创建新的mipan异步插件
func NewMipanPlugin() *MipanPlugin {
	return &MipanPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("mipan", Priority),
	}
}

// newMipanClient 构建 HTTP client；若配置了 PROXY 环境变量则走代理。
// 原因(2026-08-11): mipan.so 按出口 IP 风控, 服务器 IP(阿里云)已被拉黑, /api/token 返回 403 HTML;
// 走 mihomo 代理换出口 IP 即可恢复。复用 config.AppConfig.ProxyURL(与 TG/TMDB 同一套代理配置)。
func newMipanClient() *http.Client {
	client := &http.Client{Timeout: 15 * time.Second}
	if config.AppConfig != nil && config.AppConfig.ProxyURL != "" {
		if proxyURL, err := url.Parse(config.AppConfig.ProxyURL); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}
	return client
}

// Search 执行搜索并返回结果（兼容性方法）
func (p *MipanPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	client := newMipanClient()
	return p.doSearch(client, keyword, ext)
}

// AsyncSearch 重写异步搜索方法，直接调用doSearch
// 不走BaseAsyncPlugin的4秒超时，因为mipan.so搜索数百条结果需要更长时间
func (p *MipanPlugin) AsyncSearch(keyword string, searchFunc func(*http.Client, string, map[string]interface{}) ([]model.SearchResult, error), mainCacheKey string, ext map[string]interface{}) ([]model.SearchResult, error) {
	client := newMipanClient()
	return p.doSearch(client, keyword, ext)
}

// doSearch 实际的搜索实现
func (p *MipanPlugin) doSearch(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	// mipan.so 的搜索接口使用 GET + query 参数；使用 POST/JSON 会返回空结果
	reqURL := fmt.Sprintf("%s?kw=%s", SearchURL, url.QueryEscape(keyword))

	// 先取 token（2026-08 改版后 /api/search 必须带 X-Mipan-Token）
	token, err := p.obtainToken(client, false)
	if err != nil {
		fmt.Printf("[%s] 获取token失败: %v\n", Source, err)
		return nil, fmt.Errorf("[%s] 获取token失败: %w", Source, err)
	}

	// 最多重试 1 次：token 过期 / 服务器 IP 变化会导致 403，强制刷新后重试
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			token, err = p.obtainToken(client, true)
			if err != nil {
				fmt.Printf("[%s] 刷新token失败: %v\n", Source, err)
				return nil, fmt.Errorf("[%s] 刷新token失败: %w", Source, err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("[%s] 创建请求失败: %w", Source, err)
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Referer", "https://www.mipan.so/")
		req.Header.Set("Origin", "https://www.mipan.so")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("X-Mipan-Token", token)

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			fmt.Printf("[%s] 请求失败: %v\n", Source, err)
			return nil, fmt.Errorf("[%s] 请求失败: %w", Source, err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if resp.StatusCode != 200 {
			if resp.StatusCode == 403 && attempt == 0 {
				fmt.Printf("[%s] 状态码 403 (token可能过期/IP变化), 刷新重试: %s\n", Source, string(respBody))
				invalidateToken()
				continue
			}
			fmt.Printf("[%s] 状态码 %d: %s\n", Source, resp.StatusCode, string(respBody))
			return nil, fmt.Errorf("[%s] 状态码 %d", Source, resp.StatusCode)
		}

		fmt.Printf("[%s] 收到响应, 大小: %d bytes, 关键词: %s\n", Source, len(respBody), keyword)
		return parseMipanResponse(respBody, keyword)
	}

	return nil, fmt.Errorf("[%s] 重试后仍返回403", Source)
}

// ============ 响应解析 ============

type MipanResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total        int                    `json:"total"`
		MergedByType map[string][]MipanLink `json:"merged_by_type"`
	} `json:"data"`
}

type MipanLink struct {
	URL      string   `json:"url"`
	Password string   `json:"password"`
	Note     string   `json:"note"`
	Datetime string   `json:"datetime"`
	Source   string   `json:"source"`
	Images   []string `json:"images"`
}

func parseMipanResponse(body []byte, keyword string) ([]model.SearchResult, error) {
	var resp MipanResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Printf("[%s] JSON解析失败: %v\n", Source, err)
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	if resp.Code != 0 {
		fmt.Printf("[%s] API返回错误: code=%d, message=%s\n", Source, resp.Code, resp.Message)
		return nil, fmt.Errorf("API错误: %s", resp.Message)
	}

	fmt.Printf("[%s] API报告total: %d, 网盘类型数: %d\n", Source, resp.Data.Total, len(resp.Data.MergedByType))

	results := make([]model.SearchResult, 0)
	seen := make(map[string]bool)
	dupCount := 0

	// 遍历 merged_by_type（按网盘分类）
	for cloudType, links := range resp.Data.MergedByType {
		for idx, link := range links {
			url := strings.TrimSpace(link.URL)
			if url == "" {
				continue
			}

			// 去重
			if seen[url] {
				dupCount++
				continue
			}
			seen[url] = true

			// 标题
			title := strings.TrimSpace(link.Note)
			if title == "" {
				title = fmt.Sprintf("%s - %s资源", keyword, cloudType)
			}

			// UniqueID用索引生成，避免URL特殊字符问题
			uniqueID := fmt.Sprintf("mipan-%s-%d", cloudType, idx)
			messageID := fmt.Sprintf("mipan-%s-%d", cloudType, idx)

			// 解析时间
			dt := parseDatetime(link.Datetime)

			results = append(results, model.SearchResult{
				MessageID: messageID,
				UniqueID:  uniqueID,
				// 不要设置 Channel 字段：后端会据此把来源拼成 "tg:mipan"，
				// 留空则按 UniqueID 正确识别为 "plugin:mipan"
				Datetime:  dt,
				Title:     title,
				Content:   fmt.Sprintf("关键词: %s | 类型: %s", keyword, cloudType),
				Links: []model.Link{
					{
						URL:      url,
						Type:     cloudType,
						Password: link.Password,
					},
				},
				Tags:   []string{cloudType},
				Images: link.Images,
			})
		}
	}

	fmt.Printf("[%s] 解析完成: %d 条结果 (去重: %d, API total: %d)\n",
		Source, len(results), dupCount, resp.Data.Total)
	return results, nil
}

// parseDatetime 解析API返回的时间字符串
func parseDatetime(dt string) time.Time {
	if dt == "" {
		return time.Now()
	}
	// 尝试RFC3339格式: 2025-12-29T09:20:00+08:00
	t, err := time.Parse(time.RFC3339, dt)
	if err != nil {
		return time.Now()
	}
	return t
}
