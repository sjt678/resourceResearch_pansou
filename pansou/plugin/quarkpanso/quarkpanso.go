package quarkpanso

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"pansou/model"
	"pansou/plugin"
)

const (
	SearchURL = "https://www.quarkpanso.com/v1/search/disk"
	Source    = "quarkpanso"
	Priority  = 8
)

// 该值是浏览器端 fingerprintjs 产生的指纹；服务器可能会基于它做反爬校验。
// 如果后续请求被拒或返回 0 条，可以从浏览器 DevTools 抓最新的值替换。
const fpData = "9a439b63ed091314219c45705eaaa99e"

// 在init函数中注册插件
func init() {
	plugin.RegisterGlobalPlugin(NewQuarkpansoPlugin())
}

// QuarkpansoPlugin 夸克盘搜插件
type QuarkpansoPlugin struct {
	*plugin.BaseAsyncPlugin
}

// NewQuarkpansoPlugin 创建新的夸克盘搜异步插件
func NewQuarkpansoPlugin() *QuarkpansoPlugin {
	return &QuarkpansoPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("quarkpanso", Priority),
	}
}

// Search 执行搜索并返回结果（兼容性方法）
func (p *QuarkpansoPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return p.doSearch(client, keyword)
}

// AsyncSearch 重写异步搜索方法，直接调用doSearch
// 走自己的 15s 客户端超时，不受 BaseAsyncPlugin 的 4s 全局超时限制
func (p *QuarkpansoPlugin) AsyncSearch(keyword string, searchFunc func(*http.Client, string, map[string]interface{}) ([]model.SearchResult, error), mainCacheKey string, ext map[string]interface{}) ([]model.SearchResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return p.doSearch(client, keyword)
}

// doSearch 实际的搜索实现
// 单页请求：size=30, page=1，足够大多数场景；如需更多可改为循环分页。
func (p *QuarkpansoPlugin) doSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	payload := map[string]interface{}{
		"page":          1,
		"q":             keyword,
		"user":          "",
		"exact":         false,
		"format":        []string{},
		"share_time":    "",
		"size":          30,
		"order":         "",
		"type":          "",
		"search_ticket": "",
		"exclude_user":  []string{},
		"adv_params": map[string]interface{}{
			"wechat_pwd": "",
			"search_code": "",
			"platform":   "pc",
			"fp_data":    fpData,
			"automated":  "0",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("[%s] 序列化请求体失败: %w", Source, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", SearchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("[%s] 创建请求失败: %w", Source, err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Referer", "https://www.quarkpanso.com/")
	req.Header.Set("Origin", "https://www.quarkpanso.com")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[%s] 请求失败: %w", Source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("[%s] 状态码 %d: %s", Source, resp.StatusCode, truncate(string(respBody), 200))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 读取响应失败: %w", Source, err)
	}

	return parseQuarkpansoResponse(respBody, keyword)
}

// ============ 响应解析 ============

// 顶层响应：{"code":200,"msg":"请求成功","data":{"total":150,"per_size":15,"took":38,"list":[...]}}
type QuarkpansoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Total int                  `json:"total"`
		List  []QuarkpansoDiskItem `json:"list"`
	} `json:"data"`
}

// 列表项：一条分享的网盘资源
type QuarkpansoDiskItem struct {
	DiskID     string   `json:"disk_id"`
	DiskName   string   `json:"disk_name"`
	DiskPass   string   `json:"disk_pass"`
	DiskType   string   `json:"disk_type"`
	Files      string   `json:"files"`
	ShareUser  string   `json:"share_user"`
	SharedTime string   `json:"shared_time"`
	Link       string   `json:"link"`
	Tags       []string `json:"tags"`
}

// 去除 disk_name / files 中的 <em>...</em> 关键词高亮标签
var emTagRegex = regexp.MustCompile(`</?em>`)

func stripEmTags(s string) string {
	return emTagRegex.ReplaceAllString(s, "")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func parseQuarkpansoResponse(body []byte, keyword string) ([]model.SearchResult, error) {
	var resp QuarkpansoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("[%s] JSON解析失败: %w", Source, err)
	}

	// 注意：夸克盘搜的成功码是 200（mipan 是 0），这里单独处理
	if resp.Code != 200 {
		return nil, fmt.Errorf("[%s] API错误: code=%d msg=%s", Source, resp.Code, resp.Msg)
	}

	results := make([]model.SearchResult, 0, len(resp.Data.List))
	seen := make(map[string]bool)

	for idx, item := range resp.Data.List {
		url := strings.TrimSpace(item.Link)
		if url == "" {
			continue
		}
		// 用 link 去重（同一资源不会被重复入结果集）
		if seen[url] {
			continue
		}
		seen[url] = true

		// 标题：去掉 <em> 高亮标签
		title := strings.TrimSpace(stripEmTags(item.DiskName))
		if title == "" {
			title = fmt.Sprintf("%s - %s资源", keyword, item.DiskType)
		}

		// 网盘类型标准化成小写（QUARK → quark），方便与 mipan 等源输出对齐
		cloudType := strings.ToLower(strings.TrimSpace(item.DiskType))
		if cloudType == "" {
			cloudType = "others"
		}

		// 解析分享时间
		dt := parseSharedTime(item.SharedTime)

		// Content：拼上关键词、类型、分享人、文件清单（截断避免过长）
		contentParts := []string{
			fmt.Sprintf("关键词: %s", keyword),
			fmt.Sprintf("类型: %s", cloudType),
		}
		if item.ShareUser != "" {
			contentParts = append(contentParts, fmt.Sprintf("分享人: %s", item.ShareUser))
		}
		if item.Files != "" {
			contentParts = append(contentParts, fmt.Sprintf("文件: %s", truncate(stripEmTags(item.Files), 200)))
		}

		// UniqueID 用 cloudType + idx（同一关键词下稳定且唯一）
		uniqueID := fmt.Sprintf("quarkpanso-%s-%d", cloudType, idx)
		messageID := uniqueID

		results = append(results, model.SearchResult{
			MessageID: messageID,
			UniqueID:  uniqueID,
			// 不设 Channel 字段：后端会据此把来源拼成 "plugin:quarkpanso"
			Datetime: dt,
			Title:    title,
			Content:  strings.Join(contentParts, " | "),
			Links: []model.Link{
				{
					URL:      url,
					Type:     cloudType,
					Password: item.DiskPass,
				},
			},
			Tags: item.Tags,
		})
	}

	return results, nil
}

// parseSharedTime 解析分享时间字符串，支持 "2006-01-02 15:04:05" 与 RFC3339
func parseSharedTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}
