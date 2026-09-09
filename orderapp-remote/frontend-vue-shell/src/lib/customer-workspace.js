export const customerWorkspacePages = [
  ['customerPriceTables', '我的价格表', 'bean_list'],
  ['customerProductOrder', '现货录单', 'product_order'],
  ['customerDirectShip', '代发录单', 'direct_ship'],
  ['customerProcessing', '代加工', 'processing'],
  ['customerInventory', '我的库存', 'inventory_custody'],
  ['customerOrderFees', '订单费用', 'settlement'],
  ['customerSettlement', '往来账单', 'settlement'],
  ['customerMall', '商城', 'mall']
]
export function capabilityCodes(values = []) {
  return values.map((v) => (typeof v === 'string' ? v : v.enabled ? v.code : '')).filter(Boolean)
}
export function customerWorkspaceCapability(key) {
  return customerWorkspacePages.find((p) => p[0] === key)?.[2] || ''
}
export function customerWorkspaceMenu(capabilities = []) {
  const codes = capabilityCodes(capabilities)
  const items = [{ key: 'customerProcessingPortal', label: '首页', title: '客户首页' }]
  if (codes.some((c) => ['direct_ship', 'product_order', 'processing', 'mall'].includes(c)))
    items.push({ key: 'customerOrders', label: '我的订单', title: '我的订单' })
  for (const [key, label, code] of customerWorkspacePages)
    if (codes.includes(code)) items.push({ key, label, title: label })
  const groups = [{ id: 'customerWorkbench', name: '客户中心', items }]
  return groups
}
export function resetCustomerContinuation(form) {
  for (const key of [
    'receiver_name',
    'receiver_phone',
    'receiver_address',
    'receiver_company',
    'notes',
    'ship_tracking_no',
    'discount_amount'
  ])
    form[key] = ''
}
