import { miniRequest } from './client'
import type { LoginResponse,MeResponse } from './customerPortal'
export type RegistrationProfile={nickname:string;verified_phone:string;verified_at:string;created_at:string;last_seen_at:string}
export type BeanCard={key:string;name:string;visibility:string;version:string;updated_at:string}
export function registerVisitor(code:string,phoneCode:string,nickname:string){return miniRequest<LoginResponse>('/api/mini/registration',{method:'POST',data:{code,phone_code:phoneCode,nickname:nickname.trim()}})}
export function updateRegistration(token:string,nickname:string){return miniRequest<MeResponse>('/api/mini/registration',{method:'PUT',token,data:{nickname:nickname.trim()}})}
export function fetchBeanCenter(){return miniRequest<{rows:BeanCard[]}>('/api/mini/bean-center')}
export function wechatLoginCode():Promise<string>{return new Promise((resolve,reject)=>uni.login({provider:'weixin',success:r=>r.code?resolve(r.code):reject(new Error('未取得微信登录凭证')),fail:()=>reject(new Error('微信登录失败，请重试'))}))}
