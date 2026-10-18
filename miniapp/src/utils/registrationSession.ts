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
      }
    }
    const expectedToken = session.token
    try {
      const code = await wechatLoginCode()
      if (session.autoRestoreBlocked || session.token !== expectedToken) return
      const result = await loginWithCode(code)
      if (session.autoRestoreBlocked || session.token !== expectedToken) return
      if (result.registration_complete) {
        session.setToken(result.token)
        session.applyContext(result)
      }
    } catch (cause) {
      if (!session.autoRestoreBlocked && session.token === expectedToken) throw cause
    }
  })()
  try { await restoring } finally { restoring = null }
}
