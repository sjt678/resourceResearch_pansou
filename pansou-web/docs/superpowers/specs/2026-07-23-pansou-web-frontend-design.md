# PanSou Web 前端设计文档

**日期**: 2026-07-23
**项目**: pansou-web（网盘资源搜索前端）
**状态**: 设计已确认，待实现

---

## 1. 项目概述

### 1.1 目标
为 pansou 后端（Go + Gin 实现的网盘资源聚合搜索 API）开发一个完整、功能全面、开箱即用的前端项目。前端独立部署在 pansou 项目旁的同级目录 `d:\Desktop\pansou_project\pansou-web`。

### 1.2 范围
- ✅ 核心搜索功能（搜索框 + 结果列表 + 网盘类型 Tab + 关键词高亮 + 一键复制）
- ✅ 搜索历史记录（localStorage，最近 20 条）
- ✅ 链接有效性检测（调用后端 `/api/check/links`）
- ❌ 不包含：资源收藏夹、用户登录、主题切换（留扩展位）

### 1.3 用户故事
作为网盘资源搜索用户，我希望：
1. 输入关键词回车即可搜索 100+ TG 频道和 89 个搜索源
2. 按网盘类型（夸克/百度/阿里等）快速筛选结果
3. 一键复制网盘链接，无需手动选择文本
4. 检测链接是否失效，避免点进去发现被删
5. 查看搜索历史，快速重搜

---

## 2. 技术栈

| 层级 | 技术 | 版本 | 用途 |
|---|---|---|---|
| 框架 | Vue 3 | ^3.4 | Composition API + `<script setup>` |
| 构建 | Vite | ^5.0 | 开发热更新、生产构建 |
| UI 库 | Element Plus | ^2.7 | 组件库 |
| 状态 | Pinia | ^2.1 | 全局状态（API URL 等） |
| 路由 | Vue Router | ^4.3 | 预留扩展（单页面其实够用） |
| HTTP | Axios | ^1.7 | 请求库 |
| 工具 | @vueuse/core | ^10.9 | localStorage 响应式封装等 |

### 2.1 目录结构

```
d:\Desktop\pansou_project\pansou-web\
├── public/
│   └── favicon.ico
├── src/
│   ├── api/
│   │   ├── index.js          # axios 实例 + 拦截器
│   │   ├── search.js         # 搜索接口封装
│   │   └── check.js          # 链接检测接口封装
│   ├── components/
│   │   ├── SearchBar.vue     # 搜索框（含历史下拉）
│   │   ├── ResultTabs.vue    # 网盘类型 Tab 切换
│   │   ├── ResultCard.vue    # 单条资源卡片
│   │   ├── CheckButton.vue   # 链接检测按钮
│   │   └── EmptyState.vue    # 空状态占位
│   ├── composables/
│   │   ├── useHistory.js     # 搜索历史 localStorage 封装
│   │   └── useSearch.js      # 搜索逻辑封装（loading/error）
│   ├── views/
│   │   └── Home.vue          # 唯一主页面
│   ├── stores/
│   │   └── config.js         # Pinia store（API base URL 等）
│   ├── styles/
│   │   └── variables.css     # CSS 变量（主题色、圆角、阴影）
│   ├── App.vue
│   ├── main.js
│   └── router.js
├── index.html
├── package.json
├── vite.config.js
└── README.md
```

---

## 3. 设计系统

### 3.1 配色方案（明亮简洁现代）

采用**湖蓝**作为主色，传达科技感但不冷漠。

| 用途 | CSS 变量 | 色值 | 说明 |
|---|---|---|---|
| 主色 Primary | `--color-primary` | `#0EA5E9` | 按钮、链接、高亮 |
| 主色悬浮 | `--color-primary-hover` | `#0284C7` | hover 状态 |
| 主色浅 | `--color-primary-light` | `#E0F2FE` | 背景、标签底色 |
| 成功 | `--color-success` | `#10B981` | 链接有效 |
| 警告 | `--color-warning` | `#F59E0B` | 待检测 |
| 危险 | `--color-danger` | `#EF4444` | 链接失效 |
| 背景 | `--color-bg` | `#F8FAFC` | slate-50，柔和米白 |
| 卡片底 | `--color-card` | `#FFFFFF` | 纯白卡片 |
| 主文本 | `--color-text` | `#0F172A` | slate-900 |
| 次文本 | `--color-text-secondary` | `#64748B` | slate-500 |
| 边框 | `--color-border` | `#E2E8F0` | slate-200 |

### 3.2 网盘类型品牌色映射

| 网盘 | 品牌色 | 图标字符 |
|---|---|---|
| quark 夸克 | `#3B82F6` 蓝 | Q |
| baidu 百度 | `#06B6D4` 青 | 百 |
| aliyun 阿里 | `#F97316` 橙 | 阿 |
| 115 | `#84CC16` 绿 | 115 |
| weiyun 微云 | `#8B5CF6` 紫 | 微 |
| others | `#64748B` 灰 | 链 |

### 3.3 字体规范

```css
--font-display: 'Manrope', system-ui, sans-serif;  /* 标题字体 */
--font-body: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; /* 正文 */
--font-mono: 'JetBrains Mono', monospace;  /* 链接、代码 */
```

### 3.4 圆角 & 阴影

```css
--radius-sm: 8px;    /* 小元素：标签、按钮 */
--radius-md: 12px;   /* 中元素：输入框 */
--radius-lg: 16px;   /* 大元素：卡片 */
--radius-xl: 24px;   /* 特殊：主容器 */

--shadow-sm: 0 1px 2px rgba(15, 23, 42, .04), 0 1px 3px rgba(15, 23, 42, .06);
--shadow-md: 0 4px 6px -1px rgba(15, 23, 42, .06), 0 2px 4px -2px rgba(15, 23, 42, .05);
--shadow-lg: 0 10px 25px -5px rgba(15, 23, 42, .08), 0 8px 10px -6px rgba(15, 23, 42, .05);
```

### 3.5 动效规范

| 场景 | 动效 | 时长 |
|---|---|---|
| 卡片入场 | translateY(20px) → 0 + opacity 0→1 | 400ms ease-out |
| 列表项依次出现 | stagger delay 50ms × index | 错峰 |
| 按钮 hover | scale(1.02) + 阴影加深 | 200ms |
| Tab 切换 | 淡入淡出 | 200ms |
| 搜索 loading | 骨架屏 | - |
| 复制成功 | 按钮 → ✓ → 回弹 | 600ms |

### 3.6 布局规范

- **最大宽度**: `max-width: 1200px`，居中
- **响应式断点**:
  - `≤768px`: 单列卡片，搜索框全宽
  - `769-1024px`: 双列卡片
  - `≥1025px`: 三列卡片（默认）

---

## 4. 页面布局 & 组件设计

### 4.1 整体页面结构（单页面 Home.vue）

```
┌─────────────────────────────────────────────────┐
│  [Header 顶栏]                                   │
│   Logo "PanSou"  ·  GitHub 图标                  │
├─────────────────────────────────────────────────┤
│         [Hero 搜索区]                            │
│         大标题 "网盘资源搜索"                      │
│         副标题 "聚合 100+ TG 频道 · 89 个搜索源"  │
│      ┌─────────────────────────────┐            │
│      │ 🔍  搜索关键词...    [搜索]  │            │
│      └─────────────────────────────┘            │
│      最近搜索: 速度与激情 | 复仇者联盟 | 三体      │
├─────────────────────────────────────────────────┤
│  [搜索结果区 - 搜索后显示]                        │
│  找到 247 条结果 · 用时 2.3s                     │
│  ┌──────┬──────┬──────┬──────┬──────┐          │
│  │全部247│夸克85│百度60│阿里45 │其他57 │          │
│  └──────┴──────┴──────┴──────┴──────┘          │
│  ┌──────────────┐ ┌──────────────┐ ┌────────┐  │
│  │ [结果卡片1]    │ │ [结果卡片2]    │ │ [卡片3] │  │
│  │ 标题...        │ │ 标题...        │ │        │  │
│  │ 📁夸克 12-01  │ │ 📁百度 11-30  │ │        │  │
│  │ [复制] [检测]  │ │ [复制] [检测]  │ │        │  │
│  └──────────────┘ └──────────────┘ └────────┘  │
│  [加载更多] 或 [已经到底啦]                      │
└─────────────────────────────────────────────────┘
```

### 4.2 核心组件设计

#### 4.2.1 SearchBar.vue（搜索框）
- 回车触发搜索
- 历史下拉（最多 20 条，localStorage）
- 清空历史按钮
- Loading 时按钮变 loading 状态

#### 4.2.2 ResultTabs.vue（结果分类 Tab）
- 切换显示不同网盘类型的结果
- 数字徽章显示数量
- 品牌色图标标识

#### 4.2.3 ResultCard.vue（资源卡片）⭐ 核心
```
┌────────────────────────────────────┐
│ 🎬 速度与激情10.4K HDR版           │  ← 标题 + 关键词高亮
│ 描述内容预览，支持换行...           │  ← content 截断
│ 📁 夸克网盘   📅 2024-12-01   3小时前│  ← 网盘类型 + 时间
│ [📋 复制链接] [🔍 检测有效性] [↗]  │  ← 操作按钮
└────────────────────────────────────┘
```

**状态变化**：
- 检测中：按钮变 loading
- 检测有效：绿色边框 + ✓ 标识
- 检测失效：红色边框 + ✗ 标识 + 链接划线
- 复制成功：按钮变 ✓ "已复制" 600ms 后恢复

#### 4.2.4 CheckButton.vue（链接检测按钮）
封装检测逻辑：
- 点击 → 调 `/api/check/links`
- loading 状态
- 结果反馈（el-message toast）

#### 4.2.5 EmptyState.vue（空状态）
- 初始状态：展示使用提示
- 无结果：展示"没找到" + 建议
- 搜索中：骨架屏

### 4.3 数据流

```
用户输入 → SearchBar 触发 search
  → useSearch composable 调用 API
  → useHistory 保存搜索词到 localStorage
  → 返回数据 → ResultTabs 按网盘类型分组
  → ResultCard 渲染卡片
  → CheckButton 触发链接检测（独立请求）
```

### 4.4 关键交互细节

1. **搜索触发**: 回车触发，不做输入即搜
2. **URL 同步**: 搜索词同步到 URL `?q=xxx`，刷新页面保留搜索
3. **骨架屏**: 搜索 loading 时显示 3-6 个骨架卡片
4. **错误重试**: API 失败显示错误状态 + 重试按钮
5. **虚拟滚动**: 结果超过 50 条时启用虚拟滚动，避免卡顿

---

## 5. API 对接

### 5.1 搜索接口

**`GET /api/search`**

请求参数（query string）:
| 参数 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|
| `kw` | string | ✅ | - | 搜索关键词 |
| `src` | string | ❌ | `all` | 数据源：`all`/`tg`/`plugin` |
| `res` | string | ❌ | `merge` | 返回类型：`merge`（按网盘类型分组） |
| `cloud_types` | string | ❌ | 空 | 逗号分隔的网盘类型 |

响应结构:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 247,
    "merged_by_type": {
      "quark": [
        {
          "url": "https://pan.quark.cn/s/xxx",
          "password": "",
          "note": "速度与激情10.4K HDR",
          "datetime": "2024-12-01T...",
          "source": "tg:Aliyun_4K_Movies",
          "images": ["https://..."]
        }
      ],
      "baidu": [...],
      "aliyun": [...]
    }
  }
}
```

前端数据转换（拍平成统一数组）:
```javascript
function flattenResults(mergedByType) {
  return Object.entries(mergedByType).flatMap(([cloudType, items]) =>
    items.map(item => ({ ...item, cloudType }))
  )
}
```

### 5.2 链接检测接口

**`POST /api/check/links`**

请求体:
```json
{
  "items": [
    {
      "disk_type": "quark",
      "url": "https://pan.quark.cn/s/xxx",
      "password": ""
    }
  ]
}
```

响应:
```json
{
  "results": [
    {
      "disk_type": "quark",
      "url": "https://pan.quark.cn/s/xxx",
      "normalized_url": "...",
      "state": "valid",
      "cache_hit": false,
      "checked_at": 1733000000,
      "expires_at": 1733003600,
      "summary": "链接有效"
    }
  ]
}
```

**state 字段映射到卡片状态**:
| state | 卡片表现 |
|---|---|
| `valid` | 绿色边框 + ✓ |
| `invalid` / `expired` | 红色边框 + ✗ + 链接划线 |
| `error` | 灰色 + ⚠️（检测失败，非链接问题） |

### 5.3 健康检查接口

**`GET /api/health`**

返回服务状态、插件列表、频道列表。前端可用于启动时验证后端连通性。

---

## 6. 错误处理策略

| 场景 | 处理方式 |
|---|---|
| 网络错误/超时 | 全屏错误状态 + 重试按钮 |
| API 返回 `code != 0` | el-message 错误提示 + 显示 message |
| 搜索无结果 | 空状态：建议更换关键词 |
| 链接检测失败 | 单卡片 toast 提示，不影响其他卡片 |
| 请求被取消（切 tab） | 静默处理，不弹错误 |

**Axios 拦截器**:
```javascript
service.interceptors.response.use(
  res => res.data,
  err => {
    if (err.code === 'ERR_CANCELED') return Promise.reject(err)
    ElMessage.error(err.message || '网络错误')
    return Promise.reject(err)
  }
)
```

---

## 7. 部署方案

### 7.1 开发模式

```javascript
// vite.config.js
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8888',
      changeOrigin: true
    }
  }
}
```
访问 `http://localhost:5173`，跨域由 Vite 代理解决。

### 7.2 生产模式（两种选择）

**方案 A: 独立静态部署（推荐）**
```bash
npm run build
# 把 dist/ 目录扔到任何静态服务器
# nginx 配置示例:
location /api {
  proxy_pass http://pansou:8888;
}
location / {
  root /usr/share/nginx/html;
  try_files $uri $uri/ /index.html;
}
```

**方案 B: Docker 容器化部署**
```dockerfile
# Dockerfile
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

提供 `docker-compose.yml` 一键起前端 + 后端:
```yaml
services:
  pansou-web:
    build: .
    ports: ["80:80"]
    depends_on: [pansou]
```

### 7.3 环境变量

```env
# .env.development
VITE_API_BASE=   # 空值走 vite proxy

# .env.production
VITE_API_BASE=   # 空值同源部署，或填完整 URL
```

### 7.4 关键配置项

- **API base URL**: 通过环境变量 `VITE_API_BASE` 配置
- **每页加载**: 默认渲染前 50 条，滚动加载更多
- **历史记录上限**: 20 条
- **检测请求并发**: 3 个（避免压垮后端）

---

## 8. 测试策略

由于项目以 UI 交互为主，测试以**手动验证清单**为主：

### 8.1 功能验证清单
- [ ] 输入关键词回车，能正确返回搜索结果
- [ ] Tab 切换能正确筛选网盘类型
- [ ] 关键词在标题/内容中正确高亮
- [ ] 复制按钮能正确复制链接到剪贴板
- [ ] 搜索历史能保存、显示、点击重搜、清空
- [ ] 链接检测按钮点击后能显示 loading → 结果
- [ ] 检测有效/失效/错误状态显示正确
- [ ] 响应式布局：手机单列、平板双列、桌面三列
- [ ] 骨架屏在搜索 loading 时正确显示
- [ ] 错误状态显示重试按钮

### 8.2 兼容性验证
- [ ] Chrome/Edge 最新版
- [ ] Firefox 最新版
- [ ] Safari（如有）
- [ ] 移动端浏览器（Chrome iOS/Safari iOS）

---

## 9. 实现顺序建议

1. 项目脚手架搭建（Vite + Vue3 + Element Plus）
2. 设计系统注入（CSS 变量、字体引入）
3. API 层封装（axios + 拦截器 + 接口封装）
4. 核心 SearchBar + 搜索逻辑
5. ResultCard + ResultTabs 结果展示
6. 搜索历史 composable
7. 链接检测 CheckButton
8. 空状态 + 骨架屏 + 错误处理
9. 响应式布局调整
10. 构建配置 + Dockerfile + 部署文档

---

**设计文档完成。** 下一步将进入实现规划阶段（writing-plans）。
