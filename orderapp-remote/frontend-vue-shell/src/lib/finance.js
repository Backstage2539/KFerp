export function currentMonth(date = new Date()) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  return `${year}-${month}`
}

export function monthFromDate(value) {
  const text = String(value || '').trim()
  return /^\d{4}-\d{2}-\d{2}$/.test(text) ? text.slice(0, 7) : ''
}

export function money(value) {
  const n = Number(value || 0)
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

export function percent(value) {
  const n = Number(value || 0) * 100
  return `${n.toFixed(2)}%`
}

export function rateToPercent(value) {
  const n = Number(value || 0) * 100
  return Number(n.toFixed(4)).toString()
}

export function rateFromPercent(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? Number((n / 100).toFixed(6)) : 0
}

export function financeStatusLabel(status) {
  const labels = {
    draft: '未结账',
    closed: '已结账',
    adjusted: '已调整',
  }
  return labels[status] || '未结账'
}

export function companyTypeLabel(value) {
  const labels = {
    coffee_roaster: '咖啡烘焙厂',
    coffee_trader: '咖啡贸易商',
    coffee_processor: '咖啡壳豆加工厂',
    combined: '综合咖啡工厂',
  }
  return labels[value] || value || ''
}

export function taxpayerTypeLabel(value) {
  const labels = {
    small_scale: '小规模纳税人',
    general: '一般纳税人',
  }
  return labels[value] || value || ''
}

export function closingModeLabel(value) {
  const labels = {
    strong_lock: '强锁账',
    light_confirmation: '轻确认',
  }
  return labels[value] || value || ''
}

export function financeReportExportUrls(month) {
  const safeMonth = encodeURIComponent(month || currentMonth())
  return {
    pdf: `/api/finance/reports/${safeMonth}/pdf`,
    excel: `/api/finance/reports/${safeMonth}/excel`,
  }
}

export function financeAccountantHandoffUrl(month) {
  const safeMonth = encodeURIComponent(month || currentMonth())
  return `/api/finance/reports/${safeMonth}/accountant-handoff.xlsx`
}

function numberOrNull(value) {
  if (value === undefined || value === null || value === '') return null
  const n = Number(value)
  return Number.isFinite(n) ? n : null
}

function valueAtPath(source, path) {
  return String(path || '').split('.').reduce((current, key) => current?.[key], source)
}

function pickReportValue(report, adjustedKey, baseKey) {
  const adjusted = numberOrNull(valueAtPath(report, adjustedKey))
  if (adjusted !== null) return adjusted
  return numberOrNull(valueAtPath(report, baseKey)) || 0
}

export function financeMetricCards(report = {}) {
  return [
    {
      label: '不含税收入',
      value: pickReportValue(report, 'adjusted_tax_exclusive_revenue', 'tax_exclusive_revenue'),
      tone: 'revenue',
    },
    {
      label: '毛利',
      value: pickReportValue(report, 'adjusted_gross_profit', 'gross_profit'),
      tone: 'profit',
      sub: percent(report.gross_margin),
    },
    {
      label: '净利',
      value: pickReportValue(report, 'adjusted_net_profit', 'operating_net_profit'),
      tone: 'net',
    },
    {
      label: '税费估算',
      value: pickReportValue(report, 'adjusted_tax_total', 'tax.total_tax'),
      tone: 'tax',
    },
  ].map((card) => ({
    ...card,
    display: money(card.value),
  }))
}

export function financeTaxRows(report = {}) {
  const tax = report.tax || {}
  return [
    ['销项税额', tax.output_vat],
    ['进项税额', tax.input_vat],
    ['不可抵扣进项', tax.non_deductible_input_vat],
    ['可抵扣进项', tax.deductible_input_vat],
    ['应交增值税', tax.vat_payable],
    ['附加税', tax.surtax],
    ['企业所得税计税所得', tax.cit_taxable_income],
    ['企业所得税估算', tax.cit_payable],
    ['小微优惠节税估算', tax.cit_preference_saving],
  ]
}
