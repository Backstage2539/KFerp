function toOrderID(value) {
  const id = Number(value || 0)
  return Number.isFinite(id) && id > 0 ? id : 0
}

export function selectableOrderIDs(rows = []) {
  return (rows || [])
    .filter((row) => !row?.is_void)
    .map((row) => toOrderID(row?.id))
    .filter(Boolean)
}

export function orderListSelectionState(rows = [], selectedIDs = []) {
  const visibleIDs = selectableOrderIDs(rows)
  const selected = new Set((selectedIDs || []).map(toOrderID).filter(Boolean))
  const selectedCount = visibleIDs.filter((id) => selected.has(id)).length
  const selectableCount = visibleIDs.length
  return {
    checked: selectableCount > 0 && selectedCount === selectableCount,
    indeterminate: selectedCount > 0 && selectedCount < selectableCount,
    selectableCount,
    selectedCount,
  }
}

export function toggleOrderPageSelection(rows = [], selectedIDs = []) {
  const visibleIDs = selectableOrderIDs(rows)
  if (!visibleIDs.length) return [...(selectedIDs || [])]
  const visible = new Set(visibleIDs)
  const selected = new Set((selectedIDs || []).map(toOrderID).filter(Boolean))
  const allSelected = visibleIDs.every((id) => selected.has(id))
  if (allSelected) {
    return [...selected].filter((id) => !visible.has(id))
  }
  for (const id of visibleIDs) selected.add(id)
  return [...selected]
}
