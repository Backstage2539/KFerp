export const customerWorkspacePages = [
  ['customerPriceTables', '我的价格表', 'bean_list'],
  ['customerProductOrder', '现货录单', 'product_order'],
  ['customerDirectShip', '代发录单', 'direct_ship'],
  ['customerProcessing', '代加工', 'processing'],
  ['customerInventory', '我的库存', 'inventory_custody'],
  ['customerSettlement', '结算中心', 'settlement'],
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
  if (codes.includes('settlement'))
    groups.push({
      id: 'customerFinance',
      name: '费用相关',
      items: [
        { key: 'financeExpenses', label: '费用明细' },
        { key: 'financeReport', label: '经营报告' },
        { key: 'financeClosing', label: '结账相关' }
      ]
    })
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
