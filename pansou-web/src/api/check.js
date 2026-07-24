import service from './index.js'

/**
 * 检测网盘链接有效性（支持批量）
 * @param {Array<{disk_type:string,url:string,password?:string}>} items
 * @param {AbortSignal} [signal]
 * @returns {Promise<{results:Array}>}
 */
export function checkLinks(items, signal) {
  return service.post('/api/check/links', { items }, { signal })
}
