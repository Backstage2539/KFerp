import { miniRequest } from './client'
export type SampleReceiver = { name: string; phone: string; address: string }
export type SampleItem = { key: string; product_id: number; code: string; name: string }
export type SampleActivity = { id: number; kind: string; status: string; note: string; actor: string; created_at: string }
export type SampleRequest = {
 id: number; number: string; nickname: string; verified_phone?: string; entry_key: string;
 publication_id: number; publication_name: string; publication_version: string; receiver: SampleReceiver;
 note: string; items: SampleItem[]; work_order_id: number; assignee_id: number; assignee_name: string;
 status: string; followup_status: string; revision: number; customer_id: number; order_id: number;
 order_no: string; shipment_status: string; created_at: string; next_reminder_at?: string; confirmation?: SampleConfirm; activities?: SampleActivity[]
}
export type SampleOption = { id: number; name: string }
export type SampleSpec = SampleOption & { product_id: number; unit: string }
export type SampleOptions = { customers: SampleOption[]; employees: SampleOption[]; specs: SampleSpec[]; sources: SampleOption[]; order_types: SampleOption[]; customer_types: string[]; suggested_customer_id?: number }
export type SampleSubmit = { request_id: string; entry_key: string; publication_id: number; product_keys: string[]; receiver: SampleReceiver; note: string }
export type SampleConfirmItem = { product_id: number; bom_spec_id: number; qty: number; unit_price: number; spec_name?: string; unit?: string }
export type SampleConfirm = { revision: number; customer_id: number; new_customer?: { name: string; customer_type: string; source_id: number; order_type_id: number }; receiver: SampleReceiver; items: SampleConfirmItem[]; shipping_amount: number; note: string }
export type StaffNotification = { id: number; title: string; body: string; read: boolean; source_type: string; source_id: number; payload?: {request_id?: number}; created_at: string }
const base = (staff: boolean) => staff ? '/api/mini/employee/sample-requests' : '/api/mini/sample-requests'
export const submitSample = (token: string, data: SampleSubmit) => miniRequest<SampleRequest>(base(false), {token, method:'POST',data})
export function fetchSamples(token: string, staff: boolean, query: {q?: string; status?: string; page?: number} = {}) {
 const params = Object.entries(query).filter(([,v])=>v!==undefined && v!=='').map(([k,v])=>`${k}=${encodeURIComponent(String(v))}`).join('&')
 return miniRequest<{rows:SampleRequest[];total:number;is_admin:boolean}>(`${base(staff)}?page_size=20&${params}`, {token})
}
export const fetchSample = (token: string, staff: boolean, id: number) => miniRequest<SampleRequest>(`${base(staff)}/${id}`, {token})
export const fetchSampleOptions = (token: string, id: number, q = '') => miniRequest<SampleOptions>(`${base(true)}/${id}/options?q=${encodeURIComponent(q)}`, {token})
export const assignSample = (token: string, id: number, revision: number, employee_id: number) => miniRequest<SampleRequest>(`${base(true)}/${id}/assign`, {token, method:'POST',data:{revision, employee_id}})
export const confirmSample = (token: string, id: number, data: SampleConfirm) => miniRequest<SampleRequest>(`${base(true)}/${id}/confirm`, {token, method:'POST',data})
export const followupSample = (token: string, id: number, revision: number, status: string, note: string) => miniRequest<SampleRequest>(`${base(true)}/${id}/followup`, {token, method:'POST',data:{revision,status,note}})
export const fetchStaffNotifications = (token: string, unread = false, beforeID = 0) => miniRequest<{notifications:StaffNotification[];unread_count:number}>(`/api/mini/employee/notifications${unread?'?status=unread':''}${beforeID?(unread?'&':'?')+'before_id='+beforeID:''}`,{token})
export const readStaffNotification = (token: string, id: number) => miniRequest<{ok:boolean}>(`/api/mini/employee/notifications/${id}/read`,{token,method:'POST'})
