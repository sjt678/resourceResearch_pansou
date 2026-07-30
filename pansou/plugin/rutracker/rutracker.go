package rutracker

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
	siteURL        = "https://rutracker.org"
)

// RuTrackerPlugin RuTracker BT 索引器（L2，覆盖 软件/ISO；需登录与代理，以探针为准）
type RuTrackerPlugin struct {
	*plugin.BaseAsyncPlugin
}

func init() {
	plugin.RegisterGlobalPlugin(NewRuTrackerPlugin())
}

// NewRuTrackerPlugin 创建插件实例（优先级 3，跳过 Service 过滤，插件内自过滤）
func NewRuTrackerPlugin() *RuTrackerPlugin {
	return &RuTrackerPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPluginWithFilter("rutracker", 3, true),
	}
}

// Search 兼容性方法
func (p *RuTrackerPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	res, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return res.Results, nil
}

// SearchWithResult 返回带 IsFinal 标记的结果
func (p *RuTrackerPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实际搜索逻辑（未登录时返回登录页，通常无结果，符合降级预期）
func (p *RuTrackerPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	searchKeyword := keyword
	if ext != nil {
		if v, ok := ext["health_probe"]; ok && v == true && util.IsCJK(keyword) {
			searchKeyword = "test"
		}
	}
	searchURL := fmt.Sprintf("%s/forum/tracker.php?nm=%s&o=10&s=2&w=1", siteURL, url.QueryEscape(searchKeyword))

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
	doc.Find("table.forumline tbody tr, table tbody tr").Each(func(i int, s *goquery.Selection) {
		if s.Find("th").Length() > 0 {
			return
		}
		titleLink := s.Find("a[href*='viewtopic.php']").First()
		href, exists := titleLink.Attr("href")
		if !exists || href == "" {
			return
		}
		title := strings.TrimSpace(titleLink.Text())
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
			Content:  strings.TrimSpace(s.Find("td").Last().Text()),
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
	return plugin.FilterResultsByKeyword(results, searchKeyword), nil
}
