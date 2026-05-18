export function isDripProduct(product) {
  return product?.product_kind === 'drip_bag'
}

export function dripUnitOptions(product) {
  const bagGrams = positiveNumber(product?.drip_bag_grams) || 10
  const boxBagCount = positiveInteger(product?.drip_box_bag_count) || 10
  return [
    { value: 'bag', label: '袋', spec: `${formatNumber(bagGrams)}g/袋` },
    { value: 'box', label: '盒', spec: `${boxBagCount}袋/盒` }
  ]
}

export function validateDripProduct(product) {
  if (!isDripProduct(product)) return []
  const errors = []
  if (!positiveNumber(product?.drip_bag_grams)) errors.push('每袋熟豆克重必须大于 0')
  if (!positiveInteger(product?.drip_box_bag_count)) errors.push('每盒袋数必须大于 0')
  return errors
}

export function componentTypeLabel(type) {
  if (type === 'finished_product') return '成品'
  return '物料'
}

function positiveNumber(value) {
  const n = Number(value || 0)
  return n > 0 ? n : 0
}

function positiveInteger(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0
}

function formatNumber(value) {
  const n = Number(value || 0)
  return Number.isInteger(n) ? String(n) : n.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}
