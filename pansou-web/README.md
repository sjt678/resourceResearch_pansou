# PanSou Web

网盘资源聚合搜索前端，对接 [pansou](../pansou) 后端 API。

## 技术栈

- Vue 3 + Vite 5 + Element Plus
- Pinia（状态） + Vue Router（路由） + Axios（HTTP）
- @vueuse/core（工具）

## 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 启动开发服务器

```bash
npm run dev
```

默认访问 http://localhost:5173 ，`/api` 请求会被 Vite 代理到 `http://localhost:8888`（后端）。

> 开发模式下默认走 vite proxy，`.env.development` 里 `VITE_API_BASE` 留空即可。如需指向其他后端，改成完整地址。

### 3. 构建生产包

```bash
npm run build
```

产物在 `dist/`，可直接扔到任意静态服务器。

## 环境变量

| 变量 | 说明 |
|---|---|
| `VITE_API_BASE` | 后端 API 基址。留空 = 开发走 vite proxy / 生产同源部署；填完整 URL = 跨域直连后端 |

## 后端 API 对接

| 接口 | 方法 | 用途 |
|---|---|---|
| `/api/search` | GET | 搜索资源，返回按网盘类型分组的结果 |
| `/api/check/links` | POST | 检测网盘链接有效性 |
| `/api/health` | GET | 健康检查，返回频道数、插件数、是否启用认证 |

### 搜索响应结构

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 247,
    "merged_by_type": {
      "quark": [{ "url": "...", "password": "", "note": "...", "datetime": "...", "source": "tg:xxx" }]
    }
  }
}
```

前端会把 `merged_by_type` 拍平成统一数组，每条附带 `cloudType` 字段，用于 Tab 筛选与卡片展示。

## 功能

- 关键词搜索（回车触发，URL 同步 `?q=xxx`，刷新保留）
- 网盘类型 Tab 切换（夸克/百度/阿里/115/微云/其他）
- 关键词高亮
- 一键复制链接（含提取码）
- 链接有效性检测（valid / invalid / expired / error 状态）
- 搜索历史（localStorage，最近 20 条）
- 骨架屏、空状态、错误重试
- 响应式布局（手机单列 / 平板双列 / 桌面三列）

## 部署

### 方案 A：独立静态部署（推荐）

```bash
npm run build
# 把 dist/ 放到 nginx，并反代 /api 到后端
```

nginx 配置参考 `nginx.conf`。

### 方案 B：Docker

```bash
docker compose up -d --build
```

`docker-compose.yml` 同时拉起前端（nginx）与后端，前端 80 端口，后端 8888 端口。

## 目录结构

```
src/
├── api/            # axios 实例 + 接口封装
├── components/     # 组件（SearchBar/ResultTabs/ResultCard/CheckButton/EmptyState/SkeletonCard）
├── composables/    # useHistory / useSearch
├── stores/         # Pinia store
├── styles/         # CSS 变量 + 全局样式
├── utils/          # 网盘类型映射、时间格式化、关键词高亮
├── views/          # Home.vue 主页面
├── App.vue
├── main.js
└── router.js
```
