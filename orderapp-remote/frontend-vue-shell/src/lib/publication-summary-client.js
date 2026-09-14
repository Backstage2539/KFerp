export function createInFlightRequestDeduper(request) {
  const inFlight = new Map()
  return function requestOnce(key) {
    const requestKey = String(key || '')
    if (inFlight.has(requestKey)) return inFlight.get(requestKey)
    const pending = Promise.resolve(request(requestKey))
    inFlight.set(requestKey, pending)
    pending.finally(() => {
      if (inFlight.get(requestKey) === pending) inFlight.delete(requestKey)
    }).catch(() => {})
    return pending
  }
}

export function createLatestRequestGate() {
  const revisions = new Map()
  return {
    begin(key) {
      const next = (revisions.get(key) || 0) + 1
      revisions.set(key, next)
      return next
    },
    isCurrent(key, revision) {
      return revisions.get(key) === revision
    },
  }
}

export function buildPublicationSummaryURL({
  listType,
  scope,
  customerID = 0,
  publicationPurpose = 'factory_supply',
  productTypeCategoryID = 0,
  classificationTemplateID = 0,
  status = 'active',
  page = 1,
  pageSize = 10,
  search = '',
} = {}) {
  const params = new URLSearchParams({
    view: 'summary',
    list_type: String(listType || 'commercial'),
    scope: String(scope || 'official'),
    publication_purpose: String(publicationPurpose || 'factory_supply'),
    status: String(status || 'active'),
    page: String(Math.max(1, Number(page) || 1)),
    page_size: String(Number(pageSize) || 10),
  })
  if (Number(productTypeCategoryID) > 0) params.set('product_type_category_id', String(Number(productTypeCategoryID)))
  if (Number(classificationTemplateID) > 0) params.set('classification_template_id', String(Number(classificationTemplateID)))
  if (String(scope || '') === 'customer') params.set('customer_id', String(Number(customerID) || 0))
  if (String(search || '').trim()) params.set('search', String(search).trim())
  return `/api/costing/bean-list/publications?${params.toString()}`
}
