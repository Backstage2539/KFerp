export function customerDossierNavigationDetail(row = {}) {
  const customerID = Number(row?.customer?.id || row?.id || 0)
  return {
    key: 'customers',
    params: customerID > 0 ? { edit_id: customerID } : {},
  }
}
