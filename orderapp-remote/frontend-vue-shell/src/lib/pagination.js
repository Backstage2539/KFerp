export const DEFAULT_PAGE_SIZE = 10
export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]

function positiveInteger(value, fallback) {
  const n = Number.parseInt(String(value ?? ''), 10)
  return Number.isFinite(n) && n > 0 ? n : fallback
}

export function normalizePageSize(value, options = PAGE_SIZE_OPTIONS) {
  const sorted = [...options].map((item) => positiveInteger(item, DEFAULT_PAGE_SIZE)).sort((a, b) => a - b)
  const fallback = sorted[0] || DEFAULT_PAGE_SIZE
  const requested = positiveInteger(value, fallback)
  const max = sorted[sorted.length - 1] || fallback
  if (requested >= max) return max
  return sorted.find((option) => option >= requested) || fallback
}

export function pageCount(total, pageSize = DEFAULT_PAGE_SIZE) {
  const size = normalizePageSize(pageSize)
  const count = Math.max(0, positiveInteger(total, 0))
  return Math.max(1, Math.ceil(count / size))
}

export function clampPage(page, total, pageSize = DEFAULT_PAGE_SIZE) {
  const pages = pageCount(total, pageSize)
  return Math.min(Math.max(positiveInteger(page, 1), 1), pages)
}

export function slicePageRows(rows, pagination = {}) {
  const source = Array.isArray(rows) ? rows : []
  const pageSize = normalizePageSize(pagination.pageSize)
  const page = clampPage(pagination.page, source.length, pageSize)
  const start = (page - 1) * pageSize
  return source.slice(start, start + pageSize)
}

export function paginationFromApi(data = {}) {
  const pageSize = normalizePageSize(data.limit ?? data.page_size ?? data.pageSize)
  const fallbackTotal = Array.isArray(data.rows) ? data.rows.length : 0
  const total = Math.max(0, positiveInteger(data.total ?? data.total_count, fallbackTotal))
  const totalPages = Math.max(1, positiveInteger(data.total_pages, pageCount(total, pageSize)))
  const page = Math.min(Math.max(positiveInteger(data.page, 1), 1), totalPages)
  return {
    total,
    page,
    pageSize,
    totalPages,
    hasPrev: data.has_prev === undefined ? page > 1 : Boolean(data.has_prev),
    hasNext: data.has_next === undefined ? page < totalPages : Boolean(data.has_next),
  }
}
