import service from './index.js'
import { sortCloudTypes } from '@/utils/cloud.js'

/**
 * 搜索网盘资源
 * @param {Object} params
 * @param {string} params.kw 搜索关键词（必填）
 * @param {string} [params.src] 数据来源：all / tg / plugin，默认 all
 * @param {string} [params.res] 返回类型：merge（按网盘类型分组），默认 merge
 * @param {string} [params.cloud_types] 逗号分隔的网盘类型
 * @param {Array<string>} [params.plugins] 指定插件列表
 * @param {boolean} [params.refresh] 强制刷新缓存
 * @param {AbortSignal} [signal] 用于取消请求
 * @returns {Promise<{total:number, merged_by_type:Object}>}
 */
export function search(params, signal) {
  return service.get('/api/search', {
    params: {
      kw: params.kw,
      src: params.src || 'all',
      res: params.res || 'merge',
      ...(params.cloud_types ? { cloud_types: params.cloud_types } : {}),
      ...(params.plugins && params.plugins.length ? { plugins: params.plugins.join(',') } : {}),
      ...(params.refresh ? { refresh: 'true' } : {})
    },
    signal
  })
}

/**
 * 健康检查，获取后端状态、插件列表、频道数、是否启用认证
 * @returns {Promise<Object>}
 */
export function health() {
  return service.get('/api/health')
}

/**
 * 将后端 merged_by_type 拍平为统一数组，每条附带 cloudType 字段
 * 保持后端返回的综合排序顺序（不再按时间重排，避免覆盖后端的智能排序）
 * 类型顺序按 cloud.js 中定义的优先级排列
 * @param {Object} mergedByType
 * @returns {Array<{url:string,password:string,note:string,datetime:string,source:string,images:Array,cloudType:string}>}
 */
export function flattenResults(mergedByType) {
  if (!mergedByType || typeof mergedByType !== 'object') return []
  // 按预定义的类型优先级排序，保持每种类型内部的后端综合排序顺序
  const types = sortCloudTypes(Object.keys(mergedByType))
  const list = []
  for (const cloudType of types) {
    const items = mergedByType[cloudType] || []
    for (const item of items) {
      list.push({ ...item, cloudType })
    }
  }
  return list
}
