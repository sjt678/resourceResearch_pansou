package util

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent 通用浏览器 User-Agent，避免被站点反爬拦截
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// SetCommonHeaders 为请求设置通用反爬请求头
func SetCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
}

// FetchWithRetry 带克隆请求重试的 HTTP 请求，仅在返回 2xx 时成功；
// 全部重试失败或无响应时返回 error。调用方负责关闭返回的响应体。
func FetchWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	const maxRetries = 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(1<<uint(i-1)) * 200 * time.Millisecond)
		}
		// 克隆请求，避免并发/重试时复用同一个 req 导致的问题
		reqClone := req.Clone(req.Context())
		resp, err := client.Do(reqClone)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("HTTP 请求重试 %d 次后仍失败", maxRetries)
	}
	return nil, lastErr
}

// IsCJK 判断字符串是否包含中日韩字符（用于探针态下的英文关键词降级）
func IsCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3040 && r <= 0x30FF) || (r >= 0xAC00 && r <= 0xD7AF) {
			return true
		}
	}
	return false
}

// ShortHash 返回字符串的 8 位十六进制 fnv 哈希，用于从 URL 等派生稳定的 UniqueID
func ShortHash(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

// HasPrefixAny 判断 s 是否以任一前缀开头
func HasPrefixAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
