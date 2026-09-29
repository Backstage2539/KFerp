export function duplicateBOMComponentRow(component, nextRowID) {
  if (!component || typeof component !== 'object' || Array.isArray(component)) {
    throw new TypeError('BOM component row is required')
  }

  const rowID = String(nextRowID || '').trim()
  if (!rowID || rowID === String(component.row_id || '')) {
    throw new Error('A new BOM component row ID is required')
  }

  return { ...component, row_id: rowID }
}
