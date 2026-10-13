import { fetchMe, loginWithCode } from '../api/customerPortal'
import { isAuthenticationExpiredRequestError } from '../api/client'
import { wechatLoginCode } from '../api/registration'
import { useSessionStore } from '../stores/session'
let restoring:Promise<void>|null=null
export async function restoreRegistrationSession(session:ReturnType<typeof useSessionStore>){
 if(restoring)return restoring
 restoring=(async()=>{
 const known=session.registrationKnown
 if(session.token){try{session.applyContext(await fetchMe(session.token));return}catch(e){if(!isAuthenticationExpiredRequestError(e))throw e;session.clearSession(true)}}
 if(!known)return
 const result=await loginWithCode(await wechatLoginCode())
 if(result.registration_complete){session.setToken(result.token);session.applyContext(result)}
 })()
 try{await restoring}finally{restoring=null}
}
