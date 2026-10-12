import { beforeEach, describe, expect, it, vi } from 'vitest'
vi.mock('../api/registration', () => ({ registerVisitor: vi.fn(), wechatLoginCode: vi.fn() }))
vi.mock('../utils/registrationSession', () => ({ restoreRegistrationSession: vi.fn() }))
vi.mock('../stores/session', () => ({ useSessionStore: () => session }))
import { registerVisitor, wechatLoginCode } from '../api/registration'
import { restoreRegistrationSession } from '../utils/registrationSession'
import { useBeanRegistration } from './useBeanRegistration'

const session = { registrationComplete: false, registration: null, setToken: vi.fn(), applyContext: vi.fn() }
const card = { key: 'a'.repeat(32), name: '咖啡豆标准', visibility: 'registered', version: 'v1', updated_at: '' }
const response = { token: 'verified', registration_complete: true, current_customer_id: 0, bindings: [{customer_id: 1}, {customer_id: 2}] }
function deferred<T>() { let resolve!: (value:T)=>void; const promise = new Promise<T>(r => {resolve=r}); return {promise,resolve} }

describe('price card inline registration', () => {
  beforeEach(() => { vi.resetAllMocks(); session.registrationComplete = false; vi.mocked(wechatLoginCode).mockResolvedValue('wx-code'); vi.mocked(registerVisitor).mockResolvedValue(response as never) })
  it('opens a sheet over the selected card without navigating to a login or permission page', async () => {
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard(card)
    expect(form.visible.value).toBe(true); expect(form.title.value).toBe(card.name); expect(navigate).not.toHaveBeenCalled()
    form.nickname.value = ' 微信昵称 '
    await form.submit({code:'phone-code'})
    expect(registerVisitor).toHaveBeenCalledWith('wx-code','phone-code','微信昵称')
    expect(session.setToken).toHaveBeenCalledWith('verified'); expect(session.applyContext).toHaveBeenCalledWith(response)
    expect(navigate).toHaveBeenCalledTimes(1); expect(navigate).toHaveBeenCalledWith(card.key); expect(form.visible.value).toBe(false)
  })
  it('awaits restored registration and opens directly without collecting details again', async () => {
    vi.mocked(restoreRegistrationSession).mockImplementation(async () => { session.registrationComplete = true })
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard(card)
    expect(navigate).toHaveBeenCalledWith(card.key); expect(form.visible.value).toBe(false)
    expect(registerVisitor).not.toHaveBeenCalled()
  })
  it.each(['public','authenticated'])('leaves %s entry access to the existing page permissions', async visibility => {
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard({...card,visibility})
    expect(navigate).toHaveBeenCalledWith(card.key); expect(form.visible.value).toBe(false)
  })
  it('keeps the selected card stable on repeated taps and submits the phone credential once', async () => {
    const wait = deferred<typeof response>(); vi.mocked(registerVisitor).mockReturnValue(wait.promise as never)
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard(card); await form.openCard({...card,key:'b'.repeat(32)})
    form.nickname.value='访客'; const first = form.submit({code:'once'}); await Promise.resolve()
    await form.submit({code:'once'}); expect(registerVisitor).toHaveBeenCalledTimes(1)
    wait.resolve(response); await first; expect(navigate).toHaveBeenCalledTimes(1); expect(navigate).toHaveBeenCalledWith(card.key)
  })
  it('does not request a phone exchange with empty nickname or rejected authorization', async () => {
    const navigate=vi.fn(), form=useBeanRegistration(navigate); await form.openCard(card)
    await form.submit({code:'phone'}); expect(form.error.value).toContain('昵称')
    form.nickname.value='访客'; await form.submit({errMsg:'getPhoneNumber:fail user deny'})
    expect(registerVisitor).not.toHaveBeenCalled(); expect(form.visible.value).toBe(true)
    form.close(); expect(form.visible.value).toBe(false); expect(navigate).not.toHaveBeenCalled()
  })
  it('keeps failures in the sheet and allows a fresh authorization retry', async () => {
    vi.mocked(registerVisitor).mockRejectedValueOnce(new Error('凭证失效，请重试'))
    const navigate=vi.fn(), form=useBeanRegistration(navigate); await form.openCard(card); form.nickname.value='访客'
    await form.submit({code:'old'}); expect(form.error.value).toContain('凭证失效'); expect(form.busy.value).toBe(false)
    await form.submit({code:'fresh'}); expect(navigate).toHaveBeenCalledTimes(1)
  })
  it('does not reopen or navigate when the user leaves during session restoration', async () => {
    const wait=deferred<void>(); vi.mocked(restoreRegistrationSession).mockReturnValue(wait.promise)
    const navigate=vi.fn(), form=useBeanRegistration(navigate), opening=form.openCard(card)
    form.close(); wait.resolve(); await opening
    expect(form.visible.value).toBe(false); expect(navigate).not.toHaveBeenCalled()
  })
  it('ignores a registration response arriving after cancellation', async () => {
    const wait=deferred<typeof response>(); vi.mocked(registerVisitor).mockReturnValue(wait.promise as never)
    const navigate=vi.fn(), form=useBeanRegistration(navigate); form.openRequired(card.key,card.name); form.nickname.value='访客'
    const saving=form.submit({code:'phone'}); await Promise.resolve(); form.close(); wait.resolve(response); await saving
    expect(navigate).not.toHaveBeenCalled(); expect(session.setToken).not.toHaveBeenCalled()
  })
  it('stays on the directory when session restoration fails instead of asking for phone again', async () => {
    vi.mocked(restoreRegistrationSession).mockRejectedValue(new Error('网络暂不可用'))
    const navigate=vi.fn(), form=useBeanRegistration(navigate)
    await expect(form.openCard(card)).rejects.toThrow('网络暂不可用')
    expect(form.visible.value).toBe(false); expect(navigate).not.toHaveBeenCalled()
  })
})
