package telegram

import (
	"strings"
	"testing"

	"pansou/config"
)

// TestClassifyLinkType 验证 classifyLinkType 对各类网盘/磁力链接的判定。
func TestClassifyLinkType(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"baidu", "https://pan.baidu.com/s/1abc", "baidu"},
		{"aliyun", "https://www.aliyundrive.com/s/xyz", "aliyun"},
		{"quark", "https://pan.quark.cn/s/xyz", "quark"},
		{"uc", "https://weiyun.com/xxx", "uc"},
		{"tianyi", "https://cloud.189.cn/yyy", "tianyi"},
		{"115", "https://115.com/zzz", "115"},
		{"123", "https://www.123pan.com/aaa", "123"},
		{"magnet", "magnet:?xt=urn:btih:abcdef0123456789", "magnet"},
		{"others", "https://example.com/page", "others"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyLinkType(c.url); got != c.want {
				t.Errorf("classifyLinkType(%q) = %q, want %q", c.url, got, c.want)
			}
		})
	}
}

// TestExtractPassword 验证 extractPassword 从 URL 的 pwd= 参数与邻近文本中提取提取码。
func TestExtractPassword(t *testing.T) {
	t.Run("from_url_pwd", func(t *testing.T) {
		if got := extractPassword("https://pan.baidu.com/s/1abc?pwd=ab12", ""); got != "ab12" {
			t.Errorf("从 URL 提取失败: got %q", got)
		}
	})

	t.Run("from_text_extract_code", func(t *testing.T) {
		if got := extractPassword("https://x.com", "提取码: wxyz"); got != "wxyz" {
			t.Errorf("从文本 '提取码' 提取失败: got %q", got)
		}
	})

	t.Run("from_text_password", func(t *testing.T) {
		if got := extractPassword("https://x.com", "密码：1234"); got != "1234" {
			t.Errorf("从文本 '密码' 提取失败: got %q", got)
		}
	})

	t.Run("none", func(t *testing.T) {
		if got := extractPassword("https://x.com", "无提取码信息"); got != "" {
			t.Errorf("无提取码时应为空, got %q", got)
		}
	})
}

// TestNormalizeChannel 验证 normalizeChannel 去掉前导 @ 与首尾空白。
func TestNormalizeChannel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"@mychannel", "mychannel"},
		{"mychannel", "mychannel"},
		{"  @Chan  ", "Chan"},
		{"@sub_channel_01", "sub_channel_01"},
	}
	for _, c := range cases {
		if got := normalizeChannel(c.in); got != c.want {
			t.Errorf("normalizeChannel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestDegradeModeNoCredentials 验证降级模式下未配置 TG 凭据时，
// Search 返回包含“未配置凭据”的明确错误，且不会 panic。
//
// 注意：telegram.Initialize 直接读取 config.AppConfig 字段，因此必须先在测试里
// 调用 config.Init() 初始化全局配置，避免 nil 解引用。默认环境下 TG_API_ID 等为空，
// 因此 client 保持 nil，searchImpl 返回未配置凭据错误。
func TestDegradeModeNoCredentials(t *testing.T) {
	config.Init()

	p := NewTelegramPlugin()
	_, err := p.Search("anything", nil)
	if err == nil {
		t.Fatalf("期望返回未配置凭据错误, 实际为 nil")
	}
	if !strings.Contains(err.Error(), "未配置凭据") {
		t.Errorf("错误信息应包含 '未配置凭据', 实际 %q", err.Error())
	}
}
