import { defineStore } from 'pinia'
import type { DirectShipDraftLine } from '../utils/directShipDraft'

export type CustomerOrderMode = 'direct_ship' | 'product_order' | 'stock_purchase'
type FulfillmentMode = 'stock' | 'on_demand'

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
  fulfillment_mode?: 'stock' | 'on_demand'
  delivery_mode?: 'ship' | 'customer_stock'
  delivery_warehouse_code?: string
  return_to_direct_ship?: boolean
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
  fulfillmentMode?: FulfillmentMode
}

function draftKey(customerID: number, orderMode: CustomerOrderMode, fulfillmentMode?: FulfillmentMode): string {
  return `${Number(customerID || 0)}:${orderMode}${orderMode === 'direct_ship' && fulfillmentMode ? `:${fulfillmentMode}` : ''}`
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
    saveDraft(customerID: number, orderMode: CustomerOrderMode, draft: CustomerOrderDraft, fulfillmentMode?: FulfillmentMode) {
      this.drafts[draftKey(customerID, orderMode, fulfillmentMode || draft.fulfillment_mode)] = clone(draft)
    },
    restoreDraft(customerID: number, orderMode: CustomerOrderMode, fulfillmentMode?: FulfillmentMode): CustomerOrderDraft | null {
      let draft = this.drafts[draftKey(customerID, orderMode, fulfillmentMode)]
      if (!draft && orderMode === 'direct_ship') {
        const legacy = this.drafts[draftKey(customerID, orderMode)]
        if (legacy && (!fulfillmentMode || (legacy.fulfillment_mode || 'stock') === fulfillmentMode)) draft = legacy
        if (!fulfillmentMode && !draft) draft = this.drafts[draftKey(customerID, orderMode, 'stock')] || this.drafts[draftKey(customerID, orderMode, 'on_demand')]
      }
      if (draft && fulfillmentMode && draft.fulfillment_mode && draft.fulfillment_mode !== fulfillmentMode) return null
      return draft ? clone(draft) : null
    },
    clearDraft(customerID: number, orderMode: CustomerOrderMode, fulfillmentMode?: FulfillmentMode) {
      delete this.drafts[draftKey(customerID, orderMode, fulfillmentMode)]
      const legacy = this.drafts[draftKey(customerID, orderMode)]
      if (fulfillmentMode && legacy && (legacy.fulfillment_mode || 'stock') === fulfillmentMode) delete this.drafts[draftKey(customerID, orderMode)]
    },
    stageRecipient(customerID: number, orderMode: CustomerOrderMode, recipient: CustomerOrderRecipient, fulfillmentMode?: FulfillmentMode) {
      this.recipientHandoff = { customerID: Number(customerID || 0), orderMode, recipient: clone(recipient), fulfillmentMode }
    },
    consumeRecipient(customerID: number, orderMode: CustomerOrderMode, fulfillmentMode?: FulfillmentMode): CustomerOrderRecipient | null {
      const staged = this.recipientHandoff
      if (!staged || staged.customerID !== Number(customerID || 0) || staged.orderMode !== orderMode || staged.fulfillmentMode !== fulfillmentMode) return null
      this.recipientHandoff = null
      return clone(staged.recipient)
    },
  },
})
