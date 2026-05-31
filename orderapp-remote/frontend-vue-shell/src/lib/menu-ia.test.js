import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  defaultExpandedGroups,
  groupForView,
  menuMap,
  menuGroups,
  primaryMenuKeys,
  restoreExpandedGroups,
  toggleExpandedGroup,
} from './menu-ia.js'

test('primary menu replaces overlapping inventory pages with warehouse inventory workspaces', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('warehouseInventory'))
  assert.ok(keys.includes('stockOperations'))
  assert.ok(keys.includes('stockOutboundLogs'))
  assert.ok(keys.includes('materials'))
  assert.equal(keys.includes('materialBatches'), false)
  assert.equal(keys.includes('stockBatches'), false)
  assert.equal(keys.includes('stockLedger'), false)
  assert.equal(keys.includes('inventory'), false)
})

test('warehouse inventory and legacy stock views resolve to the inventory group', () => {
  assert.equal(groupForView(menuGroups, 'warehouseInventory')?.id, 'inventory')
  assert.equal(groupForView(menuGroups, 'stockOperations')?.id, 'inventory')
  assert.equal(groupForView(menuGroups, 'stockOutboundLogs')?.id, 'inventory')
})

test('expanded menu groups persist and keep current group open', () => {
  const initial = defaultExpandedGroups(menuGroups, 'warehouseInventory')
  assert.ok(initial.includes('inventory'))

  const closed = toggleExpandedGroup(initial, 'inventory')
  assert.equal(closed.includes('inventory'), false)

  const restored = restoreExpandedGroups(menuGroups, JSON.stringify(['sales']), 'warehouseInventory')
  assert.deepEqual(restored, ['sales', 'inventory'])
})

test('product menu splits product archive, customer names and config templates as peer pages', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('productMaster'))
  assert.ok(keys.includes('customerProductAliases'))
  assert.ok(keys.includes('productConfigTemplates'))
  assert.equal(keys.includes('productSettings'), false)
  assert.ok(keys.includes('costing'))
  assert.equal(keys.includes('products'), false)
  assert.equal(groupForView(menuGroups, 'productMaster')?.id, 'product')
  assert.equal(groupForView(menuGroups, 'customerProductAliases')?.id, 'product')
  assert.equal(groupForView(menuGroups, 'productConfigTemplates')?.id, 'product')
  assert.equal(groupForView(menuGroups, 'costing')?.id, 'product')
  assert.equal(menuGroups.find((group) => group.id === 'product')?.items.map((item) => item.label).join(' / '), '商品档案 / 客户商品名 / 商品配置模板 / 生产 BOM / 产品价格表 / 行业字段模板 / 成本核价手册 / 生豆销售手册')
})

test('production menu exposes the production flow manual as a primary page', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('productionManual'))
  assert.equal(groupForView(menuGroups, 'productionManual')?.id, 'production')
})

test('process templates live in the production menu for discoverability', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('processTemplates'))
  assert.equal(groupForView(menuGroups, 'processTemplates')?.id, 'production')
  assert.equal(groupForView(menuGroups, 'industryFieldTemplates')?.id, 'product')
})

test('operation manuals live inside their functional menu groups', () => {
  const expectations = [
    ['orderSalesManual', 'sales'],
    ['productionManual', 'production'],
    ['inventoryMaterialsManual', 'inventory'],
    ['costingManual', 'product'],
    ['greenBeanSalesManual', 'product'],
    ['settingsAuditManual', 'settings'],
    ['notificationManual', 'settings'],
    ['customerFulfillmentManual', 'customerManagement'],
    ['requirementsManual', 'requirements'],
  ]
  const keys = primaryMenuKeys(menuGroups)
  for (const [key, groupID] of expectations) {
    assert.ok(keys.includes(key), `${key} should be a primary menu item`)
    assert.equal(groupForView(menuGroups, key)?.id, groupID)
  }
  assert.equal(menuGroups.some((group) => /手册|文档/.test(group.name)), false)
})

test('settings menu exposes sales order settings and keeps sales order detail hidden', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('companyProfile'))
  assert.ok(keys.includes('salesOrderSettings'))
  assert.ok(keys.includes('logisticsSettings'))
  assert.equal(keys.includes('salesOrder'), false)
  assert.equal(groupForView(menuGroups, 'companyProfile')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'salesOrderSettings')?.id, 'settings')
  assert.equal(groupForView(menuGroups, 'logisticsSettings')?.id, 'settings')
})

test('sales menu no longer exposes the removed quote export page', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.equal(keys.includes('quotePrint'), false)
  assert.equal(groupForView(menuGroups, 'quotePrint'), null)
  assert.equal(menuMap.quotePrint, undefined)
  assert.equal(JSON.stringify(menuGroups).includes('报价导出'), false)
})

test('customer management menu consolidates customer dossier, portal settings, templates and manual', () => {
  const keys = primaryMenuKeys(menuGroups)
  for (const key of ['customers', 'customerPortalSettings', 'customerCapabilityTemplates', 'customerFulfillmentManual']) {
    assert.ok(keys.includes(key))
    assert.equal(groupForView(menuGroups, key)?.id, 'customerManagement')
  }
  assert.equal(groupForView(menuGroups, 'customers')?.id, 'customerManagement')
  assert.notEqual(groupForView(menuGroups, 'customers')?.id, 'sales')
  assert.equal(menuGroups.find((group) => group.id === 'customerManagement')?.name, '客户管理')
  assert.equal(menuGroups.find((group) => group.id === 'customerManagement')?.items.find((item) => item.key === 'customerCapabilityTemplates')?.label, '客户门户能力模板')
  assert.equal(keys.includes('workspaceModeManual'), false)
  assert.equal(keys.includes('customerPortalManual'), false)
  assert.equal(menuMap.workspaceModeManual?.title, '客户履约手册')
  assert.equal(menuMap.customerPortalManual?.title, '客户履约手册')
  assert.equal(keys.includes('customerProcessingPortal'), false)
  assert.equal(groupForView(menuGroups, 'customerProcessingPortal'), null)
})

test('system menu merges user permissions into employee maintenance', () => {
  const keys = primaryMenuKeys(menuGroups)
  assert.ok(keys.includes('employees'))
  assert.equal(keys.includes('userPermissions'), false)
  assert.equal(groupForView(menuGroups, 'employees')?.id, 'system')
  assert.equal(groupForView(menuGroups, 'userPermissions'), null)
  assert.equal(JSON.stringify(menuGroups).includes('用户权限'), false)
})

test('finance menu exposes monthly finance workflows as primary pages', () => {
  const keys = primaryMenuKeys(menuGroups)
  for (const key of ['financeDashboard', 'financeExpenses', 'financeClosing', 'financeReport', 'financeSettings', 'financeManual']) {
    assert.ok(keys.includes(key))
    assert.equal(groupForView(menuGroups, key)?.id, 'finance')
  }
})

test('remaining ERP click-matrix targets reference real Vue shell views', () => {
  const remainingTargets = [
    'workOrders',
    'jobCards',
    'qualityInspections',
    'produceLogs',
    'productionCosts',
    'stockOperations',
    'stockOutboundLogs',
    'purchase',
    'materials',
    'productMaster',
    'customerProductAliases',
    'productConfigTemplates',
    'costing',
    'bom',
    'order',
    'customers',
    'salesOrderSettings',
    'logisticsSettings',
    'senderSettings',
    'orderInvoice',
    'salesOrder',
    'deliveryNote',
    'financeSettings',
    'customerPortalSettings',
    'customerCapabilityTemplates',
    'companyProfile',
    'machines',
    'employees',
    'departments',
    'audit',
  ]

  for (const key of remainingTargets) {
    assert.ok(menuMap[key], `${key} should resolve to a Vue shell view`)
  }
})
