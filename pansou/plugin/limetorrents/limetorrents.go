package limetorrents

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"pansou/model"
	"pansou/plugin"
	"pansou/util"
)

const (
	defaultTimeout = 12 * time.Second
	siteURL        = "https://www.limetorrents.lol"
)

// LimeTorrentsPlugin LimeTorrents 国际 BT 索引器（L2，覆盖 综合）
type LimeTorrentsPlugin struct {
	*plugin.BaseAsyncPlugin
}

func init() {
	plugin.RegisterGlobalPlugin(NewLimeTorrentsPlugin())
}

// NewLimeTorrentsPlugin 创建插件实例（优先级 3，跳过 Service 过滤，插件内自过滤）
func NewLimeTorrentsPlugin() *LimeTorrentsPlugin {
	return &LimeTorrentsPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPluginWithFilter("limetorrents", 3, true),
	}
}

// Search 兼容性方法
func (p *LimeTorrentsPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	res, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return res.Results, nil
}

// SearchWithResult 返回带 IsFinal 标记的结果
func (p *LimeTorrentsPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

type listItem struct {
	title string
	href  string
}

// searchImpl 实际搜索逻辑
func (p *LimeTorrentsPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	searchKeyword := keyword
	if ext != nil {
		if v, ok := ext["health_probe"]; ok && v == true && util.IsCJK(keyword) {
			searchKeyword = "test"
		}
	}
	searchURL := fmt.Sprintf("%s/search/all/%s/seeds/1/", siteURL, url.PathEscape(searchKeyword))

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

	var items []listItem
	// 跳过表头行
	doc.Find("table.table2 tr, table tbody tr").Each(func(i int, s *goquery.Selection) {
		if s.Find("th").Length() > 0 {
			return
		}
		titleLink := s.Find("td a, a").First()
		href, _ := titleLink.Attr("href")
		title := strings.TrimSpace(titleLink.Text())
		if href == "" || title == "" {
			return
		}
		items = append(items, listItem{title: title, href: href})
	})

	if len(items) == 0 {
		return []model.SearchResult{}, nil
	}

	// 并发获取详情页磁力链接（有界信号量）
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	raw := make([]model.SearchResult, len(items))
	for i, it := range items {
		wg.Add(1)
		go func(idx int, it listItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			magnet, _ := p.fetchMagnet(client, it.href)
			if magnet == "" {
				return
			}
			raw[idx] = model.SearchResult{
				UniqueID: fmt.Sprintf("%s-%s", p.Name(), util.ShortHash(it.href)),
				Title:    it.title,
				Content:  "",
				Datetime: time.Now(),
				Channel:  "",
				Links: []model.Link{
					{Type: "magnet", URL: magnet, Password: ""},
				},
			}
		}(i, it)
	}
	wg.Wait()

	out := make([]model.SearchResult, 0, len(raw))
	for _, r := range raw {
		if r.UniqueID != "" {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return []model.SearchResult{}, nil
	}
	return plugin.FilterResultsByKeyword(out, searchKeyword), nil
}

// fetchMagnet 抓取详情页提取首个磁力链接
func (p *LimeTorrentsPlugin) fetchMagnet(client *http.Client, listHref string) (string, error) {
	detail := listHref
	if strings.HasPrefix(listHref, "/") {
		detail = siteURL + listHref
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, detail, nil)
	if err != nil {
		return "", err
	}
	util.SetCommonHeaders(req)
	resp, err := util.FetchWithRetry(client, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}
	if m, ok := doc.Find(`a[href^="magnet:"]`).First().Attr("href"); ok && m != "" {
		return m, nil
	}
	return "", nil
}
