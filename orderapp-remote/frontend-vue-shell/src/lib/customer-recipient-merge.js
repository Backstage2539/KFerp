function currentField(value) {
  return String(value ?? '')
}

function parsedField(value) {
  return currentField(value).trim()
}

export function customerPhoneForERPForm(customer = {}) {
  return parsedField(customer.company_phone) || parsedField(customer.phone)
}

export function customerRecipientFieldSnapshot(current = {}) {
  return {
    contact: currentField(current.contact),
    company_phone: currentField(current.company_phone),
    address: currentField(current.address),
  }
}

export function mergeCustomerRecipientFields(
  current = {},
  parsed = {},
  started = customerRecipientFieldSnapshot(current),
) {
  const latest = customerRecipientFieldSnapshot(current)
  const recipientName = parsedField(parsed.recipient_name)
  const phone = parsedField(parsed.phone)
  const address = parsedField(parsed.address)

  return {
    contact: latest.contact === started.contact && recipientName ? recipientName : latest.contact,
    company_phone: latest.company_phone === started.company_phone && phone ? phone : latest.company_phone,
    address: latest.address === started.address && address ? address : latest.address,
  }
}
