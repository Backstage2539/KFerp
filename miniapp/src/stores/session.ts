import { clearHomeOrderEntry } from '../utils/homeOrderHandoff'
import { employeeEntryCache } from '../utils/employeeEntryCache'
import { currentLoginConsent, loginAccount, loginPolicyVersion, type LoginConsent } from '../utils/loginConsent'
import type { RegistrationProfile } from '../api/registration'
import { markPreviewToken, unmarkPreviewToken } from '../api/client'
import { defineStore } from 'pinia'
import type { CustomerBinding, MiniappEntryMode, MeResponse, LoginResponse } from '../api/customerPortal'
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

export function storedLoginSession(): { token: string; consent: LoginConsent | null } {
  const record = uni.getStorageSync(miniappTokenStorageKey())
  if (uni.getStorageSync(legacyTokenKey)) uni.removeStorageSync(legacyTokenKey)
  if (currentLoginConsent(record?.consent) && typeof record.token === 'string') {
    return { token: record.token, consent: record.consent }
  }
  if (record) uni.removeStorageSync(miniappTokenStorageKey())
  return { token: '', consent: null }
}

export const useSessionStore = defineStore('session', {
  state: () => ({
    ...storedLoginSession(),
    previewEmployeeToken: '',
    autoRestoreBlocked: !!uni.getStorageSync(miniappStorageKey('signed-out', configuredMiniappEnvironment())),
 registration: null as RegistrationProfile|null,
 registrationComplete:false,
 registrationKnown:!!uni.getStorageSync(miniappStorageKey('registered',configuredMiniappEnvironment())),
    miniUserID: 0,
    currentCustomerID: 0,
    currentCustomerName: '',
    themeKey: defaultMiniappThemeKey as MiniappThemeKey,
    entryMode: 'services' as MiniappEntryMode,
    bindings: [] as CustomerBinding[],
    capabilities: [] as Capability[],
    fulfillmentModes: [] as ('stock' | 'on_demand')[],
    accountType: '' as string,
    employeeID: 0,
    employeeName: '',
    roles: [] as string[],
    permissions: [] as string[],
  }),
  getters: {
    isCustomerPreview: (state) => !!state.previewEmployeeToken,
  },
  actions: {
    acceptLogin(response: LoginResponse) {
      const account = loginAccount(response)
      if (!account || !response.token) throw new Error('无法确认登录身份，请重新登录')
      const consent = { account, version: loginPolicyVersion, acceptedAt: Date.now() }
      this.setToken(response.token, consent)
      this.applyContext(response)
    },
    setToken(token: string, accepted?: LoginConsent) {
      if (token !== this.token) {
        employeeEntryCache.clear()
        clearHomeOrderEntry()
      }
      const consent = accepted ?? this.consent
      uni.setStorageSync(miniappTokenStorageKey(), { token, consent })
      this.consent = consent
      if (this.previewEmployeeToken) unmarkPreviewToken(this.token)
      this.previewEmployeeToken = ''
      this.token = token
      this.autoRestoreBlocked = false
      uni.removeStorageSync(miniappStorageKey('signed-out', configuredMiniappEnvironment()))
    },
    beginCustomerPreview(token: string, context: Omit<MeResponse, 'current_customer_name'> & {current_customer_name?: string}) {
      if (this.accountType !== 'employee' || !this.roles.includes('admin') || !this.token || !token || context.account_type !== 'preview' || !context.current_customer_id) {
        throw new Error('无法进入客户只读预览')
      }
      employeeEntryCache.clear()
      clearHomeOrderEntry()
      this.previewEmployeeToken = this.token
      this.token = token
      markPreviewToken(token)
      this.applyContext(context)
    },
    endCustomerPreview() {
      if (!this.previewEmployeeToken) return
      const employeeToken = this.previewEmployeeToken
      unmarkPreviewToken(this.token)
      this.previewEmployeeToken = ''
      this.clearSession(true)
      this.setToken(employeeToken)
    },
    logout() {
      if (this.previewEmployeeToken) unmarkPreviewToken(this.token)
      this.previewEmployeeToken = ''
      this.consent = null
      this.autoRestoreBlocked = true
      uni.setStorageSync(miniappStorageKey('signed-out', configuredMiniappEnvironment()), true)
      this.clearSession(true)
    },
    clearSession(preserveRegistrationHint = false) {
      employeeEntryCache.clear()
      clearHomeOrderEntry()
      if (this.previewEmployeeToken) {
        this.endCustomerPreview()
        return
      }
      this.token = ''
      this.registration = null
      this.registrationComplete = false
      if (!preserveRegistrationHint) {
        this.registrationKnown = false
        uni.removeStorageSync(miniappStorageKey('registered', configuredMiniappEnvironment()))
      }
      this.miniUserID = 0
      this.currentCustomerID = 0
      this.currentCustomerName = ''
      this.themeKey = defaultMiniappThemeKey
      this.entryMode = 'services'
      this.bindings = []
      this.capabilities = []
      this.fulfillmentModes = []
      this.accountType = ''
      this.employeeID = 0
      this.employeeName = ''
      this.roles = []
      this.permissions = []
      if (!preserveRegistrationHint) this.consent = null
      if (this.consent) uni.setStorageSync(miniappTokenStorageKey(), { token: '', consent: { ...this.consent } })
      else uni.removeStorageSync(miniappTokenStorageKey())
    },
    applyContext(context: {
 registration_complete?:boolean
 registration?:RegistrationProfile
      mini_user_id: number
      current_customer_id: number
      current_customer_name?: string
      theme_key?: string
      miniapp_entry_mode?: string
      bindings: CustomerBinding[]
      capabilities: Capability[]
      fulfillment_modes?: ('stock' | 'on_demand')[]
      account_type?: string
      employee_id?: number
      employee_name?: string
      roles?: string[]
      permissions?: string[]
    }) {
      if(context.registration_complete!==undefined){this.registrationComplete=context.registration_complete;this.registration=context.registration||null;this.registrationKnown=context.registration_complete;uni.setStorageSync(miniappStorageKey('registered',configuredMiniappEnvironment()),context.registration_complete)}
      if (context.fulfillment_modes !== undefined) this.fulfillmentModes = context.fulfillment_modes
      else if (this.currentCustomerID !== context.current_customer_id) this.fulfillmentModes = []
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
