// 网盘类型元数据：品牌色、图标字符、中文名
// cloud_type 取自后端 merged_by_type 的 key（后端 link.Type 值）
export const CLOUD_META = {
  quark: { label: '夸克', color: '#3b82f6', icon: 'Q' },
  baidu: { label: '百度', color: '#06b6d4', icon: '百' },
  aliyun: { label: '阿里', color: '#f97316', icon: '阿' },
  aliyundrive: { label: '阿里', color: '#f97316', icon: '阿' },
  115: { label: '115', color: '#84cc16', icon: '115' },
  weiyun: { label: '微云', color: '#8b5cf6', icon: '微' },
  uc: { label: 'UC', color: '#f97316', icon: 'UC' },
  xunlei: { label: '迅雷', color: '#0ea5e9', icon: '迅' },
  tianyi: { label: '天翼', color: '#0ea5e9', icon: '天' },
  123: { label: '123', color: '#10b981', icon: '123' },
  pikpak: { label: 'PikPak', color: '#8b5cf6', icon: 'P' },
  guangya: { label: '光雅', color: '#f43f5e', icon: '光' },
  mobile: { label: '移动', color: '#0ea5e9', icon: '移' },
  magnet: { label: '磁力', color: '#ef4444', icon: '磁' },
  ed2k: { label: '电驴', color: '#a855f7', icon: '驴' },
  others: { label: '其他', color: '#64748b', icon: '链' }
}

const DEFAULT_META = { label: '网盘', color: '#64748b', icon: '链' }

export function getCloudMeta(cloudType) {
  if (!cloudType) return DEFAULT_META
  const key = String(cloudType).toLowerCase()
  return CLOUD_META[key] || { ...DEFAULT_META, label: cloudType, icon: cloudType.slice(0, 2) }
}

// 排序权重：已知类型优先，其余按名称
const ORDER = [
  'quark', 'baidu', 'aliyun', 'aliyundrive', '115', 'uc', 'weiyun',
  'xunlei', 'tianyi', '123', 'pikpak', 'guangya', 'mobile', 'magnet', 'ed2k', 'others'
]

export function sortCloudTypes(types) {
  return [...types].sort((a, b) => {
    const ia = ORDER.indexOf(a)
    const ib = ORDER.indexOf(b)
    if (ia !== -1 && ib !== -1) return ia - ib
    if (ia !== -1) return -1
    if (ib !== -1) return 1
    return a.localeCompare(b)
  })
}
