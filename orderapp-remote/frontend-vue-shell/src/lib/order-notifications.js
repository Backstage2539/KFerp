export const notificationEvents=[['created','下单成功'],['paid','付款成功'],['production_started','开始生产'],['production_finished','生产完成'],['shipped','发货进度'],['cancelled','订单取消']]
export const noticeSources=[['order_no','订单号'],['status','状态'],['occurred_at','通知时间'],['customer_name','客户'],['items','商品'],['due_date','交期'],['responsible_name','负责人']]
export function templateFields(content){const fields=[...String(content||'').matchAll(/\{\{([a-z_]+\d*).DATA\}\}/g)].map(m=>m[1]);return fields.includes('content')?[]:[...new Set(fields)]}
export function canRetry(status){return status==='failed'}
export function statusLabel(status){return ({ready:'待发送',sending:'发送中',accepted:'微信已受理，待回执',delivered:'已送达',retry:'等待重试',failed:'失败',unknown:'结果不明，需核对',skipped:'已停止',confirmed:'人工核对已发送'})[status]||status}
