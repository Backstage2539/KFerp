import test from 'node:test'
import assert from 'node:assert/strict'

import {
  clampPage,
  pageCount,
  paginationFromApi,
  slicePageRows,
  normalizePageSize,
} from './pagination.js'

test('pagination helpers normalize totals, page size and page count', () => {
  assert.equal(normalizePageSize(0), 10)
  assert.equal(normalizePageSize('20'), 20)
  assert.equal(normalizePageSize(999), 100)
  assert.equal(pageCount(0, 10), 1)
  assert.equal(pageCount(52, 10), 6)
  assert.equal(clampPage(9, 52, 10), 6)
  assert.equal(clampPage(-1, 52, 10), 1)
})

test('pagination helpers slice visible rows for the current page', () => {
  const rows = Array.from({ length: 25 }, (_, index) => ({ id: index + 1 }))
  assert.deepEqual(slicePageRows(rows, { page: 1, pageSize: 10 }).map((row) => row.id), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
  assert.deepEqual(slicePageRows(rows, { page: 3, pageSize: 10 }).map((row) => row.id), [21, 22, 23, 24, 25])
  assert.deepEqual(slicePageRows(rows, { page: 99, pageSize: 10 }).map((row) => row.id), [21, 22, 23, 24, 25])
})

test('pagination helpers read API pagination metadata consistently', () => {
  assert.deepEqual(paginationFromApi({ total: 53, page: 4, limit: 20, total_pages: 3 }), {
    total: 53,
    page: 3,
    pageSize: 20,
    totalPages: 3,
    hasPrev: true,
    hasNext: false,
  })
  assert.deepEqual(paginationFromApi({ rows: [{ id: 1 }, { id: 2 }], page: 1, limit: 10 }), {
    total: 2,
    page: 1,
    pageSize: 10,
    totalPages: 1,
    hasPrev: false,
    hasNext: false,
  })
})
