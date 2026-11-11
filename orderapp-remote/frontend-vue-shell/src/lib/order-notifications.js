export const notificationEvents=[['created','下单成功'],['paid','付款成功'],['production_started','开始生产'],['production_finished','生产完成'],['shipped','发货进度'],['cancelled','订单取消']]
export const officialTestEvents=[...notificationEvents,['order_accepted','订单已接单'],['shipment_created','订单发货'],['receipt_received','货款到账'],['bill_issued','对账单生成']]
export const officialConfigEvents=officialTestEvents
export const noticeSources=[['order_no','订单号'],['status','状态'],['occurred_at','通知时间'],['customer_name','客户'],['items','商品'],['tracking_no','快递单号'],['receipt_date','实际到账日期'],['receivable_amount','关联应收总额'],['receipt_no','收款单号'],['received_amount','本次到账金额'],['bill_no','账单号'],['bill_amount','账单金额'],['bill_period','账期'],['due_date','交期'],['responsible_name','负责人']]
export function normalizeNoticeConfig(config){return {...config,events:[...(config.events||[])],official_events:[...(config.official_events??config.events??[])],templates:config.templates||{}}}
export function approvedEnumValues(value){return [...new Set(String(value||'').split(/\r?\n/).map(v=>v.trim()).filter(Boolean))]}
export function templateFields(content){const fields=[...String(content||'').matchAll(/\{\{([a-z_]+\d*).DATA\}\}/g)].map(m=>m[1]);return fields.includes('content')?[]:[...new Set(fields)]}
export function canRetry(status){return status==='failed'}
export function canVerifyBusinessTest(delivery){return delivery?.status==='delivered' && delivery.channel==='wechat_service_account' && delivery.notice?.test===true && delivery.notice.test_detail===true && delivery.notice.milestones?.length===1 && ['order_accepted','shipment_created','receipt_received','bill_issued'].includes(delivery.notice.milestones[0])}
export function statusLabel(status){return ({ready:'待发送',sending:'发送中',accepted:'微信已受理，待回执',delivered:'已送达',retry:'等待重试',failed:'失败',unknown:'结果不明，需核对',skipped:'已停止',confirmed:'人工核对已发送'})[status]||status}
