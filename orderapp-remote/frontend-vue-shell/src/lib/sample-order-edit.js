// Eligibility is read from the server form response. It is never sent as write authority.
export function isSampleOrderEdit(data, context={}) {
  return data?.is_sample_order === true && Number(data.edit_id) > 0 && !context.copyMode && !context.customerPortal
}
export function validManualOrderPrice(row, allowZero=false) {
  const raw=row?.unit_price
  if(!row?.manual_price || raw === null || raw === undefined || String(raw).trim()==='') return false
  const value=Number(raw)
  return Number.isFinite(value) && (allowZero ? value >= 0 : value > 0)
}
export function storedSampleSpecKey(row) {
  const identity=sampleStoredItemIdentity(row)
  return identity.item_id > 0 && identity.product_id > 0 && identity.bom_spec_id > 0 && identity.bom_variant_id > 0
    ? Object.values(identity).join(':') : ''
}
export function preservesSampleSpec(row, sampleEdit) {
  return sampleEdit && !!row?.sample_stored_spec_key && row.sample_stored_spec_key===storedSampleSpecKey(row)
}
export function sampleStoredItemIdentity(item) {
  return {
    item_id:Number(item?.item_id || 0),
    product_id:Number(item?.product_id || 0),
    parent_product_id:Number(item?.parent_product_id || item?.product_id || 0),
    bom_spec_id:Number(item?.bom_spec_id || 0),
    bom_variant_id:Number(item?.bom_variant_id || 0),
    customer_product_alias_id:Number(item?.customer_product_alias_id || 0),
    customer_product_reference_id:Number(item?.customer_product_reference_id || 0),
  }
}
