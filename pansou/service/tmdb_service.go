package service

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
)

// TMDBService 封装 TMDB 搜索建议能力
type TMDBService struct {
	apiKey  string
	proxy   string
	client  *http.Client
	baseURL string

	mu    sync.RWMutex
	cache map[string]tmdbCacheEntry
}

type tmdbCacheEntry struct {
	names     []string
	expiresAt time.Time
}

// tmdbSearchResponse TMDB /search/multi 响应结构（只取需要的字段）
type tmdbSearchResponse struct {
	Results []struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`          // 电视剧名
		Title        string `json:"title"`         // 电影名
		OriginalName string `json:"original_name"` // 原始名
		MediaType    string `json:"media_type"`    // movie/tv/person
		Overview     string `json:"overview"`
	} `json:"results"`
	TotalResults int `json:"total_results"`
}

// NewTMDBService 创建 TMDB 服务实例
func NewTMDBService() *TMDBService {
	s := &TMDBService{
		cache: make(map[string]tmdbCacheEntry),
	}
	if config.AppConfig != nil {
		s.apiKey = config.AppConfig.TMDBAPIKey
		s.proxy = config.AppConfig.TMDBProxy
		s.baseURL = "https://api.themoviedb.org/3"
	}
	// HTTP 客户端：超时 8 秒，可选代理
	s.client = &http.Client{Timeout: 8 * time.Second}
	if s.proxy != "" {
		if proxyURL, err := url.Parse(s.proxy); err == nil {
			s.client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}
	return s
}

// Enabled 是否启用
func (s *TMDBService) Enabled() bool {
	return s != nil && s.apiKey != ""
}

// Suggest 搜索建议：返回与 query 相关的影视剧名
// 命中缓存直接返回；未命中调 TMDB /search/multi 并缓存 7 天
func (s *TMDBService) Suggest(query string, limit int) []string {
	if !s.Enabled() || strings.TrimSpace(query) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}

	key := strings.ToLower(strings.TrimSpace(query))

	// 查缓存
	s.mu.RLock()
	if entry, ok := s.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		if len(entry.names) > limit {
			return entry.names[:limit]
		}
		return entry.names
	}
	s.mu.RUnlock()

	// 调 TMDB
	names := s.callTMDBSearch(key)
	if names == nil {
		return nil
	}

	// 写缓存 7 天
	s.mu.Lock()
	s.cache[key] = tmdbCacheEntry{
		names:     names,
		expiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	s.mu.Unlock()

	if len(names) > limit {
		return names[:limit]
	}
	return names
}

// callTMDBSearch 调 TMDB /search/multi 接口
func (s *TMDBService) callTMDBSearch(query string) []string {
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("language", "zh-CN")
	params.Set("query", query)
	params.Set("page", "1")
	params.Set("include_adult", "false")

	reqURL := fmt.Sprintf("%s/search/multi?%s", s.baseURL, params.Encode())

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var result tmdbSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}

	// 提取名称：优先 movie/tv，跳过 person
	names := make([]string, 0, len(result.Results))
	seen := make(map[string]bool)
	for _, item := range result.Results {
		if item.MediaType == "person" {
			continue
		}
		name := item.Name
		if name == "" {
			name = item.Title
		}
		if name == "" {
			name = item.OriginalName
		}
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
