package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"pansou/model"
	"pansou/plugin"
)

// mockPlugin 是一个可控的 plugin.AsyncSearchPlugin 实现，用于验证健康探针的行为。
// 它同时实现了 GetMainCacheKey/SetMainCacheKey，以满足 health 包内未导出的
// mainCacheAccessor 接口，从而可以验证探针在探测期间是否临时清空主缓存键（缓存隔离）。
type mockPlugin struct {
	name     string
	priority int
	fail     bool

	mu            sync.Mutex
	mainCacheKey  string // 模拟 BaseAsyncPlugin.MainCacheKey 共享字段
	searchCalled  bool
	cacheKeyAtRun string // Search 被调用瞬间的主缓存键值（用于断言隔离）
}

// 编译期接口断言，确保 mockPlugin 满足所需契约。
var (
	_ plugin.AsyncSearchPlugin = (*mockPlugin)(nil)
	_ mainCacheAccessor        = (*mockPlugin)(nil)
)

func newMockPlugin(name string, priority int, fail bool) *mockPlugin {
	return &mockPlugin{name: name, priority: priority, fail: fail}
}

func (m *mockPlugin) Name() string { return m.name }

func (m *mockPlugin) Priority() int { return m.priority }

func (m *mockPlugin) AsyncSearch(
	keyword string,
	searchFunc func(*http.Client, string, map[string]interface{}) ([]model.SearchResult, error),
	mainCacheKey string,
	ext map[string]interface{},
) ([]model.SearchResult, error) {
	return m.Search(keyword, ext)
}

func (m *mockPlugin) SetMainCacheKey(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mainCacheKey = key
}

func (m *mockPlugin) GetMainCacheKey() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mainCacheKey
}

func (m *mockPlugin) SetCurrentKeyword(keyword string) {}

func (m *mockPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	m.mu.Lock()
	m.searchCalled = true
	// 探针在进入 Search 前应将主缓存键临时清空为 ""，这里记录实际值以验证隔离。
	m.cacheKeyAtRun = m.mainCacheKey
	m.mu.Unlock()

	if m.fail {
		return nil, errors.New("mock probe failure: " + m.name)
	}
	return []model.SearchResult{
		{
			UniqueID: m.name + "-1",
			Title:    "ok-result",
			Channel:  "",
			Links:    []model.Link{{Type: "others", URL: "https://example.com/" + m.name}},
		},
	}, nil
}

func (m *mockPlugin) SkipServiceFilter() bool { return false }

// 包级共享的探针实例与 mock 插件，在 TestMain 中一次性注册，避免全局注册表被多个测试反复污染。
var (
	testChecker       *HealthChecker
	mockOKA, mockOKB  *mockPlugin
	mockFail          *mockPlugin
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	mockOKA = newMockPlugin("m_ok_a", 1, false)
	mockOKB = newMockPlugin("m_ok_b", 2, false)
	mockFail = newMockPlugin("m_fail_c", 3, true)

	for _, mp := range []*mockPlugin{mockOKA, mockOKB, mockFail} {
		// 预设一个非空的“真实”主缓存键，用于验证探针探测时会被临时清空并随后恢复。
		mp.SetMainCacheKey("user-" + mp.Name())
		plugin.RegisterGlobalPlugin(mp)
	}

	// NewHealthChecker 会同时登记为包内全局单例 globalChecker，
	// 路由处理函数通过 GetHealthChecker() 读取的正是同一实例。
	testChecker = NewHealthChecker()

	os.Exit(m.Run())
}

// TestRunProbeAggregatesHealth 验证 RunProbe + GetHealth 的聚合统计是否正确，
// 以及探针是否对主缓存做了临时隔离（探测期间主缓存键为 ""，结束后恢复）。
func TestRunProbeAggregatesHealth(t *testing.T) {
	testChecker.RunProbe()

	status := testChecker.GetHealth()

	if status.Total != 3 {
		t.Fatalf("期望 Total=3, 实际 %d", status.Total)
	}
	if status.Available != 2 {
		t.Fatalf("期望 Available=2, 实际 %d", status.Available)
	}
	if status.UpdatedAt == 0 {
		t.Errorf("期望 UpdatedAt 已被设置 (非 0)")
	}

	byName := make(map[string]PluginHealth, len(status.Plugins))
	for _, ph := range status.Plugins {
		byName[ph.Name] = ph
	}

	if ph, ok := byName["m_ok_a"]; !ok || ph.Status != "ok" {
		t.Errorf("m_ok_a 应为 ok, 实际 %+v", ph)
	}
	if ph, ok := byName["m_ok_b"]; !ok || ph.Status != "ok" {
		t.Errorf("m_ok_b 应为 ok, 实际 %+v", ph)
	}
	if ph, ok := byName["m_fail_c"]; !ok || ph.Status != "fail" {
		t.Errorf("m_fail_c 应为 fail, 实际 %+v", ph)
	}
	if ph, ok := byName["m_ok_a"]; !ok || ph.ResultCount != 1 {
		t.Errorf("m_ok_a 的 ResultCount 应为 1, 实际 %d", ph.ResultCount)
	}
	if ph, ok := byName["m_fail_c"]; ok && ph.Error == "" {
		t.Errorf("m_fail_c 应带有错误信息, 实际为空")
	}

	// 验证缓存隔离：探测瞬间主缓存键曾被清空为 ""。
	if mockOKA.cacheKeyAtRun != "" {
		t.Errorf("探针探测时主缓存键应为空(隔离), 实际 %q", mockOKA.cacheKeyAtRun)
	}
	// 验证探测结束后主缓存键已恢复到预设值。
	if got := mockOKA.GetMainCacheKey(); got != "user-m_ok_a" {
		t.Errorf("探针结束后主缓存键应恢复为 user-m_ok_a, 实际 %q", got)
	}
	if !mockOKA.searchCalled {
		t.Errorf("m_ok_a.Search 应被探针调用")
	}
}

// TestHealthRoutes 通过 httptest 验证健康探针路由：
// GET  /api/health/plugins        返回 200 与聚合 JSON
// POST /api/health/plugins/check  触发 RunProbe 后返回 200 与聚合 JSON
func TestHealthRoutes(t *testing.T) {
	// 确保状态已被填充（测试执行顺序不固定时也能稳定）。
	testChecker.RunProbe()

	r := gin.New()
	api := r.Group("/api")
	RegisterHealthRoutes(api)

	// GET /api/health/plugins
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health/plugins", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/health/plugins 期望 200, 实际 %d", w.Code)
	}

	var getBody HealthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &getBody); err != nil {
		t.Fatalf("无法解析 GET 响应 JSON: %v", err)
	}
	if getBody.Total != 3 || getBody.Available != 2 {
		t.Errorf("GET 响应期望 Total=3/Available=2, 实际 %d/%d", getBody.Total, getBody.Available)
	}

	// POST /api/health/plugins/check
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/health/plugins/check", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("POST /api/health/plugins/check 期望 200, 实际 %d", w2.Code)
	}

	var postBody HealthStatus
	if err := json.Unmarshal(w2.Body.Bytes(), &postBody); err != nil {
		t.Fatalf("无法解析 POST 响应 JSON: %v", err)
	}
	if postBody.Total != 3 || postBody.Available != 2 {
		t.Errorf("POST 响应期望 Total=3/Available=2, 实际 %d/%d", postBody.Total, postBody.Available)
	}
}
