import { defineStore } from 'pinia'
import type { CustomerBinding, MiniappEntryMode } from '../api/customerPortal'
import {
  configuredMiniappEnvironment,
  miniappStorageKey,
  type MiniappBuildEnvironment,
} from '../config/environment'
import type { Capability } from '../utils/capabilities'
import { defaultMiniappThemeKey, normalizeMiniappThemeKey, type MiniappThemeKey } from '../utils/themes'

const legacyTokenKey = 'kferp.mini.token'

export function miniappTokenStorageKey(environment?: MiniappBuildEnvironment): string {
  if (environment) return `kferp.mini.${environment}.token`
  return miniappStorageKey('token', configuredMiniappEnvironment())
}

function storedMiniappToken(): string {
  const token = uni.getStorageSync(miniappTokenStorageKey()) || ''
  if (uni.getStorageSync(legacyTokenKey)) uni.removeStorageSync(legacyTokenKey)
  return token
}

export const useSessionStore = defineStore('session', {
  state: () => ({
    token: storedMiniappToken(),
    miniUserID: 0,
    currentCustomerID: 0,
    currentCustomerName: '',
    themeKey: defaultMiniappThemeKey as MiniappThemeKey,
    entryMode: 'services' as MiniappEntryMode,
    bindings: [] as CustomerBinding[],
    capabilities: [] as Capability[],
    accountType: '' as string,
    employeeID: 0,
    employeeName: '',
    roles: [] as string[],
    permissions: [] as string[],
  }),
  actions: {
    setToken(token: string) {
      this.token = token
      uni.setStorageSync(miniappTokenStorageKey(), token)
    },
    clearSession() {
      this.token = ''
      this.miniUserID = 0
      this.currentCustomerID = 0
      this.currentCustomerName = ''
      this.themeKey = defaultMiniappThemeKey
      this.entryMode = 'services'
      this.bindings = []
      this.capabilities = []
      this.accountType = ''
      this.employeeID = 0
      this.employeeName = ''
      this.roles = []
      this.permissions = []
      uni.removeStorageSync(miniappTokenStorageKey())
    },
    applyContext(context: {
      mini_user_id: number
      current_customer_id: number
      current_customer_name?: string
      theme_key?: string
      miniapp_entry_mode?: string
      bindings: CustomerBinding[]
      capabilities: Capability[]
      account_type?: string
      employee_id?: number
      employee_name?: string
      roles?: string[]
      permissions?: string[]
    }) {
      this.miniUserID = context.mini_user_id
      this.currentCustomerID = context.current_customer_id
      this.currentCustomerName = context.current_customer_name || ''
      this.bindings = context.bindings || []
      this.capabilities = context.capabilities || []
      this.themeKey = normalizeMiniappThemeKey(context.theme_key)
      this.entryMode = context.miniapp_entry_mode === 'mall' ? 'mall' : 'services'
      this.accountType = context.account_type || 'customer'
      this.employeeID = context.employee_id || 0
      this.employeeName = context.employee_name || ''
      this.roles = context.roles || []
      this.permissions = context.permissions || []
    },
  },
})
