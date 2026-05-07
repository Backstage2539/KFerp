export const orderInvoiceFileAccept = '.pdf,image/png,image/jpeg,image/gif,image/webp'

const allowedInvoiceTypes = new Set([
  'application/pdf',
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
])

const allowedInvoiceExtensions = new Set(['.pdf', '.png', '.jpg', '.jpeg', '.gif', '.webp'])

export function invoiceStatusLabel(status) {
  switch (String(status || '').trim()) {
    case 'requested':
      return '已申请'
    case 'uploaded':
      return '已上传'
    default:
      return '未申请'
  }
}

export function invoiceStatusTone(status) {
  switch (String(status || '').trim()) {
    case 'uploaded':
      return 'ok'
    case 'requested':
      return 'warn'
    default:
      return 'muted'
  }
}

export function orderInvoiceFileAllowed(file) {
  if (!file) return false
  const type = String(file.type || '').toLowerCase()
  if (allowedInvoiceTypes.has(type)) return true
  const name = String(file.name || '').toLowerCase()
  const dot = name.lastIndexOf('.')
  const ext = dot >= 0 ? name.slice(dot) : ''
  return allowedInvoiceExtensions.has(ext) && type !== 'image/svg+xml'
}

export function orderInvoiceAssetName(asset) {
  if (!asset) return '暂无发票文件'
  if (asset.filename) return asset.filename
  const url = String(asset.url || '')
  if (!url) return '暂无发票文件'
  const parts = url.split('/').filter(Boolean)
  return decodeURIComponent(parts[parts.length - 1] || '发票文件')
}
