import { clampPage, normalizePageSize, pageCount, PAGE_SIZE_OPTIONS } from './pagination'

const tableState = new WeakMap()

export function installTableAutoPagination(root = document.body) {
  if (!root || typeof MutationObserver === 'undefined') return () => {}
  let scheduled = false
  const refresh = () => {
    scheduled = false
    applyTableAutoPagination(root)
  }
  const schedule = () => {
    if (scheduled) return
    scheduled = true
    window.requestAnimationFrame(refresh)
  }
  const observer = new MutationObserver(schedule)
  observer.observe(root, { childList: true, subtree: true })
  schedule()
  return () => observer.disconnect()
}

export function applyTableAutoPagination(root = document.body) {
  const tables = Array.from(root.querySelectorAll('.table-wrap > table, table[data-auto-pagination="on"]'))
  for (const table of tables) {
    if (shouldSkipTable(table)) continue
    renderAutoPagination(table)
  }
}

function shouldSkipTable(table) {
  if (!table || table.dataset.autoPagination === 'off') return true
  const wrapper = table.closest('.table-wrap') || table
  const next = wrapper.nextElementSibling
  if (next?.classList?.contains('pager')) return true
  if (next?.classList?.contains('list-pagination-controls')) return true
  if (table.closest('[data-pagination-controls="server"]')) return true
  return false
}

function dataRows(table) {
  return Array.from(table.tBodies || [])
    .flatMap((tbody) => Array.from(tbody.rows || []))
    .filter((row) => row.dataset.paginationIgnore !== 'true')
    .filter((row) => !isEmptyStateRow(row))
}

function renderAutoPagination(table) {
  const rows = dataRows(table)
  if (!rows.length) {
    removeControls(table)
    return
  }
  let state = tableState.get(table)
  if (!state) {
    state = { page: 1, pageSize: 10 }
    tableState.set(table, state)
  }
  state.pageSize = normalizePageSize(state.pageSize)
  state.page = clampPage(state.page, rows.length, state.pageSize)
  const totalPages = pageCount(rows.length, state.pageSize)
  const start = (state.page - 1) * state.pageSize
  const end = start + state.pageSize
  rows.forEach((row, index) => {
    row.hidden = index < start || index >= end
  })
  const controls = ensureControls(table)
  controls.setAttribute('data-auto-pagination', 'true')
  controls.innerHTML = ''
  controls.append(
    summaryNode(rows.length, totalPages, state.page),
    actionsNode(table, state, rows.length, totalPages),
  )
}

function isEmptyStateRow(row) {
  const cells = Array.from(row.cells || [])
  if (cells.length !== 1) return false
  const colspan = Number.parseInt(cells[0].getAttribute('colspan') || '1', 10)
  return colspan > 1 && /暂无|没有|无记录|No data/i.test(cells[0].textContent || '')
}

function ensureControls(table) {
  const wrapper = table.closest('.table-wrap') || table
  const existing = wrapper.nextElementSibling
  if (existing?.dataset?.autoPagination === 'true') return existing
  const controls = document.createElement('div')
  controls.className = 'list-pagination-controls auto-pagination-controls'
  controls.dataset.autoPagination = 'true'
  wrapper.insertAdjacentElement('afterend', controls)
  return controls
}

function removeControls(table) {
  const wrapper = table.closest('.table-wrap') || table
  const existing = wrapper.nextElementSibling
  if (existing?.dataset?.autoPagination === 'true') existing.remove()
}

function summaryNode(total, totalPages, page) {
  const node = document.createElement('div')
  node.className = 'pagination-summary'
  node.textContent = `共 ${total} 条 / ${totalPages} 页，当前第 ${page} 页`
  return node
}

function actionsNode(table, state, total, totalPages) {
  const node = document.createElement('div')
  node.className = 'pagination-actions'
  const prev = button('上一页', state.page <= 1, () => {
    state.page -= 1
    renderAutoPagination(table)
  })
  const jumpLabel = document.createElement('label')
  const jumpText = document.createElement('span')
  jumpText.textContent = '跳至'
  const jumpInput = document.createElement('input')
  jumpInput.type = 'number'
  jumpInput.min = '1'
  jumpInput.max = String(totalPages)
  jumpInput.value = String(state.page)
  jumpInput.addEventListener('keyup', (event) => {
    if (event.key !== 'Enter') return
    state.page = clampPage(jumpInput.value, total, state.pageSize)
    renderAutoPagination(table)
  })
  jumpLabel.append(jumpText, jumpInput)
  const jumpButton = button('跳转', false, () => {
    state.page = clampPage(jumpInput.value, total, state.pageSize)
    renderAutoPagination(table)
  })
  const sizeLabel = document.createElement('label')
  const sizeText = document.createElement('span')
  sizeText.textContent = '每页'
  const sizeSelect = document.createElement('select')
  for (const option of PAGE_SIZE_OPTIONS) {
    const item = document.createElement('option')
    item.value = String(option)
    item.textContent = `${option} 条`
    item.selected = option === state.pageSize
    sizeSelect.append(item)
  }
  sizeSelect.addEventListener('change', () => {
    state.pageSize = normalizePageSize(sizeSelect.value)
    state.page = 1
    renderAutoPagination(table)
  })
  sizeLabel.append(sizeText, sizeSelect)
  const next = button('下一页', state.page >= totalPages, () => {
    state.page += 1
    renderAutoPagination(table)
  })
  node.append(prev, jumpLabel, jumpButton, sizeLabel, next)
  return node
}

function button(text, disabled, handler) {
  const node = document.createElement('button')
  node.className = 'secondary'
  node.type = 'button'
  node.textContent = text
  node.disabled = disabled
  node.addEventListener('click', handler)
  return node
}
