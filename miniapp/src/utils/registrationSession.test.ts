import {beforeEach,describe,it,expect,vi} from 'vitest'
vi.mock('../api/customerPortal',()=>({fetchMe:vi.fn(),loginWithCode:vi.fn()}))
vi.mock('../api/registration',()=>({wechatLoginCode:vi.fn()}))
import {fetchMe,loginWithCode} from '../api/customerPortal'
import {wechatLoginCode} from '../api/registration'
import {MiniRequestError} from '../api/client'
import {createPinia,setActivePinia} from 'pinia'
import {useSessionStore} from '../stores/session'
import {restoreRegistrationSession} from './registrationSession'
function session(){return {token:'expired',registrationKnown:true,clearSession:vi.fn(),setToken:vi.fn(),applyContext:vi.fn()} as unknown as Parameters<typeof restoreRegistrationSession>[0]}
describe('registration session reuse',()=>{
 beforeEach(()=>vi.resetAllMocks())
 it('restores a verified profile after expiry without another phone authorization',async()=>{
 const s=session();vi.mocked(fetchMe).mockRejectedValue(new MiniRequestError('expired',401));vi.mocked(wechatLoginCode).mockResolvedValue('new-wx-code')
 vi.mocked(loginWithCode).mockResolvedValue({token:'new-token',registration_complete:true} as never)
 await restoreRegistrationSession(s);expect(loginWithCode).toHaveBeenCalledWith('new-wx-code');expect(s.setToken).toHaveBeenCalledWith('new-token')
 })
 it('keeps the registration hint after a temporary restore failure and retries without collecting phone details',async()=>{
 vi.stubEnv('VITE_KFERP_ENVIRONMENT','development')
 vi.stubEnv('VITE_KFERP_API_BASE','https://dev.qacoohee.com/app')
 vi.stubGlobal('uni',{getStorageSync:vi.fn(),setStorageSync:vi.fn(),removeStorageSync:vi.fn()})
 setActivePinia(createPinia())
 const s=useSessionStore();s.setToken('expired');s.registrationKnown=true
 vi.mocked(fetchMe).mockRejectedValue(new MiniRequestError('expired',401))
 vi.mocked(wechatLoginCode).mockResolvedValue('wx-code')
 vi.mocked(loginWithCode).mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({token:'restored',registration_complete:true} as never)
 await expect(restoreRegistrationSession(s)).rejects.toThrow('network')
 expect(s.registrationKnown).toBe(true)
 await restoreRegistrationSession(s)
 expect(s.token).toBe('restored');expect(s.registrationComplete).toBe(true)
 vi.unstubAllGlobals();vi.unstubAllEnvs()
 })
 it('reuses a valid session and does not re-collect a profile',async()=>{
 const s=session();vi.mocked(fetchMe).mockResolvedValue({registration_complete:true} as never);await restoreRegistrationSession(s)
 expect(loginWithCode).not.toHaveBeenCalled();expect(wechatLoginCode).not.toHaveBeenCalled();expect(s.applyContext).toHaveBeenCalled()
 })
 it('does not silently restore after an explicit logout',async()=>{
 const s=session();s.token='';s.registrationKnown=false;await restoreRegistrationSession(s);expect(wechatLoginCode).not.toHaveBeenCalled()
 })
 it('does not adopt an unregistered restored identity',async()=>{
 const s=session();s.token='';vi.mocked(wechatLoginCode).mockResolvedValue('wx');vi.mocked(loginWithCode).mockResolvedValue({token:'guest',registration_complete:false} as never)
 await restoreRegistrationSession(s);expect(s.setToken).not.toHaveBeenCalled()
 })
})
