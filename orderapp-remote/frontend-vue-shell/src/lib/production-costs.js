export function visibleProductionTraceLinks(rows = []) {
  return (rows || []).filter((row) => {
    if (Number(row?.stock_entry_id || 0) > 0) return true
    if (String(row?.entry_no || '').trim()) return true
    if (String(row?.entry_type || '').trim()) return true
    if (Number(row?.material_id || 0) > 0) return true
    if (String(row?.material_name || '').trim()) return true
    if (String(row?.batch_code || '').trim()) return true
    return Number(row?.qty_g || 0) > 0
  })
}
