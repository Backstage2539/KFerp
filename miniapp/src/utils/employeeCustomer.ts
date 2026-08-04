import type { EmployeeCustomerRecipientParseResponse } from '../api/customerPortal'

export type EmployeeCustomerRecipientFields = {
  name?: string
  contact?: string
  phone?: string
  address?: string
}

export type EmployeeCustomerRecipientTargetFields = {
  contact: string
  phone: string
  address: string
}

function cleanParsedField(value: unknown): string {
  return String(value ?? '').trim()
}

function currentField(value: unknown): string {
  return String(value ?? '')
}

export function snapshotEmployeeCustomerRecipientFields(
  current: EmployeeCustomerRecipientFields,
): EmployeeCustomerRecipientTargetFields {
  return {
    contact: currentField(current.contact),
    phone: currentField(current.phone),
    address: currentField(current.address),
  }
}

export function mergeEmployeeCustomerRecipientFields(
  current: EmployeeCustomerRecipientFields,
  parsed: EmployeeCustomerRecipientParseResponse,
  started: EmployeeCustomerRecipientTargetFields = snapshotEmployeeCustomerRecipientFields(current),
): EmployeeCustomerRecipientTargetFields {
  const latest = snapshotEmployeeCustomerRecipientFields(current)
  const recipientName = cleanParsedField(parsed.recipient_name)
  const phone = cleanParsedField(parsed.phone)
  const address = cleanParsedField(parsed.address)

  return {
    contact: latest.contact === started.contact && recipientName ? recipientName : latest.contact,
    phone: latest.phone === started.phone && phone ? phone : latest.phone,
    address: latest.address === started.address && address ? address : latest.address,
  }
}
