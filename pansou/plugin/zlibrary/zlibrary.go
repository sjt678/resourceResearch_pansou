package zlibrary

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"pansou/model"
	"pansou/plugin"
	"pansou/util"
)

const (
	defaultTimeout = 12 * time.Second
	siteURL        = "https://zh.b-ok.cc"
)

// ZLibraryPlugin Z-Library 电子书搜索插件（L2，需代理/登录，可靠性较低，以探针为准）
type ZLibraryPlugin struct {
	*plugin.BaseAsyncPlugin
}

func init() {
	plugin.RegisterGlobalPlugin(NewZLibraryPlugin())
}

// NewZLibraryPlugin 创建插件实例（优先级 3，走 Service 过滤）
func NewZLibraryPlugin() *ZLibraryPlugin {
	return &ZLibraryPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("zlibrary", 3),
	}
}

// Search 兼容性方法
func (p *ZLibraryPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	res, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return res.Results, nil
}

// SearchWithResult 返回带 IsFinal 标记的结果
func (p *ZLibraryPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实际搜索逻辑
func (p *ZLibraryPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	searchKeyword := keyword
	if ext != nil {
		if v, ok := ext["health_probe"]; ok && v == true && util.IsCJK(keyword) {
			searchKeyword = "test"
		}
	}
	searchURL := fmt.Sprintf("%s/s/%s", siteURL, url.QueryEscape(searchKeyword))

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s] 创建请求失败: %w", p.Name(), err)
	}
	util.SetCommonHeaders(req)

	resp, err := util.FetchWithRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("[%s] 搜索请求失败: %w", p.Name(), err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 解析页面失败: %w", p.Name(), err)
	}

	var results []model.SearchResult
	doc.Find(".book-item, table tr, .li").Each(func(i int, s *goquery.Selection) {
		link := s.Find("a").First()
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}
		title := strings.TrimSpace(link.Text())
		if title == "" {
			return
		}
		fullURL := href
		if strings.HasPrefix(href, "/") {
			fullURL = siteURL + href
		}
		results = append(results, model.SearchResult{
			UniqueID: fmt.Sprintf("%s-%s", p.Name(), util.ShortHash(href)),
			Title:    title,
			Content:  strings.TrimSpace(s.Find("div, p").First().Text()),
			Datetime: time.Now(),
			Channel:  "",
			Links: []model.Link{
				{Type: "others", URL: fullURL, Password: ""},
			},
		})
	})

	if len(results) == 0 {
		return []model.SearchResult{}, nil
	}
	return results, nil
}
