import { fetchMe, loginWithCode } from '../api/customerPortal'
import { isAuthenticationExpiredRequestError } from '../api/client'
import { wechatLoginCode } from '../api/registration'
import { useSessionStore } from '../stores/session'
let restoring: Promise<void> | null = null
export async function restoreRegistrationSession(session: ReturnType<typeof useSessionStore>) {
  if (session.autoRestoreBlocked) return
  if (restoring) return restoring
  restoring = (async () => {
    if (session.token) {
      const expectedToken = session.token
      try {
        const context = await fetchMe(expectedToken)
        if (!session.autoRestoreBlocked && session.token === expectedToken) session.applyContext(context)
        return
      } catch (cause) {
        if (session.autoRestoreBlocked || session.token !== expectedToken) return
        if (!isAuthenticationExpiredRequestError(cause)) throw cause
        session.clearSession(true)
        // Preview expiry restores the original employee token, but its context
        // was cleared. Revalidate it without allowing a late response to replace
        // a newer login or an explicit logout.
        if (session.token && session.token !== expectedToken) {
          const employeeToken = session.token
          try {
            const employeeContext = await fetchMe(employeeToken)
            if (!session.autoRestoreBlocked && session.token === employeeToken) session.applyContext(employeeContext)
          } catch (employeeError) {
            if (session.autoRestoreBlocked || session.token !== employeeToken) return
            if (isAuthenticationExpiredRequestError(employeeError)) session.clearSession(true)
            throw employeeError
          }
          return
        }
      }
    }
    const expectedToken = session.token
    try {
      const code = await wechatLoginCode()
      if (session.autoRestoreBlocked || session.token !== expectedToken) return
      const result = await loginWithCode(code)
      if (session.autoRestoreBlocked || session.token !== expectedToken) return
      if (result.registration_complete || (result.bindings || []).some(binding => binding.status === 'approved')) {
        session.setToken(result.token)
        session.applyContext(result)
      }
    } catch (cause) {
      if (!session.autoRestoreBlocked && session.token === expectedToken && session.registrationKnown) throw cause
    }
  })()
  try { await restoring } finally { restoring = null }
}
