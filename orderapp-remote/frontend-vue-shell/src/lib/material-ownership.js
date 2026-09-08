export function materialOwnerLabel(material = {}, fallbackCompanyName = '本公司') {
  return String(material.owner_name || '').trim() || (Number(material.owner_customer_id || 0) > 0 ? `客户 #${Number(material.owner_customer_id)}` : fallbackCompanyName)
}

export function filterMaterialsByOwnership(materials = [], ownership = 'all') {
  const selected = String(ownership || 'all').trim()
  if (!selected || selected === 'all') return [...(materials || [])]
  if (selected === 'factory') return (materials || []).filter((row) => Number(row.owner_customer_id || 0) === 0)
  if (selected.startsWith('customer:')) {
    const id = Number(selected.slice('customer:'.length) || 0)
    return (materials || []).filter((row) => Number(row.owner_customer_id || 0) === id)
  }
  const query = selected.toLowerCase()
  return (materials || []).filter((row) => Number(row.owner_customer_id || 0) > 0 && materialOwnerLabel(row).toLowerCase().includes(query))
}

export function buildMaterialCreatePayload(materialPayload = {}, ownerType = '', ownerCustomerID = 0) {
  const type = String(ownerType || '').trim().toLowerCase()
  if (!['factory', 'customer'].includes(type)) throw new Error('请选择物料归属')
  const customerID = Number(ownerCustomerID || 0)
  if (type === 'customer' && customerID <= 0) throw new Error('请选择归属客户')
  return { ...materialPayload, owner_type: type, owner_customer_id: type === 'customer' ? customerID : 0 }
}
