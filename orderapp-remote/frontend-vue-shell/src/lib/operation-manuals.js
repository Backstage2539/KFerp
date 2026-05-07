export const operationManualsByView = {
  orderSalesManual: { doc: 'OP_MANUAL_ORDER_SALES.md', title: '订单销售手册' },
  productionManual: { doc: 'OP_MANUAL_PRODUCTION.md', title: '生产手册' },
  inventoryMaterialsManual: { doc: 'OP_MANUAL_INVENTORY_MATERIALS.md', title: '库存物料手册' },
  costingManual: { doc: 'OP_MANUAL_COSTING.md', title: '成本核价手册' },
  financeManual: { doc: 'OP_MANUAL_FINANCE.md', title: '财务手册' },
  settingsAuditManual: { doc: 'OP_MANUAL_SETTINGS_AUDIT.md', title: '设置审计手册' },
  customerFulfillmentManual: { doc: 'OP_MANUAL_CUSTOMER_FULFILLMENT.md', title: '客户履约手册' },
  requirementsManual: { doc: 'OP_MANUAL_REQUIREMENTS.md', title: '需求管理手册' },
}

export function manualDocNameForView(viewKey) {
  return operationManualsByView[viewKey]?.doc || ''
}

export function manualTitleForView(viewKey, fallback = '') {
  return operationManualsByView[viewKey]?.title || fallback
}

export function parseManualMarkdown(markdown) {
  const lines = String(markdown || '').replace(/\r\n/g, '\n').split('\n')
  const blocks = []
  let index = 0

  while (index < lines.length) {
    const line = lines[index].trim()
    if (!line) {
      index += 1
      continue
    }

    if (line.startsWith('# ')) {
      blocks.push({ type: 'h1', text: line.slice(2).trim() })
      index += 1
      continue
    }
    if (line.startsWith('## ')) {
      blocks.push({ type: 'h2', text: line.slice(3).trim() })
      index += 1
      continue
    }
    if (line.startsWith('### ')) {
      blocks.push({ type: 'h3', text: line.slice(4).trim() })
      index += 1
      continue
    }
    if (line.startsWith('>')) {
      blocks.push({ type: 'quote', text: line.replace(/^>\s?/, '').trim() })
      index += 1
      continue
    }
    if (isTableStart(lines, index)) {
      const parsed = parseTable(lines, index)
      blocks.push(parsed.block)
      index = parsed.nextIndex
      continue
    }
    if (line.startsWith('- ')) {
      const items = []
      while (index < lines.length && lines[index].trim().startsWith('- ')) {
        items.push(lines[index].trim().slice(2).trim())
        index += 1
      }
      blocks.push({ type: 'ul', items })
      continue
    }
    if (/^\d+\.\s+/.test(line)) {
      const items = []
      while (index < lines.length && /^\d+\.\s+/.test(lines[index].trim())) {
        items.push(lines[index].trim().replace(/^\d+\.\s+/, '').trim())
        index += 1
      }
      blocks.push({ type: 'ol', items })
      continue
    }

    const paragraph = [line]
    index += 1
    while (
      index < lines.length &&
      lines[index].trim() &&
      !isSpecialLine(lines[index].trim()) &&
      !isTableStart(lines, index)
    ) {
      paragraph.push(lines[index].trim())
      index += 1
    }
    blocks.push({ type: 'p', text: paragraph.join(' ') })
  }

  return blocks
}

function isSpecialLine(line) {
  return line.startsWith('# ') ||
    line.startsWith('## ') ||
    line.startsWith('### ') ||
    line.startsWith('>') ||
    line.startsWith('- ') ||
    /^\d+\.\s+/.test(line)
}

function isTableStart(lines, index) {
  const header = lines[index]?.trim() || ''
  const separator = lines[index + 1]?.trim() || ''
  return header.startsWith('|') && header.endsWith('|') && /^\|[\s:|-]+\|$/.test(separator)
}

function parseTable(lines, index) {
  const headers = splitTableRow(lines[index])
  index += 2
  const rows = []
  while (index < lines.length) {
    const line = lines[index].trim()
    if (!line.startsWith('|') || !line.endsWith('|')) break
    rows.push(splitTableRow(line))
    index += 1
  }
  return { block: { type: 'table', headers, rows }, nextIndex: index }
}

function splitTableRow(line) {
  return line.split('|').slice(1, -1).map((cell) => cell.trim())
}
