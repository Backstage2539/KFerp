import { beforeEach, describe, expect, it, vi } from 'vitest'
vi.mock('../api/registration', () => ({ registerVisitor: vi.fn(), wechatLoginCode: vi.fn() }))
vi.mock('../api/customerPortal', () => ({ loginWithPhoneIdentify: vi.fn() }))
import { loginWithPhoneIdentify } from '../api/customerPortal'
vi.mock('../utils/registrationSession', () => ({ restoreRegistrationSession: vi.fn() }))
vi.mock('../stores/session', () => ({ useSessionStore: () => session }))
import { registerVisitor, wechatLoginCode } from '../api/registration'
import { restoreRegistrationSession } from '../utils/registrationSession'
import { useBeanRegistration } from './useBeanRegistration'

const session = { token: '', miniUserID: 0, bindings: [] as Array<{customer_id:number;status:string}>, accountType: '', roles: [] as string[], registrationComplete: false, registration: null, setToken: vi.fn(), applyContext: vi.fn() }
const card = { key: 'a'.repeat(32), name: '咖啡豆标准', visibility: 'registered', version: 'v1', updated_at: '' }
const response = { token: 'verified', registration_complete: true, current_customer_id: 0, bindings: [{customer_id: 1}, {customer_id: 2}] }
function deferred<T>() { let resolve!: (value:T)=>void; const promise = new Promise<T>(r => {resolve=r}); return {promise,resolve} }

describe('price card inline registration', () => {
  it('已登录客户连续打开权限内价格表不重复登记', async () => {
    Object.assign(session, { token: 'customer-session', miniUserID: 151, accountType: 'customer', bindings: [{ customer_id: 9, status: 'approved' }] })
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard(card); await form.openCard({ ...card, key: 'b'.repeat(32) })
    expect(navigate).toHaveBeenCalledTimes(2)
    expect(form.visible.value).toBe(false); expect(session.registrationComplete).toBe(false)
    Object.assign(session, { token: '', miniUserID: 0, bindings: [] })
  })
  beforeEach(() => { vi.resetAllMocks(); Object.assign(session, {token:'',miniUserID:0,bindings:[]}); session.registrationComplete = false; session.accountType='';session.roles=[]; vi.mocked(wechatLoginCode).mockResolvedValue('wx-code'); vi.mocked(loginWithPhoneIdentify).mockResolvedValue(response as never) })
  it('opens a sheet over the selected card without navigating to a login or permission page', async () => {
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard(card)
    expect(form.visible.value).toBe(true); expect(form.title.value).toBe(card.name); expect(navigate).not.toHaveBeenCalled()
    await form.submit({code:'phone-code'})
    expect(loginWithPhoneIdentify).toHaveBeenCalledWith('wx-code','phone-code')
    expect(session.setToken).toHaveBeenCalledWith('verified'); expect(session.applyContext).toHaveBeenCalledWith(response)
    expect(navigate).toHaveBeenCalledTimes(1); expect(navigate).toHaveBeenCalledWith(card.key); expect(form.visible.value).toBe(false)
  })
  it.each(['sales','admin'])('lets a %s employee open registered center cards without collecting phone',async role=>{session.accountType='employee';session.roles=[role];const navigate=vi.fn(),form=useBeanRegistration(navigate);await form.openCard(card);expect(navigate).toHaveBeenCalledWith(card.key);expect(form.visible.value).toBe(false);expect(loginWithPhoneIdentify).not.toHaveBeenCalled()})
  it('keeps disabled identities blocked without a registration prompt',async()=>{vi.mocked(restoreRegistrationSession).mockRejectedValue(new Error('账号已停用'));const navigate=vi.fn(),form=useBeanRegistration(navigate);await expect(form.openCard(card)).rejects.toThrow('账号已停用');expect(form.visible.value).toBe(false);expect(navigate).not.toHaveBeenCalled()})
  it('awaits restored registration and opens directly without collecting details again', async () => {
    vi.mocked(restoreRegistrationSession).mockImplementation(async () => { session.registrationComplete = true })
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard(card)
    expect(navigate).toHaveBeenCalledWith(card.key); expect(form.visible.value).toBe(false)
    expect(loginWithPhoneIdentify).not.toHaveBeenCalled()
  })
  it.each(['public','authenticated'])('leaves %s entry access to the existing page permissions', async visibility => {
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard({...card,visibility})
    expect(navigate).toHaveBeenCalledWith(card.key); expect(form.visible.value).toBe(false)
  })
  it('keeps the selected card stable on repeated taps and submits the phone credential once', async () => {
    const wait = deferred<typeof response>(); vi.mocked(loginWithPhoneIdentify).mockReturnValue(wait.promise as never)
    const navigate = vi.fn(), form = useBeanRegistration(navigate)
    await form.openCard(card); await form.openCard({...card,key:'b'.repeat(32)})
    const first = form.submit({code:'once'}); await Promise.resolve()
    await form.submit({code:'once'}); expect(loginWithPhoneIdentify).toHaveBeenCalledTimes(1)
    wait.resolve(response); await first; expect(navigate).toHaveBeenCalledTimes(1); expect(navigate).toHaveBeenCalledWith(card.key)
  })
  it('does not request a phone exchange when authorization is rejected', async () => {
    const navigate=vi.fn(), form=useBeanRegistration(navigate); await form.openCard(card)
    await form.submit({errMsg:'getPhoneNumber:fail user deny'})
    expect(loginWithPhoneIdentify).not.toHaveBeenCalled(); expect(form.visible.value).toBe(true)
    form.close(); expect(form.visible.value).toBe(false); expect(navigate).not.toHaveBeenCalled()
  })
  it('keeps failures in the sheet and allows a fresh authorization retry', async () => {
    vi.mocked(loginWithPhoneIdentify).mockRejectedValueOnce(new Error('凭证失效，请重试'))
    const navigate=vi.fn(), form=useBeanRegistration(navigate); await form.openCard(card);
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
    const wait=deferred<typeof response>(); vi.mocked(loginWithPhoneIdentify).mockReturnValue(wait.promise as never)
    const navigate=vi.fn(), form=useBeanRegistration(navigate); form.openRequired(card.key,card.name);
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

describe('phone authorization distinguishes customer from visitor', () => {
  beforeEach(() => {
    vi.resetAllMocks(); vi.stubGlobal('uni', {reLaunch: vi.fn()})
    Object.assign(session, {token:'',miniUserID:0,bindings:[],accountType:'',roles:[],registrationComplete:false})
    vi.mocked(wechatLoginCode).mockResolvedValue('wx-code')
  })
  it('sends an employee to the employee home without opening the visitor price card', async () => {
    vi.mocked(loginWithPhoneIdentify).mockResolvedValue({token:'employee',account_type:'employee',current_customer_id:0,bindings:[]} as never)
    const open=vi.fn(), form=useBeanRegistration(open)
    form.openRequired(card.key,card.name)
    await form.submit({code:'phone'})
    expect(uni.reLaunch).toHaveBeenCalledWith({url:'/pages/home/home'})
    expect(open).not.toHaveBeenCalled()
  })
  it('sends a customer to their home instead of the visitor price card', async () => {
    vi.mocked(loginWithPhoneIdentify).mockResolvedValue({token:'customer',registration_complete:true,current_customer_id:74,bindings:[{customer_id:74,status:'approved'}]} as never)
    const open=vi.fn(), form=useBeanRegistration(open)
    form.openRequired(card.key,card.name)
    await form.submit({code:'phone'})
    expect(uni.reLaunch).toHaveBeenCalledWith({url:'/pages/home/home'})
    expect(open).not.toHaveBeenCalled()
  })
  it('sends a multi-customer identity to the customer selector', async () => {
    vi.mocked(loginWithPhoneIdentify).mockResolvedValue({token:'customer',registration_complete:true,current_customer_id:0,bindings:[{customer_id:1,status:'approved'},{customer_id:2,status:'approved'}]} as never)
    const open=vi.fn(), form=useBeanRegistration(open)
    form.openRequired(card.key,card.name)
    await form.submit({code:'phone'})
    expect(uni.reLaunch).toHaveBeenCalledWith({url:'/pages/customer-select/customer-select?return_to=%2Fpages%2Fhome%2Fhome'})
    expect(open).not.toHaveBeenCalled()
  })
  it('opens the requested price card for a visitor with no customer binding', async () => {
    vi.mocked(loginWithPhoneIdentify).mockResolvedValue({token:'visitor',registration_complete:true,current_customer_id:0,bindings:[]} as never)
    const open=vi.fn(), form=useBeanRegistration(open)
    form.openRequired(card.key,card.name);
    await form.submit({code:'phone'})
    expect(open).toHaveBeenCalledWith(card.key)
    expect(uni.reLaunch).not.toHaveBeenCalled()
  })
})

describe('phone identity first', () => {
  it('does not require a nickname before identifying the phone', async () => {
    vi.resetAllMocks(); vi.stubGlobal('uni', {reLaunch: vi.fn()})
    vi.mocked(wechatLoginCode).mockResolvedValue('wx')
    vi.mocked(loginWithPhoneIdentify).mockResolvedValue({token:'customer',current_customer_id:74,bindings:[{customer_id:74,status:'approved'}]} as never)
    const open=vi.fn(), form=useBeanRegistration(open)
    form.openRequired(card.key,card.name)
    await form.submit({code:'phone'})
    expect(loginWithPhoneIdentify).toHaveBeenCalledWith('wx','phone')
    expect(registerVisitor).not.toHaveBeenCalled()
    expect(uni.reLaunch).toHaveBeenCalledWith({url:'/pages/home/home'})
  })
})
