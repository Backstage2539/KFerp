import { defineStore } from 'pinia'
import type { CustomerBinding } from '../api/customerPortal'
import type { Capability } from '../utils/capabilities'

const tokenKey = 'kferp.mini.token'

export const useSessionStore = defineStore('session', {
  state: () => ({
    token: uni.getStorageSync(tokenKey) || '',
    miniUserID: 0,
    currentCustomerID: 0,
    currentCustomerName: '',
    bindings: [] as CustomerBinding[],
    capabilities: [] as Capability[],
  }),
  actions: {
    setToken(token: string) {
      this.token = token
      uni.setStorageSync(tokenKey, token)
    },
    applyContext(context: {
      mini_user_id: number
      current_customer_id: number
      current_customer_name?: string
      bindings: CustomerBinding[]
      capabilities: Capability[]
    }) {
      this.miniUserID = context.mini_user_id
      this.currentCustomerID = context.current_customer_id
      this.currentCustomerName = context.current_customer_name || ''
      this.bindings = context.bindings || []
      this.capabilities = context.capabilities || []
    },
  },
})
