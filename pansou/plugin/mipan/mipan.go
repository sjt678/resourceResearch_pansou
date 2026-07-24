package mipan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"pansou/model"
	"pansou/plugin"
)

const (
	SearchURL = "https://www.mipan.so/api/search"
	Source    = "mipan"
	Priority  = 8
)

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

// Search 执行搜索并返回结果（兼容性方法）
func (p *MipanPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return p.doSearch(client, keyword, ext)
}

// AsyncSearch 重写异步搜索方法，直接调用doSearch
// 不走BaseAsyncPlugin的4秒超时，因为mipan.so搜索数百条结果需要更长时间
func (p *MipanPlugin) AsyncSearch(keyword string, searchFunc func(*http.Client, string, map[string]interface{}) ([]model.SearchResult, error), mainCacheKey string, ext map[string]interface{}) ([]model.SearchResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return p.doSearch(client, keyword, ext)
}

// doSearch 实际的搜索实现
func (p *MipanPlugin) doSearch(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	// mipan.so 的搜索接口使用 GET + query 参数；使用 POST/JSON 会返回空结果
	reqURL := fmt.Sprintf("%s?kw=%s", SearchURL, url.QueryEscape(keyword))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s] 创建请求失败: %w", Source, err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://www.mipan.so/")
	req.Header.Set("Origin", "https://www.mipan.so")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[%s] 请求失败: %v\n", Source, err)
		return nil, fmt.Errorf("[%s] 请求失败: %w", Source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("[%s] 状态码 %d: %s\n", Source, resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("[%s] 状态码 %d", Source, resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 读取响应失败: %w", Source, err)
	}

	fmt.Printf("[%s] 收到响应, 大小: %d bytes, 关键词: %s\n", Source, len(respBody), keyword)

	return parseMipanResponse(respBody, keyword)
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
				Channel:   "mipan",
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
