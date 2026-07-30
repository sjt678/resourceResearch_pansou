package health

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"pansou/config"
	"pansou/model"
	"pansou/plugin"
)

// 探针超时控制（避免单个/整体卡死导致 /api/health/plugins 永远返回 total:0）
const (
	probeOneTimeout   = 15 * time.Second  // 单源探测硬超时：超过则标记该源 fail，不阻塞整体
	probeTotalTimeout = 120 * time.Second // 整体探针安全超时：到点即写入已得结果
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
	start := time.Now()
	ph := PluginHealth{
		Name:        p.Name(),
		Status:      "fail",
		LastChecked: start.UnixMilli(),
	}

	// 临时清空主缓存键，使本次搜索不写入业务主缓存
	if accessor, ok := p.(mainCacheAccessor); ok {
		save := accessor.GetMainCacheKey()
		accessor.SetMainCacheKey("")
		defer accessor.SetMainCacheKey(save)
	}

	// 单源硬超时：避免某个源 Search 卡死（如代理节点挂起、连接不释放）
	// 拖垮整个探针，导致 wg.Wait 永久阻塞、lastRunAt 永不写入。
	type probeResult struct {
		results []model.SearchResult
		err     error
	}
	ch := make(chan probeResult, 1)
	go func() {
		r, e := p.Search(h.probeKeyword, map[string]interface{}{
			"health_probe": true,
			"refresh":      true,
		})
		ch <- probeResult{r, e}
	}()

	select {
	case out := <-ch:
		ph.LatencyMs = time.Since(start).Milliseconds()
		if out.err != nil {
			ph.Error = out.err.Error()
			ph.ResultCount = 0
			return ph
		}
		ph.ResultCount = len(out.results)
		ph.Status = "ok"
		return ph
	case <-time.After(probeOneTimeout):
		ph.Error = "探测超时(单源超过阈值)"
		ph.LatencyMs = probeOneTimeout.Milliseconds()
		return ph
	}
}

// RunProbe 对全部已注册插件执行一次探测（有界并发，避免阻塞过久）
func (h *HealthChecker) RunProbe() {
	plugins := plugin.GetRegisteredPlugins()

	// 先打时间戳：避免任何提前返回或整体未完成时，GET 永远看到零值时间(-62135596800000)
	h.mu.Lock()
	h.lastRunAt = time.Now()
	h.mu.Unlock()

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

	// 整体安全超时：即便个别源未在单源超时内返回（极端情况），也确保探针结束并写入已得结果
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(probeTotalTimeout):
		fmt.Printf("[Health] 探针整体超过安全阈值(%.0fs)，写入已探测的部分结果\n", probeTotalTimeout.Seconds())
	}

	// 一次性写入状态表，避免读到半截数据
	h.mu.Lock()
	h.statuses = make(map[string]PluginHealth, len(results))
	available := 0
	for _, ph := range results {
		if ph.Name == "" {
			continue
		}
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
