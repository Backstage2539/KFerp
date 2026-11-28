import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { onHide } from '@dcloudio/uni-app'
import { isAuthenticationExpiredRequestError } from '../api/client'
import { fetchEmployeeOrderDraft } from '../api/customerPortal'
import { fetchEntryBase } from '../api/employeeEntry'
import { useSessionStore } from '../stores/session'
import { filterEmployeeOrderCustomers } from '../utils/employeeOrder'
import { cancelHomeOrderEntry, passHomeOrderEntry, type HomeOrderBase, type HomeOrderIntent } from '../utils/homeOrderHandoff'
import { consentScope, hasRecipientConsent } from '../utils/recipientConsent'

export const employeeOrderEntryPath = '/pages/employee-order-entry/employee-order-entry'
type Stage = 'closed' | 'consent' | 'loading' | 'ready' | 'opening' | 'error'

export function useHomeOrderEntry() {
  const session = useSessionStore()
  const stage = ref<Stage>('closed')
  const query = ref('')
  const error = ref('')
  const base = shallowRef<HomeOrderBase>()
  const customers = computed(() => filterEmployeeOrderCustomers(base.value?.customers, query.value))
  const canCreate = computed(() => session.permissions.includes('customers.read') && session.permissions.includes('customers.write'))
  let generation = 0
  let handoffID = ''
  let retryIntent: HomeOrderIntent | undefined

  function close() {
    ++generation
    if (handoffID) cancelHomeOrderEntry(handoffID)
    handoffID = ''
    retryIntent = undefined
    base.value = undefined
    query.value = ''
    error.value = ''
    stage.value = 'closed'
  }

  function dismiss() {
    if (stage.value !== 'opening') close()
  }

  function navigate(intent: HomeOrderIntent) {
    if (stage.value !== 'ready' || !base.value || !hasRecipientConsent(session)) return
    if (intent.kind === 'create' && !canCreate.value) return
    const attempt = generation
    stage.value = 'opening'
    retryIntent = intent
    function failed() {
      if (generation !== attempt || stage.value !== 'opening') return
      if (handoffID) cancelHomeOrderEntry(handoffID)
      handoffID = ''
      error.value = '录单页打开失败，请重试。'
      stage.value = 'error'
    }
    try {
      handoffID = passHomeOrderEntry(session.token, consentScope(session), base.value, intent)
      uni.navigateTo({
        url: employeeOrderEntryPath + '?home_entry=' + encodeURIComponent(handoffID),
        success() {
          if (generation !== attempt || stage.value !== 'opening') return
          handoffID = '' // The order page owns the one-time handoff after navigation.
          close()
        },
        fail: failed,
      })
    } catch {
      failed()
    }
  }

  async function load() {
    if (stage.value !== 'consent' && stage.value !== 'error') return
    if (session.accountType !== 'employee' || !hasRecipientConsent(session)) return
    const attempt = ++generation
    const token = session.token
    stage.value = 'loading'
    base.value = undefined
    error.value = ''
    try {
      const [options, { draft }] = await Promise.all([fetchEntryBase(token), fetchEmployeeOrderDraft(token)])
      if (attempt !== generation) return
      base.value = options
      stage.value = 'ready'
      const intent = retryIntent
      retryIntent = undefined
      if (draft) navigate({ kind: 'draft', draft })
      else if (intent?.kind === 'create' && canCreate.value) navigate(intent)
      else if (intent?.kind === 'customer' && options.customers?.some(customer => customer.id === intent.customerId)) navigate(intent)
    } catch (cause) {
      if (attempt !== generation) return
      if (isAuthenticationExpiredRequestError(cause)) {
        session.clearSession()
        uni.reLaunch({ url: '/pages/login/login' })
        return
      }
      error.value = cause instanceof Error ? cause.message : '客户和草稿读取失败，请重试。'
      stage.value = 'error'
    }
  }

  function open() {
    if (stage.value !== 'closed' || session.accountType !== 'employee' || !session.token) return
    stage.value = 'consent'
    if (hasRecipientConsent(session)) void load()
  }

  watch([() => session.token, () => consentScope(session), () => session.accountType, () => session.employeeID], close, { flush: 'sync' })
  watch(() => stage.value !== 'closed' && hasRecipientConsent(session), (granted, previous) => {
    if (previous && !granted && stage.value !== 'closed') close()
  }, { flush: 'sync' })
  onHide(() => {
    // A consent link opens a legal document; preserve the prompt when the employee returns.
    if (stage.value !== 'opening' && stage.value !== 'consent') close()
  })
  onBeforeUnmount(close)
  return { stage, customers, canCreate, query, error, open, dismiss, load, navigate }
}
