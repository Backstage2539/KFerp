export const qualityTargetTabs = [
  { scope: 'work_order', label: '工单质检' },
  { scope: 'raw_material', label: '原料质检' },
  { scope: 'finished_batch', label: '产品质检' },
]

export function qualityTargetAPIPath(scope) {
  if (scope === 'raw_material') return '/api/stock/material-batches?active_only=1&limit=100'
  if (scope === 'finished_batch') return '/api/stock/batches?item_type=finished_product&limit=100'
  return '/api/produce/work-orders?limit=100'
}

export function qualityTargetFromRow(scope, row = {}) {
  if (scope === 'raw_material') {
    return {
      scope,
      reference_type: scope,
      reference_no: row.batch_code || '',
      item_name: row.material_name || row.item_name || '',
    }
  }
  if (scope === 'finished_batch') {
    return {
      scope,
      reference_type: scope,
      reference_no: row.batch_code || '',
      item_name: row.item_name || row.product_name || '',
    }
  }
  return {
    scope: 'work_order',
    reference_type: 'work_order',
    reference_no: row.work_order_no || row.reference_no || '',
    item_name: row.product_name || row.item_name || '',
  }
}

export function qualityTargetStatus(row = {}) {
  return row.quality_status || 'unchecked'
}

export function qualityTargetPrimary(scope, row = {}) {
  if (scope === 'work_order') return row.work_order_no || '-'
  return row.batch_code || '-'
}

export function qualityTargetName(scope, row = {}) {
  if (scope === 'raw_material') return row.material_name || row.item_name || '-'
  if (scope === 'finished_batch') return row.item_name || row.product_name || '-'
  return row.product_name || row.item_name || '-'
}

export function qualityTargetMeta(scope, row = {}) {
  if (scope === 'raw_material') {
    return [
      row.supplier ? `供应商 ${row.supplier}` : '',
      row.remaining_g != null ? `剩余 ${Number(row.remaining_g || 0).toLocaleString('zh-CN')}g` : '',
    ].filter(Boolean).join(' · ')
  }
  if (scope === 'finished_batch') {
    return [
      row.source_doc_type ? `${row.source_doc_type} #${row.source_doc_id || '-'}` : '',
      row.remaining_g != null ? `剩余 ${Number(row.remaining_g || 0).toLocaleString('zh-CN')}g` : '',
      row.qty_units != null ? `${Number(row.qty_units || 0).toLocaleString('zh-CN')}件` : '',
    ].filter(Boolean).join(' · ')
  }
  return [
    row.batch_id ? `批次 ${row.batch_id}` : '',
    row.spec_g ? `${row.spec_g}g` : '',
    row.status ? `状态 ${row.status}` : '',
  ].filter(Boolean).join(' · ')
}

export function filterQualityTargets(scope, rows = [], keyword = '') {
  const q = String(keyword || '').trim().toLowerCase()
  if (!q) return rows
  return rows.filter((row) => [
    qualityTargetPrimary(scope, row),
    qualityTargetName(scope, row),
    qualityTargetMeta(scope, row),
    row.order_nos || '',
  ].some((value) => String(value || '').toLowerCase().includes(q)))
}
