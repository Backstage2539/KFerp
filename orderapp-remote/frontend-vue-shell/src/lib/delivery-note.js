export function deliveryNotePageUrl(orderID) {
  return `/vue-shell?view=deliveryNote&order_id=${Number(orderID || 0)}`
}

export function deliveryNoteDownloadUrl(orderID) {
  return `/orders/${Number(orderID || 0)}/delivery-note-latest.pdf`
}
