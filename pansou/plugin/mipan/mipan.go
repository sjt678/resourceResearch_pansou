package mipan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	body := map[string]string{"kw": keyword}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 构造请求失败: %w", Source, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", SearchURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("[%s] 创建请求失败: %w", Source, err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Referer", "https://www.mipan.so/")
	req.Header.Set("Origin", "https://www.mipan.so")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[%s] 请求失败: %w", Source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("[%s] 状态码 %d: %s", Source, resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 读取响应失败: %w", Source, err)
	}

	return parseMipanResponse(respBody, keyword)
}

// ============ 响应解析 ============

type MipanResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total        int                    `json:"total"`
		Links        []MipanLink            `json:"links"`
		MergedByType map[string][]MipanLink `json:"merged_by_type"`
	} `json:"data"`
}

type MipanLink struct {
	URL      string `json:"url"`
	Password string `json:"password"`
	Note     string `json:"note"`
}

func parseMipanResponse(body []byte, keyword string) ([]model.SearchResult, error) {
	var resp MipanResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", resp.Message)
	}

	results := make([]model.SearchResult, 0)
	seen := make(map[string]bool)

	// 优先用 merged_by_type（按网盘分类，更全）
	items := resp.Data.Links
	if len(resp.Data.MergedByType) > 0 {
		items = make([]MipanLink, 0)
		for _, links := range resp.Data.MergedByType {
			items = append(items, links...)
		}
	}

	now := time.Now()

	for idx, link := range items {
		if link.URL == "" {
			continue
		}
		if seen[link.URL] {
			continue
		}
		seen[link.URL] = true

		cloudType := detectCloudType(link.URL)
		title := strings.TrimSpace(link.Note)
		if title == "" {
			title = fmt.Sprintf("%s - %s资源", keyword, cloudType)
		}

		uniqueID := fmt.Sprintf("mipan-%s", link.URL)
		messageID := fmt.Sprintf("mipan-%d", idx)

		results = append(results, model.SearchResult{
			MessageID: messageID,
			UniqueID:  uniqueID,
			Channel:   "mipan",
			Datetime:  now,
			Title:     title,
			Content:   fmt.Sprintf("关键词: %s | 类型: %s", keyword, cloudType),
			Links: []model.Link{
				{
					URL:      link.URL,
					Type:     cloudType,
					Password: link.Password,
				},
			},
			Tags: []string{cloudType},
		})
	}

	fmt.Printf("[%s] 解析到 %d 条结果 (API报告total: %d)\n", Source, len(results), resp.Data.Total)
	return results, nil
}

func detectCloudType(rawURL string) string {
	lower := strings.ToLower(rawURL)

	switch {
	case strings.Contains(lower, "pan.baidu.com"):
		return "baidu"
	case strings.Contains(lower, "alipan.com") || strings.Contains(lower, "aliyundrive.com"):
		return "aliyun"
	case strings.Contains(lower, "pan.quark.cn"):
		return "quark"
	case strings.Contains(lower, "115.com") || strings.Contains(lower, "115cdn.com") || strings.Contains(lower, "anxia.com"):
		return "115"
	case strings.Contains(lower, "cloud.189.cn"):
		return "tianyi"
	case strings.Contains(lower, "drive.uc.cn"):
		return "uc"
	case strings.Contains(lower, "yun.139.com"):
		return "mobile"
	case strings.Contains(lower, "123912.com") || strings.Contains(lower, "123pan.com") || strings.Contains(lower, "123684.com") || strings.Contains(lower, "123865.com"):
		return "123"
	case strings.HasPrefix(lower, "magnet:"):
		return "magnet"
	default:
		return "other"
	}
}
