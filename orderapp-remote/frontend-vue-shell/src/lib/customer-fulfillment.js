export function importTypeOptions() {
  return [
    { value: 'processing_workbook', label: '代加工工单' },
    { value: 'direct_ship_workbook', label: '代发清单' },
    { value: 'settlement_workbook', label: '结算单' },
  ]
}

export function importSummaryCards(summary = {}) {
  const cards = []
  addPositiveCard(cards, '有效行', summary.valid_rows)
  addPositiveCard(cards, '错误行', summary.invalid_rows)
  addPositiveCard(cards, '代发订单', summary.direct_ship_orders)
  addPositiveCard(cards, '加工工单', summary.processing_orders)
  addPositiveCard(cards, '费用明细', summary.fee_items)
  return cards
}

export function rowStatusLabel(status) {
  return {
    valid: '有效',
    invalid: '错误',
    applied: '已应用',
    parsed: '已解析',
  }[status] || status || '-'
}

function addPositiveCard(cards, label, value) {
  const n = Number(value || 0)
  if (n > 0) cards.push({ label, value: n })
}
