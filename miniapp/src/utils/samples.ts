import type { SampleConfirmItem, SampleItem, SampleRequest, SampleSpec } from '../api/samples'
export function sampleTotal(items: Array<{qty: number; unit_price: number}>, freight: number): number {
 return (items.reduce((sum, row) => sum + Math.round(Number(row.unit_price || 0)*100)*Number(row.qty || 0), 0) + Math.round(Number(freight || 0)*100))/100
}
export function sampleStatus(status: string): string { return ({pending_assignment:'待分配',pending_contact:'收到工单 · 待联系',ordered:'已确认寄样',closed:'跟进已结束'} as Record<string,string>)[status] || status }
export const followupOptions = [{value:'pending',label:'待回访'},{value:'following',label:'跟进中'},{value:'won',label:'已成交'},{value:'abandoned',label:'已放弃'}]
export function followupLabel(status: string): string { return followupOptions.find(i=>i.value===status)?.label || status }
export function confirmRows(items: Array<Pick<SampleItem,'product_id'|'name'>>, specs: SampleSpec[]): Array<SampleConfirmItem & { name: string; selected: boolean }> {
 return items.map(item=>({product_id:item.product_id,name:item.name,bom_spec_id:specs.find(s=>s.product_id===item.product_id)?.id || 0,qty:1,unit_price:0,selected:true}))
}
export function canConfirmSample(row: Pick<SampleRequest,'work_order_id'|'order_id'|'followup_status'>): boolean {return row.work_order_id>0 && !row.order_id && !['won','abandoned'].includes(row.followup_status)}
export function sampleDate(value?: string): string {if(!value)return '—';const d=new Date(value);if(!Number.isFinite(d.getTime()))return '—';const pad=(v:number)=>String(v).padStart(2,'0');return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`}
export function sampleDraftID(): string {return `sample-${Date.now().toString(36)}-${Math.random().toString(36).slice(2,12)}`}

export function validSampleMoney(value:unknown): boolean {
 if(typeof value!=='number'&&typeof value!=='string')return false
 const text=String(value).trim(), number=Number(value)
 return /^\d+(?:\.\d{1,2})?$/.test(text) && Number.isFinite(number) && number>=0 && Number.isSafeInteger(Math.round(number*100))
}
