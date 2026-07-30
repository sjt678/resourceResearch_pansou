package health

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"pansou/config"
	"pansou/plugin"
)

// PluginHealth 单个插件健康信息
type PluginHealth struct {
	Name        string `json:"name"`
	Status      string `json:"status"`       // "ok" | "fail"
	LatencyMs   int64  `json:"latency"`      // 本次探测耗时（毫秒）
	ResultCount int    `json:"resultCount"`  // 返回结果数量
	LastChecked int64  `json:"lastChecked"`  // 最近一次检测时间（unix 毫秒）
	Error       string `json:"error,omitempty"`
}

// HealthStatus 聚合健康状态
type HealthStatus struct {
	UpdatedAt int64          `json:"updatedAt"`
	Total     int            `json:"total"`
	Available int            `json:"available"`
	Plugins   []PluginHealth `json:"plugins"`
}

// mainCacheAccessor 用于探针临时隔离主缓存（与主缓存键读写相关）
type mainCacheAccessor interface {
	GetMainCacheKey() string
	SetMainCacheKey(key string)
}

// HealthChecker 源健康度探针
type HealthChecker struct {
	mu           sync.RWMutex
	statuses     map[string]PluginHealth
	probeKeyword string
	interval     time.Duration
	lastRunAt    time.Time
}

// globalChecker 进程内单例，确保定时调度、API 读取的是同一份状态
var globalChecker *HealthChecker

// NewHealthChecker 创建探针实例（从 config 读取关键词与间隔），并登记为全局单例
func NewHealthChecker() *HealthChecker {
	keyword := "测试"
	interval := 30 * time.Minute
	if config.AppConfig != nil {
		if config.AppConfig.HealthProbeKeyword != "" {
			keyword = config.AppConfig.HealthProbeKeyword
		}
		interval = config.AppConfig.HealthCheckInterval
	}
	hc := &HealthChecker{
		statuses:     make(map[string]PluginHealth),
		probeKeyword: keyword,
		interval:     interval,
	}
	globalChecker = hc
	return hc
}

// GetHealthChecker 获取全局探针单例（惰性创建）
func GetHealthChecker() *HealthChecker {
	if globalChecker == nil {
		return NewHealthChecker()
	}
	return globalChecker
}

// probeOne 对单个插件执行一次探测，临时隔离主缓存避免污染业务缓存
func (h *HealthChecker) probeOne(p plugin.AsyncSearchPlugin) PluginHealth {
	ph := PluginHealth{
		Name:        p.Name(),
		Status:      "fail",
		LastChecked: time.Now().UnixMilli(),
	}

	// 临时清空主缓存键，使本次搜索不写入业务主缓存
	if accessor, ok := p.(mainCacheAccessor); ok {
		save := accessor.GetMainCacheKey()
		accessor.SetMainCacheKey("")
		defer accessor.SetMainCacheKey(save)
	}

	start := time.Now()
	results, err := p.Search(h.probeKeyword, map[string]interface{}{
		"health_probe": true,
		"refresh":      true,
	})
	ph.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		ph.Error = err.Error()
		ph.ResultCount = 0
		return ph
	}

	ph.ResultCount = len(results)
	ph.Status = "ok"
	return ph
}

// RunProbe 对全部已注册插件执行一次探测（有界并发，避免阻塞过久）
func (h *HealthChecker) RunProbe() {
	plugins := plugin.GetRegisteredPlugins()
	if len(plugins) == 0 {
		return
	}

	workerCount := len(plugins)
	if workerCount > 20 {
		workerCount = 20
	}
	sem := make(chan struct{}, workerCount)

	results := make([]PluginHealth, len(plugins))
	var wg sync.WaitGroup

	for i, p := range plugins {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, pl plugin.AsyncSearchPlugin) {
			defer wg.Done()
			defer func() { <-sem }()
			ph := h.probeOne(pl)
			results[idx] = ph
			if ph.Status == "ok" {
				fmt.Printf("OK   %s %dms (results=%d)\n", ph.Name, ph.LatencyMs, ph.ResultCount)
			} else {
				fmt.Printf("FAIL %s %dms err=%s\n", ph.Name, ph.LatencyMs, ph.Error)
			}
		}(i, p)
	}
	wg.Wait()

	// 一次性写入状态表，避免读到半截数据
	h.mu.Lock()
	h.statuses = make(map[string]PluginHealth, len(results))
	available := 0
	for _, ph := range results {
		h.statuses[ph.Name] = ph
		if ph.Status == "ok" {
			available++
		}
	}
	h.lastRunAt = time.Now()
	h.mu.Unlock()

	fmt.Printf("源可用性: %d/%d\n", available, len(results))
}

// GetHealth 聚合返回当前健康状态
func (h *HealthChecker) GetHealth() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	plugins := make([]PluginHealth, 0, len(h.statuses))
	available := 0
	for _, ph := range h.statuses {
		plugins = append(plugins, ph)
		if ph.Status == "ok" {
			available++
		}
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	return HealthStatus{
		UpdatedAt: h.lastRunAt.UnixMilli(),
		Total:     len(plugins),
		Available: available,
		Plugins:   plugins,
	}
}

// StartScheduler 启动探针调度：立即在后台跑一次启动自检（不阻塞 main 启动）；
// 若 interval > 0 则周期性调度，否则仅保留启动自检 + 手动触发。
func (h *HealthChecker) StartScheduler() {
	// 立即在后台跑一次启动自检，对运维可见
	go h.RunProbe()

	if h.interval <= 0 {
		// 0=仅启动自检+手动触发
		return
	}
	go func() {
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for range ticker.C {
			h.RunProbe()
		}
	}()
}

// RegisterHealthRoutes 在 /api 路由组下注册健康探针路由：
//   GET  /api/health/plugins        返回 GetHealth() 的 JSON（200）
//   POST /api/health/plugins/check  强制 RunProbe() 后返回最新 JSON
func RegisterHealthRoutes(api *gin.RouterGroup) {
	api.GET("/health/plugins", func(c *gin.Context) {
		c.JSON(http.StatusOK, GetHealthChecker().GetHealth())
	})
	api.POST("/health/plugins/check", func(c *gin.Context) {
		GetHealthChecker().RunProbe()
		c.JSON(http.StatusOK, GetHealthChecker().GetHealth())
	})
}
