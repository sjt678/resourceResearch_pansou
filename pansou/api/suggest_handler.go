package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"pansou/config"
	"pansou/service"
	jsonutil "pansou/util/json"
)

// 全局 TMDB 服务实例
var tmdbService *service.TMDBService

// SetTMDBService 设置 TMDB 服务实例
func SetTMDBService(s *service.TMDBService) {
	tmdbService = s
}

// SuggestHandler 搜索建议处理函数
// GET /api/suggest?q=关键词&limit=8
// 返回相关影视剧名列表，用于输入框联想
func SuggestHandler(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"suggestions": []string{}})
		return
	}

	limit := 8
	if l := c.Query("limit"); l != "" {
		if n, err := parseIntSafe(l); err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}

	var names []string
	if tmdbService != nil && tmdbService.Enabled() {
		names = tmdbService.Suggest(q, limit)
	}

	if len(names) == 0 {
		names = []string{}
	}

	response := gin.H{
		"code":        0,
		"message":     "ok",
		"data":        gin.H{"suggestions": names},
		"suggestions": names,
		"tmdb_enabled": config.AppConfig != nil && config.AppConfig.TMDBEnabled,
	}
	jsonData, _ := jsonutil.Marshal(response)
	c.Data(http.StatusOK, "application/json", jsonData)
}

// parseIntSafe 安全解析整数
func parseIntSafe(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
