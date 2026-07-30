# PanSou Telegram 频道适配器（plugin/telegram）

通用 Telegram 频道搜索适配器。通过 MTProto 用户态客户端读取**频道历史消息**并按关键词检索其中的网盘分享链接（百度/阿里/夸克/UC/天翼/115/123/磁力等）。

> 当前实现为**降级模式**：不引入 `github.com/gotd/td`，插件照常注册与可探测；在无凭据时探针会标记为 `fail` 并给出明确原因，**不影响其余 89+ 源与整体编译/启动**。

## 一、启用真实搜索所需的依赖

真实客户端使用 [gotd/td](https://github.com/gotd/td)（MTProto，预生成绑定，无需构建期 codegen）：

```bash
go get github.com/gotd/td
go mod tidy
```

> 为什么用 MTProto 而非 Bot API：Bot 无法读取公开频道历史消息，不满足"按关键词搜频道"的需求。

## 二、申请凭据（api_id / api_hash）

1. 打开 <https://my.telegram.org> 并使用手机号登录。
2. 进入 **API development tools**，填写任意应用名称，得到 `App api_id` 与 `App api_hash`。
3. 设置为环境变量（**不要提交进仓库**）：

```bash
export TG_API_ID=1234567
export TG_API_HASH=abcdef0123456789abcdef0123456789
```

## 三、一次性登录生成 session

MTProto 用户态客户端需要一次交互式登录以生成 `session` 文件。部署侧完成（PanSou 进程内不内置交互登录）：

```bash
export TG_SESSION_PATH=/path/to/telegram.session
# 使用 gotd/td 提供的交互式登录工具或自行编写一次性登录脚本生成 session 文件
```

## 四、频道配置

编辑（或新建）`plugin/telegram/config.yaml`，或通过 `TG_CONFIG_PATH` 指向外部文件：

```yaml
api:
  session_path: ""   # 留空则读 TG_SESSION_PATH
channels:
  - channel: "example_channel"
    cloud_type: "quark"
    keyword_map: {}
    enabled: true
```

字段说明：

| 字段 | 说明 |
|---|---|
| `channel` | 频道用户名（可带或不带 `@`，加载时自动规范化） |
| `cloud_type` | 该频道主要分享的网盘类型，仅作标签/过滤参考 |
| `keyword_map` | 可选：类别关键词映射，仅记录，不参与硬过滤 |
| `enabled` | 是否启用该频道（缺省视为 `true`） |

## 五、接入 gotd/td 的改造点

在 `telegram.go` 中：

1. `Initialize()` 内用 `TG_API_ID / TG_API_HASH / TG_SESSION_PATH` 创建 `*tg.Client` 并赋值 `p.client`。
2. `searchImpl()` 的 `p.client != nil` 分支并发读取各 `Enabled` 频道消息，调用 `extractLinksFromText` 解析网盘链接，构造：

```go
model.SearchResult{
    UniqueID: fmt.Sprintf("telegram-%s-%d", channel, msgID),
    Title:    ...,
    Channel:  "@" + channel,
    Links:    []model.Link{{Type: ..., URL: ..., Password: ...}},
}
```

完成接入后，健康探针（`GET /api/health/plugins`）中将显示 `telegram` 为 `ok` 且 `resultCount > 0`。
