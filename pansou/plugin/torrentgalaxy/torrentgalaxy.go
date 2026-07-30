package torrentgalaxy

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
	siteURL        = "https://torrentgalaxy.to"
)

// TorrentGalaxyPlugin TorrentGalaxy 国际 BT 索引器（L2，覆盖 音乐/影视）
type TorrentGalaxyPlugin struct {
	*plugin.BaseAsyncPlugin
}

func init() {
	plugin.RegisterGlobalPlugin(NewTorrentGalaxyPlugin())
}

// NewTorrentGalaxyPlugin 创建插件实例（优先级 3，跳过 Service 过滤，插件内自过滤）
func NewTorrentGalaxyPlugin() *TorrentGalaxyPlugin {
	return &TorrentGalaxyPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPluginWithFilter("torrentgalaxy", 3, true),
	}
}

// Search 兼容性方法
func (p *TorrentGalaxyPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	res, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return res.Results, nil
}

// SearchWithResult 返回带 IsFinal 标记的结果
func (p *TorrentGalaxyPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实际搜索逻辑（列表页直接含磁力链接）
func (p *TorrentGalaxyPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	searchKeyword := keyword
	if ext != nil {
		if v, ok := ext["health_probe"]; ok && v == true && util.IsCJK(keyword) {
			searchKeyword = "test"
		}
	}
	searchURL := fmt.Sprintf("%s/search?search=%s&sort=seeders&order=desc", siteURL, url.QueryEscape(searchKeyword))

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
	doc.Find("div.tgxtablerow, .tgxtable div[class*='tgxtablerow']").Each(func(i int, s *goquery.Selection) {
		titleLink := s.Find("a.txlight, a").First()
		title := strings.TrimSpace(titleLink.Text())
		if title == "" {
			return
		}
		magnetLink := s.Find(`a[href^="magnet:"]`).First()
		magnet, ok := magnetLink.Attr("href")
		if !ok || magnet == "" {
			return
		}
		results = append(results, model.SearchResult{
			UniqueID: fmt.Sprintf("%s-%s", p.Name(), util.ShortHash(magnet)),
			Title:    title,
			Content:  "",
			Datetime: time.Now(),
			Channel:  "",
			Links: []model.Link{
				{Type: "magnet", URL: magnet, Password: ""},
			},
		})
	})

	if len(results) == 0 {
		return []model.SearchResult{}, nil
	}
	return plugin.FilterResultsByKeyword(results, searchKeyword), nil
}
