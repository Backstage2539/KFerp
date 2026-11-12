const FIELDS = ['revenue', 'cost_expenses', 'profit']
const chartValue = (day, key) => day.cost_unavailable && key !== 'revenue' ? null : day[key]

export function buildDailyChart(days) {
  const values = days.flatMap(day => FIELDS.map(key => chartValue(day, key)).filter(value => value !== null && value !== undefined && Number.isFinite(Number(value))).map(Number))
  const min = Math.min(0, ...values), max = Math.max(0, ...values)
  const range = max - min || 1
  const x = index => days.length < 2 ? 460 : 56 + index * 820 / (days.length - 1)
  const y = value => 16 + (max - Number(value)) * 184 / range
  const series = Object.fromEntries(FIELDS.map(key => {
    const segments = []
    let segment = []
    days.forEach((day, index) => {
      const value = chartValue(day, key)
      if (value === null || value === undefined || !Number.isFinite(Number(value))) {
        if (segment.length) segments.push(segment)
        segment = []
      } else segment.push({ x: x(index), y: y(value), date: day.date, value: Number(value) })
    })
    if (segment.length) segments.push(segment)
    return [key, segments]
  }))
  return { series, min, max, zeroY: y(0), positions: days.map((day, index) => ({ date: day.date, x: x(index) })) }
}

export function ledgerRoute(link, returnParams = null, row = null) {
  if (!link || !String(link).startsWith('/')) return null
  const url = new URL(link, 'https://finance.local')
  if (url.origin !== 'https://finance.local' || !['/app/vue-shell', '/vue-shell'].includes(url.pathname)) return null
  let key = url.searchParams.get('view')
  if (!key) return null
  const params = Object.fromEntries(url.searchParams)
  delete params.view
  const returnNavigation = { key: 'financeDashboard', label: '返回财务首页' }
  if (returnParams?.month && /^\d{4}-(0[1-9]|1[0-2])$/.test(returnParams.month)) {
    returnNavigation.params = { month: returnParams.month }
    if (returnParams.date?.startsWith(`${returnParams.month}-`) && /^\d{4}-\d{2}-\d{2}$/.test(returnParams.date)) returnNavigation.params.date = returnParams.date
  }
  if (key === 'financeExpenses' && returnParams?.month) {
    params.month = returnParams.month
    if (row?.source_type === 'expense' && Number.isSafeInteger(Number(row.source_id)) && Number(row.source_id) > 0) params.expense_id = String(row.source_id)
  }
  if (key === 'customerAccount') {
    const customerID = Number(params.customer_id)
    if (!Number.isSafeInteger(customerID) || customerID <= 0) return null
    key = 'customerFulfillment'
    params.tab = 'fees'
    returnNavigation.viewContext = { type: 'factory' }
    return { key, params, customerContext: { customerID }, returnNavigation }
  }
  return { key, params, returnNavigation }
}

export function shanghaiToday() {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date())
}
