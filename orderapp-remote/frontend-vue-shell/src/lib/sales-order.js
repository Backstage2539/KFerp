export function salesOrderPageUrl(orderID) {
  return `/vue-shell?view=salesOrder&order_id=${Number(orderID || 0)}`
}

export function salesOrderDownloadUrl(orderID) {
  return `/orders/${Number(orderID || 0)}/sales-order-latest.pdf`
}

export function salesOrderImageDownloadUrl(orderID) {
  return `/orders/${Number(orderID || 0)}/sales-order-image-latest.png`
}
