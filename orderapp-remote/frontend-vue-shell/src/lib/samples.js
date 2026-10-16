export const sampleStatuses = {pending_assignment:'待分配',pending_contact:'收到样品工单／待联系',ordered:'已确认寄样',closed:'跟进已关闭'}
export const followupStatuses = {pending:'待回访',following:'跟进中',won:'已成交',abandoned:'已放弃'}
export function sampleStatus(row) { return row?.order_id ? (row.shipment_status || '待发货') : (sampleStatuses[row?.status] || '待分配') }
export function sampleTotal(items = [], freight = 0) { return (items.reduce((sum,item)=>sum+Math.round(Number(item.unit_price||0)*100)*Number(item.qty||0),0)+Math.round(Number(freight||0)*100))/100 }
export function confirmRows(items = [], specs = []) { return items.map(item=>({...item,included:true,bom_spec_id:specs.find(spec=>Number(spec.product_id)===Number(item.product_id))?.id||0,qty:1,unit_price:0})) }
export function canConfirmSample(row) { return !!row?.work_order_id && !row.order_id && !['won','abandoned'].includes(row.followup_status) && row.status !== 'closed' }
export function sampleNotificationNavigation(item) { const payload=item?.payload||{}; return payload.view==='sampleRequests'||item?.source_type==='sample_request' ? {key:'sampleRequests',params:{request_id:Number(payload.request_id||item.source_id||0)}} : null }

export function actorCanUseSamples(actor) { return !!actor && actor.account_type !== 'channel_customer' && (actor.basic_auth_admin || (actor.roles || []).some(role=>['admin','sales'].includes(role?.code))) }
