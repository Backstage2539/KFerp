function materialKind(row) {
  return String(row?.kind || row?.Kind || '').trim().toLowerCase()
}

function normalizeSearchText(value) {
  return String(value || '').trim().toLowerCase()
}

function materialSearchText(row) {
  return [
    row?.code,
    row?.Code,
    row?.name,
    row?.Name,
    row?.origin,
    row?.Origin,
    row?.supplier,
    row?.Supplier,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
}

export function selectableReceiptMaterials(rows) {
  return (rows || []).filter((row) => materialKind(row) !== 'pack')
}

export function filterReceiptMaterials(rows, query) {
  const selectable = selectableReceiptMaterials(rows)
  const normalized = normalizeSearchText(query)
  if (!normalized) return selectable

  const terms = normalized.split(/\s+/).filter(Boolean)
  return selectable.filter((row) => {
    const haystack = materialSearchText(row)
    return terms.every((term) => haystack.includes(term))
  })
}

export function receiptMaterialLabel(row) {
  const code = String(row?.code || row?.Code || '').trim()
  const name = String(row?.name || row?.Name || '').trim()
  return code ? `${name} (${code})` : name
}
