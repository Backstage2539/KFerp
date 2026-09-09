import { customerCatalogProjection } from './customer-catalog.js'

// Read all dependencies before applying anything to the current editing draft.
// The caller supplies the shared authenticated API client.
export async function fetchPriceListRefreshSnapshot({ apiGet, customerID = 0, prices = false }) {
  const customer = Number(customerID) > 0 ? Number(customerID) : 0
  const catalog = customer > 0
    ? apiGet(`/api/product-settings/customer-catalog?customer_id=${customer}`).then(data => {
      const projection = customerCatalogProjection(data)
      return { groups: projection.groups, assignments: projection.assignments,
        featureSelection: { feature_key: 'product_catalog', group_template_ids: projection.groupIDs } }
    })
    : Promise.all([
      apiGet('/api/business-groups'),
      apiGet('/api/business-group-assignments?usage_key=product_catalog&object_key=product'),
      apiGet('/api/business-group-feature-selections/product_catalog'),
    ]).then(([groups, assignments, featureSelection]) => ({
      groups: groups?.rows || groups?.groups || (Array.isArray(groups) ? groups : []),
      assignments: assignments?.rows || assignments?.assignments || (Array.isArray(assignments) ? assignments : []),
      featureSelection,
    }))
  const [beanList, grouping, templates] = await Promise.all([
    apiGet(`/api/costing/bean-list${customer ? `?customer_id=${customer}` : ''}`),
    catalog,
    prices ? Promise.all([apiGet('/api/price-tier-templates'), apiGet('/api/product-pricing-rules')]) : null,
  ])
  return {
    items: Array.isArray(beanList?.items) ? beanList.items : [], parameters: beanList?.parameters,
    ...grouping,
    ...(templates ? {
      templates: (templates[0]?.templates || templates[0]?.rows || []).filter(row => row?.active !== false),
      rules: (templates[1]?.rules || templates[1]?.rows || []).filter(row => row?.active !== false),
    } : {}),
  }
}
