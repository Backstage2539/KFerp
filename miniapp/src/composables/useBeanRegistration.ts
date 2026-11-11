import { ref } from 'vue'
import { canUseEmployeePriceCenter, hasAuthenticatedCustomerSession } from '../utils/priceCenterAccess'
import { wechatLoginCode, type BeanCard } from '../api/registration'
import { loginWithPhoneIdentify } from '../api/customerPortal'
import { useSessionStore } from '../stores/session'
import { restoreRegistrationSession } from '../utils/registrationSession'
import { phoneVerifiedDestination } from '../utils/customerSwitch'

// Keep the selected entry on its original page throughout registration.
export function useBeanRegistration(openEntry: (key: string) => void) {
  const session = useSessionStore()
  const visible = ref(false), title = ref(''), error = ref('')
  const busy = ref(false), restoring = ref(false)
  let target = '', generation = 0

  function close() {
    generation++
    visible.value = false
    busy.value = false
    restoring.value = false
    target = ''
  }
  function openRequired(key: string, name = '查看价格表') {
    if (visible.value) return
    generation++
    target = key
    title.value = name
    error.value = ''
    visible.value = true
  }
  async function openCard(card: BeanCard) {
    if (visible.value || restoring.value) return
    if (card.visibility !== 'registered') { openEntry(card.key); return }
    const request = ++generation
    restoring.value = true
    try {
      await restoreRegistrationSession(session)
      if (request !== generation) return
      if (session.registrationComplete || canUseEmployeePriceCenter(session) || hasAuthenticatedCustomerSession(session)) openEntry(card.key)
      else openRequired(card.key, card.name)
    } finally {
      // openRequired advances the generation, but still finishes this restore.
      if (request === generation || visible.value) restoring.value = false
    }
  }
  async function submit(detail: { code?: string; errMsg?: string }) {
    if (!visible.value || busy.value) return
    if (!detail.code) { error.value = '未授权手机号，可重试或暂不查看'; return }
    const request = generation, key = target
    busy.value = true
    error.value = ''
    try {
      const code = await wechatLoginCode()
      if (request !== generation) return
      const result = await loginWithPhoneIdentify(code, detail.code)
      if (request !== generation) return
      if (!result.token) throw new Error('身份验证未完成，请重新授权手机号')
      session.setToken(result.token)
      session.applyContext(result)
      close()
      const identified = result.account_type === 'employee' || (result.bindings || []).some(binding => binding.status === 'approved')
      if (identified) uni.reLaunch({ url: phoneVerifiedDestination(result) })
      else openEntry(key)
    } catch (cause) {
      if (request === generation) error.value = cause instanceof Error ? cause.message : '登记失败，请重试'
    } finally {
      if (request === generation) busy.value = false
    }
  }
  return { visible, title, error, busy, restoring, openCard, openRequired, close, submit }
}
