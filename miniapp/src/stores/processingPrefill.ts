import { defineStore } from 'pinia'
import {
  normalizeProcessingPrefillItems,
  type ProcessingPrefillItem,
} from '../utils/customerInventory'

export const useProcessingPrefillStore = defineStore('processingPrefill', {
  state: () => ({
    customerID: 0,
    items: [] as ProcessingPrefillItem[],
  }),
  actions: {
    stage(customerID: number, items: Array<Partial<ProcessingPrefillItem>>) {
      const expectedCustomerID = Math.max(0, Number(customerID || 0))
      if (expectedCustomerID <= 0) {
        this.clear()
        return
      }
      this.customerID = expectedCustomerID
      this.items = normalizeProcessingPrefillItems(items)
    },
    consume(customerID: number): ProcessingPrefillItem[] {
      const expectedCustomerID = Math.max(0, Number(customerID || 0))
      const matchesCustomer = expectedCustomerID > 0 && this.customerID > 0 && this.customerID === expectedCustomerID
      const items = matchesCustomer ? this.items.slice() : []
      this.clear()
      return items
    },
    clear() {
      this.customerID = 0
      this.items = []
    },
  },
})
