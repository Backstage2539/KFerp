import { defineStore } from 'pinia'
import type { DirectShipDraftLine } from '../utils/directShipDraft'

export type CustomerOrderMode = 'direct_ship' | 'product_order'

export type CustomerOrderRecipient = {
  id?: number
  recipient_name: string
  phone: string
  company?: string
  province?: string
  city?: string
  district?: string
  detail_address: string
  is_default?: boolean
  revision?: number
}

export type CustomerOrderDraft = {
  order_date: string
  recipient: CustomerOrderRecipient | null
  note: string
  lines: DirectShipDraftLine[]
  idempotency_key: string
}

type RecipientHandoff = {
  customerID: number
  orderMode: CustomerOrderMode
  recipient: CustomerOrderRecipient
}

function draftKey(customerID: number, orderMode: CustomerOrderMode): string {
  return `${Number(customerID || 0)}:${orderMode}`
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

export const useCustomerOrderDraftStore = defineStore('customer-order-draft', {
  state: () => ({
    drafts: {} as Record<string, CustomerOrderDraft>,
    recipientHandoff: null as RecipientHandoff | null,
  }),
  actions: {
    saveDraft(customerID: number, orderMode: CustomerOrderMode, draft: CustomerOrderDraft) {
      this.drafts[draftKey(customerID, orderMode)] = clone(draft)
    },
    restoreDraft(customerID: number, orderMode: CustomerOrderMode): CustomerOrderDraft | null {
      const draft = this.drafts[draftKey(customerID, orderMode)]
      return draft ? clone(draft) : null
    },
    clearDraft(customerID: number, orderMode: CustomerOrderMode) {
      delete this.drafts[draftKey(customerID, orderMode)]
    },
    stageRecipient(customerID: number, orderMode: CustomerOrderMode, recipient: CustomerOrderRecipient) {
      this.recipientHandoff = { customerID: Number(customerID || 0), orderMode, recipient: clone(recipient) }
    },
    consumeRecipient(customerID: number, orderMode: CustomerOrderMode): CustomerOrderRecipient | null {
      const staged = this.recipientHandoff
      if (!staged || staged.customerID !== Number(customerID || 0) || staged.orderMode !== orderMode) return null
      this.recipientHandoff = null
      return clone(staged.recipient)
    },
  },
})
