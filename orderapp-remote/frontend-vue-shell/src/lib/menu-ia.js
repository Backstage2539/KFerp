export const menuGroups = [
  {
    id: 'sales',
    name: '订单销售',
    items: [
      { key: 'order', label: '录单', title: '录单' },
      { key: 'orders', label: '订单列表', title: '订单列表' },
      { key: 'customers', label: '客户档案', title: '客户档案' },
      { key: 'quotePrint', label: '报价导出', title: '报价导出' },
    ],
  },
  {
    id: 'production',
    name: '生产管理',
    items: [
      { key: 'productionManual', label: '生产流程', title: '生产流程' },
      { key: 'productionAcceptance', label: '生产验收', title: '生产验收' },
      { key: 'producePlan', label: '生产计划/开始生产', title: '生产计划/开始生产' },
      { key: 'produceRunning', label: '生产中', title: '生产中' },
      { key: 'workOrders', label: '生产工单', title: '生产工单' },
      { key: 'jobCards', label: '工序卡', title: '工序卡' },
      { key: 'qualityInspections', label: '生产质检', title: '生产质检' },
      { key: 'produceLogs', label: '生产日志', title: '生产日志' },
      { key: 'productionCosts', label: '生产成本', title: '生产成本' },
    ],
  },
  {
    id: 'inventory',
    name: '库存管理',
    items: [
      { key: 'warehouseInventory', label: '仓库库存', title: '仓库库存' },
      { key: 'stockOperations', label: '库存作业', title: '库存作业' },
      { key: 'stockOutboundLogs', label: '出库日志', title: '出库日志' },
      { key: 'purchase', label: '采购入库', title: '采购入库' },
      { key: 'materials', label: '物料档案', title: '物料档案' },
    ],
  },
  {
    id: 'product',
    name: '商品与配方',
    items: [
      { key: 'productSettings', label: '产品设置', title: '产品设置' },
      { key: 'mallSettings', label: '商城管理', title: '商城管理' },
      { key: 'costing', label: '价格与豆单', title: '价格与豆单' },
      { key: 'bom', label: 'BOM配方维护', title: 'BOM配方维护' },
    ],
  },
  {
    id: 'finance',
    name: '财务管理',
    items: [
      { key: 'financeDashboard', label: '财务首页', title: '财务首页' },
      { key: 'financeExpenses', label: '费用管理', title: '费用管理' },
      { key: 'financeClosing', label: '月度结账', title: '月度结账' },
      { key: 'financeReport', label: '经营报告', title: '月度经营报告' },
      { key: 'financeTaxLedger', label: '票税台账', title: '票税台账' },
      { key: 'financeSettings', label: '财务设置', title: '财务设置' },
      { key: 'financeManual', label: '财务手册', title: '财务月结手册' },
    ],
  },
  {
    id: 'settings',
    name: '设置',
    items: [
      { key: 'costingSettings', label: '成本参数设置', title: '成本参数设置' },
      { key: 'machines', label: '设备产能配置', title: '设备产能配置' },
      { key: 'companyProfile', label: '公司设置', title: '公司设置' },
      { key: 'salesOrderSettings', label: '销售单设置', title: '销售单设置' },
      { key: 'senderSettings', label: '发货人设置', title: '发货人设置' },
      { key: 'outsourceSettings', label: '代加工模板设置', title: '代加工模板设置' },
      { key: 'customerPortalSettings', label: '客户门户配置', title: '客户门户配置' },
      { key: 'customerFulfillment', label: '客户履约账户', title: '客户履约账户' },
    ],
  },
  {
    id: 'system',
    name: '系统',
    items: [
      { key: 'departments', label: '部门维护', title: '部门维护' },
      { key: 'employees', label: '员工维护', title: '员工维护' },
      { key: 'userPermissions', label: '用户权限', title: '用户权限' },
      { key: 'audit', label: '操作日志', title: '操作日志' },
    ],
  },
  {
    id: 'requirements',
    name: '需求管理',
    items: [
      { key: 'reqProduct', label: '产品需求表', title: '产品需求表' },
      { key: 'reqDev', label: '开发需求表', title: '开发需求表' },
      { key: 'reqUnit', label: '单元测试表', title: '单元测试表' },
      { key: 'reqApi', label: 'API 测试表', title: 'API 测试表' },
      { key: 'reqReview', label: '需求审核表', title: '需求审核表' },
    ],
  },
]

export const hiddenViewTitles = {
  deliveryNote: '出库单',
  orderInvoice: '发票',
  materialReceipts: '原料入库',
  materialBatches: '原料批次',
  wipMaterials: 'WIP在制仓',
  stockLedger: '库存流水',
  stockBatches: '批次追溯',
  stockAdjustments: '库存调整单',
  stockOutboundLogs: '出库日志',
  inventory: '成品库存',
  allocationLogs: '分配批次查看',
  products: '产品设置',
  salesOrder: '销售单',
}

export const menuMap = Object.fromEntries([
  ...menuGroups.flatMap((group) => group.items.map((item) => [item.key, { title: item.title }])),
  ...Object.entries(hiddenViewTitles).map(([key, title]) => [key, { title }]),
])

export function primaryMenuKeys(groups = menuGroups) {
  return groups.flatMap((group) => group.items.map((item) => item.key))
}

export function groupForView(groups, key) {
  return groups.find((group) => group.items.some((item) => item.key === key)) || null
}

export function defaultExpandedGroups(groups = menuGroups, currentKey = '') {
  const ids = groups.slice(0, 3).map((group) => group.id)
  const current = groupForView(groups, currentKey)
  if (current && !ids.includes(current.id)) ids.push(current.id)
  return ids
}

export function normalizeExpandedGroups(groups, values, currentKey = '') {
  const valid = new Set(groups.map((group) => group.id))
  const out = []
  for (const value of values || []) {
    if (valid.has(value) && !out.includes(value)) out.push(value)
  }
  const current = groupForView(groups, currentKey)
  if (current && !out.includes(current.id)) out.push(current.id)
  return out.length ? out : defaultExpandedGroups(groups, currentKey)
}

export function restoreExpandedGroups(groups, raw, currentKey = '') {
  try {
    const parsed = JSON.parse(raw || '[]')
    return normalizeExpandedGroups(groups, Array.isArray(parsed) ? parsed : [], currentKey)
  } catch {
    return defaultExpandedGroups(groups, currentKey)
  }
}

export function toggleExpandedGroup(values, id) {
  if ((values || []).includes(id)) {
    return values.filter((value) => value !== id)
  }
  return [...(values || []), id]
}
