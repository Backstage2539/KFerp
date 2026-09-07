function identity(row = {}) {
  const product = Number(row.parent_product_id || row.product_id || row.sku_id || 0)
  const spec = Number(row.bom_spec_id || row.sku_snapshot?.bom_spec_id || row.effective_sales_spec?.bom_spec_id || 0)
  return spec > 0 ? `${product}:bom:${spec}:${Number(row.bom_variant_id || 0)}` : `${product}:sku:${Number(row.sku_id || row.product_id || 0)}:${row.spec_label || row.sku_snapshot?.spec_label || ''}`
}
function clone(value) { return JSON.parse(JSON.stringify(value)) }
function latest(rows) { return rows.slice().sort((a,b) => String(b.published_at || b.created_at || '').localeCompare(String(a.published_at || a.created_at || '')) || Number(b.id)-Number(a.id))[0] }
export function seedCustomerPriceRows(existing = [], generated = [], publications = [], customerID = 0) {
  const out = clone(existing)
  const present = new Set(out.map(identity))
  const wanted = new Set(generated.map(identity))
  const supply = publications.filter(p => !p.publication_purpose || p.publication_purpose === 'factory_supply')
  const own = supply.filter(p => p.owner_type === 'customer' && Number(p.owner_key || p.customer_id) === Number(customerID))
  const customer = latest(own.filter(p => p.status === 'draft')) || latest(own.filter(p => p.status === 'published'))
  const official = latest(supply.filter(p => p.owner_type !== 'customer' && p.status === 'published'))
  for (const publication of [customer, official].filter(Boolean)) {
    const grouped = new Map()
    for (const source of publication.content?.price_rows || []) {
      const key = identity(source)
      if (!wanted.has(key) || present.has(key)) continue
      if (!grouped.has(key)) grouped.set(key, [])
      const rows = grouped.get(key)
      rows.push({ ...clone(source), row_key: source.row_key || `customer:${key}:${source.min_qty || 0}:${source.max_qty ?? 'open'}:${source.price_unit || ''}`, customer_price_source_publication_id: Number(publication.id), customer_price_source_version: publication.version_no || publication.version || '' })
    }
    for (const [key,rows] of grouped) {out.push(...rows);present.add(key)}
  }
  return out
}
export function applyCustomerPriceRows(generated = [], seeds = [], overrides = {}, customerID = 0) {
  if (!(Number(customerID) > 0)) return generated
  const base = new Map(generated.map(r => [identity(r),r]))
  const out=[]
  for (const [key,current] of base) {
    const sources=seeds.filter(r => identity(r)===key)
    const candidates=sources.length?sources:[{...current,final_unit_price:0,customer_quote_missing:true}]
    for (const source of candidates) {
      const rowKey=source.row_key||current.row_key
      const edited=Number(overrides[rowKey])
      const amount=Number.isFinite(edited)&&edited>0?edited:Number(source.final_unit_price||0)
      out.push({ ...current,...clone(source),row_key:rowKey,
        product_name:current.product_name,group_snapshot:current.group_snapshot,group_source:'product_catalog',
        customer_reference_snapshot:current.customer_reference_snapshot,customer_product_alias_id:current.customer_product_alias_id,
        tier_template_id:0,pricing_rule_id:0,pricing_rule_version:'',tier_unit_compatible:sources.length?true:current.tier_unit_compatible,
        pricing_mode:'fixed_price',pricing_mode_source:'customer_quote',fixed_unit_price:amount,final_unit_price:amount,
        original_final_unit_price:Number(source.final_unit_price||0),manual_adjusted:edited>0,
        customer_quote_missing:!(amount>0),
        cost_source_snapshot:{...source.cost_source_snapshot,customer_price_source:{publication_id:source.customer_price_source_publication_id,version:source.customer_price_source_version}},
      })
    }
  }
  return out
}
