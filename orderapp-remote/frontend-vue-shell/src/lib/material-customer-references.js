function positiveUniqueIDs(values = []) {
  return Array.from(new Set((values || []).map((value) => Number(value || 0)).filter((value) => value > 0)))
}

export function materialBelongsToCatalogContext(material = {}, customerID = 0, references = []) {
  const scopedCustomerID = Number(customerID || 0)
  if (!scopedCustomerID) return true
  const materialID = Number(material.id || material.material_id || 0)
  return (references || []).some((reference) => (
    reference?.active !== false
    && Number(reference?.material_id || 0) === materialID
    && Number(reference?.customer_id || 0) === scopedCustomerID
  ))
}

export function materialCustomerIDs(material = {}, references = []) {
  const materialID = Number(material.id || material.material_id || 0)
  return positiveUniqueIDs((references || [])
    .filter((reference) => reference?.active !== false && Number(reference?.material_id || 0) === materialID)
    .map((reference) => reference.customer_id))
}

export function materialCustomerNames(material = {}, references = [], customers = []) {
  const ids = materialCustomerIDs(material, references)
  return ids.map((id) => customers.find((customer) => Number(customer?.id || 0) === id)?.name || `客户 #${id}`).join('、')
}

export function buildMaterialCreatePayload(materialPayload = {}, ownershipType = 'factory', customerIDs = []) {
  return {
    ...materialPayload,
    customer_ids: String(ownershipType || '').trim().toLowerCase() === 'customers' ? positiveUniqueIDs(customerIDs) : [],
  }
}

export function buildMaterialCustomerReferencePayload(form = {}) {
  return {
    id: Number(form.id || 0),
    material_id: Number(form.material_id || 0),
    customer_id: Number(form.customer_id || 0),
    active: form.active !== false,
    remark: String(form.remark || '').trim(),
  }
}
