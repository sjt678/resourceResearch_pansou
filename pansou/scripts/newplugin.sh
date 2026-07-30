#!/usr/bin/env bash
set -euo pipefail

# 解析插件名：优先取第一个位置参数，其次取环境变量 NAME
NAME="${1:-${NAME:-}}"
if [ -z "$NAME" ]; then
  echo "用法: make newplugin NAME=<插件名>   或   bash scripts/newplugin.sh <插件名>"
  exit 1
fi

# 包名/目录名统一小写
if [ -d "plugin/$NAME" ]; then
  echo "错误: plugin/$NAME 已存在"
  exit 1
fi

mkdir -p "plugin/$NAME"

# 首字母大写，用于类型/函数命名
NAMECAP="$(echo "$NAME" | sed 's/^\(.\)/\U\1/')"

# 类型/常量名不允许以数字开头：数字开头的插件名，NAMECAP 加前缀 "X"
# 例如 NAME=1337x -> NAMECAP=X1337x；仅影响类型/常量命名，目录与插件标识仍用原始 NAME
case "$NAMECAP" in
  [0-9]*)
    NAMECAP="X${NAMECAP}"
    ;;
esac

# 包名清洗：Go 包名不允许以数字开头，数字开头的插件名需加前缀 "x"
# 例如 NAME=1337x -> PKGNAME=x1337x；目录名与插件标识仍使用原始 NAME
case "$NAME" in
  [0-9]*)
    PKGNAME="x${NAME}"
    ;;
  *)
    PKGNAME="${NAME}"
    ;;
esac

cat > "plugin/$NAME/$NAME.go" <<'GOEOF'
package __PKGNAME__

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"pansou/model"
	"pansou/plugin"
	"pansou/util"
)

// 标准浏览器 User-Agent
const userAgent__NAMECAP__ = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// 默认请求超时
const defaultTimeout__NAMECAP__ = 8 * time.Second

// __NAMECAP__Plugin 搜索插件（脚手架生成，请按需实现 searchImpl）
type __NAMECAP__Plugin struct {
	*plugin.BaseAsyncPlugin
}

// New__NAMECAP__Plugin 创建插件实例
func New__NAMECAP__Plugin() *__NAMECAP__Plugin {
	return &__NAMECAP__Plugin{
		// 普通源优先级3；磁力/BT/宽泛源用 plugin.NewBaseAsyncPluginWithFilter("__NAME__", 3, true)
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("__NAME__", 3),
	}
}

func init() {
	plugin.RegisterGlobalPlugin(New__NAMECAP__Plugin())
	// 别忘了在 main.go 添加空导入: _ "pansou/plugin/__NAME__"
}

// Search 兼容性方法
func (p *__NAMECAP__Plugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 返回带 IsFinal 标记的结果
func (p *__NAMECAP__Plugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实际搜索逻辑（示例骨架，请替换 URL 与解析）
func (p *__NAMECAP__Plugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	searchURL := fmt.Sprintf("https://example.com/search?q=%s", url.QueryEscape(keyword))

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout__NAMECAP__)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s] 创建请求失败: %w", p.Name(), err)
	}
	req.Header.Set("User-Agent", userAgent__NAMECAP__)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := util.FetchWithRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("[%s] 搜索请求失败: %w", p.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("[%s] 搜索请求返回状态码: %d", p.Name(), resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 读取响应失败: %w", p.Name(), err)
	}
	_ = body // TODO: 用 goquery / 正则解析 body，构造 []model.SearchResult

	// TODO: 解析响应体生成 results，确保每个 result 的 Links 非空、Channel=""
	var results []model.SearchResult

	// 关键词过滤：普通源走 Service 过滤；BT/宽泛源可在此用 plugin.FilterResultsByKeyword
	results = plugin.FilterResultsByKeyword(results, keyword)
	return results, nil
}
GOEOF

# 替换占位符
sed -i "s/__NAME__/$NAME/g; s/__NAMECAP__/$NAMECAP/g; s/__PKGNAME__/$PKGNAME/g" "plugin/$NAME/$NAME.go"

echo "已生成 plugin/$NAME/$NAME.go"
echo "下一步: 在 main.go 添加 _ \"pansou/plugin/$NAME\" 空导入"
