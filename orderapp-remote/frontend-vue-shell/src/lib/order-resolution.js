export function blankResolution() { return { return_settlement: '', return_selections: [], action: '', reason: '', note: '', scope: '', lines: [], timing: '', external_status: '', posting_mode: '', posting_date: '', dispositions: [] } }
export function resolutionPayload(form, version, key, {draft=false}={}) {
 const dispositions=structuredClone(form.dispositions).filter(d=>form.action!=='credit'||form.scope!=='fees_only'||['fee','funds'].includes(d.kind)).map(d=>{
  if(d.choice==='replace_supply'){
   if(!draft&&!d.supplies?.length)throw new Error('请逐笔选择替代货源和数量')
   d.supplies=(d.supplies || []).map(s=>{
    const target=Number(s.target_id),quantity=Number(s.quantity)
    if(!Number.isSafeInteger(target)||target<0||!Number.isSafeInteger(quantity)||quantity<0||(!draft&&!s.converted_only&&(target===0||quantity===0)))throw new Error('请明确选择替代货源并填写预订数量')
    const out={reservation_id:Number(s.reservation_id),target_id:target,quantity}
    if(Number(s.converted_quantity)>0||s.physical_version){
     if(!draft&&s.physical_choice!=='keep_reserved')throw new Error('请明确选择已入库部分保留原批次预留')
     out.physical_choice=s.physical_choice||'';out.physical_version=s.physical_version||''
    }
    return out
   })
  }else delete d.supplies
  if(d.kind==='production'&&['allocate','scrap'].includes(d.choice)){
   const scrap=d.choice==='scrap'
   if(scrap){ d.cost_treatment=String(d.cost_treatment||'');if(!draft&&(!['retain_cost','reclassify_loss'].includes(d.cost_treatment)||!String(d.reference||'').trim()))throw new Error('请明确选择报损成本处理方式并填写报损依据') }else delete d.cost_treatment
   if(!draft&&!d.stock_selections?.length)throw new Error('请选择公司成品的原批次、目标订单和分配数量')
   d.stock_selections=(d.stock_selections||[]).map(s=>{
    const batch=Number(s.batch_id),target=scrap?0:Number(s.target_item_id),quantity=Number(s.quantity)
    if(![batch,target,quantity].every(v=>Number.isSafeInteger(v)&&v>=0)||(!draft&&(batch===0||quantity===0||(!scrap&&target===0))))throw new Error('请明确选择成品批次、目标订单并填写分配数量')
    return {batch_id:batch,target_item_id:target,quantity}
   })
  }else {delete d.stock_selections;delete d.cost_treatment}
  if(d.kind==='production'&&d.choice==='stop'){
   if(!draft&&!d.stops?.length)throw new Error('请逐工单选择余料、在制品、预留和产出处置')
   d.stops=(d.stops||[]).map(s=>{
    const row={work_order_id:Number(s.work_order_id),materials:s.materials||'',wip:s.wip||'',reservations:s.reservations||'',outputs:s.outputs||''}
    if(!draft&&(!Number.isSafeInteger(row.work_order_id)||row.work_order_id<=0||!row.materials||!row.wip||!row.reservations||!row.outputs))throw new Error('请明确选择余料、在制品、预留和产出处置')
    return row
   })
  }else delete d.stops
  if(d.kind==='fee'&&['partial_waiver','negotiated'].includes(d.choice)){
   const raw=String(d.amount??(d.amount_specified?Number(d.amount_cents)/100:'')).trim()
   if(draft&&!raw){d.amount_specified=false;d.amount_cents=0;delete d.amount;return d}
   if(!raw)throw new Error('请明确填写费用金额，零金额也须填写')
   if(!/^\d+(\.\d{1,2})?$/.test(raw))throw new Error('费用金额须为非负数，最多两位小数')
   d.amount_cents=Math.round(Number(raw)*100);if(!Number.isSafeInteger(d.amount_cents))throw new Error('金额超出范围')
   d.amount_specified=true
  }
  delete d.amount;return d
 })
 const result={ ...structuredClone(form), dispositions, lines: form.scope === 'selected' ? form.lines.filter(v => Number(v.quantity) > 0).map(v => ({ item_id: Number(v.item_id), quantity: Number(v.quantity) })) : [], version, idempotency_key: key }
 if(form.action==='return'){
  if(!draft&&!['keep_receivable','credit_returned_goods'].includes(form.return_settlement))throw new Error('请明确选择退回商品款的处理方式')
  if(!draft&&!form.return_selections?.length)throw new Error('请选择退回来源、货权、仓库和验收处置')
  result.return_selections=(form.return_selections||[]).map(row=>{
   const source=Number(row.source_id),batch=Number(row.batch_id),quantity=Number(row.quantity)
   if(![source,batch,quantity].every(v=>Number.isSafeInteger(v)&&v>=0)||(!draft&&([source,batch,quantity].some(v=>v===0)||!['shipment','ownership'].includes(row.source_kind)||!['company','customer'].includes(row.ownership)||!row.warehouse||!['restock','quarantine'].includes(row.quality))))throw new Error('请明确填写退回批次、数量、货权、仓库与验收处置')
   return {source_kind:row.source_kind||'',source_id:source,batch_id:batch,quantity,ownership:row.ownership||'',warehouse:row.warehouse||'',quality:row.quality||''}
  })
 }else{delete result.return_selections;delete result.return_settlement}
 return result
}
export function resolutionStatus(status, paused) { const label = ({ open: '正常', partially_closed: '部分关闭', closed: '已关闭', cancelled: '已取消' })[status] || '正常'; return paused ? `已暂停 · ${label}` : label }
export function blankResolutionStep() { return { choice: '', amount: '', account: '', reference: '', note: '', target_id: 0, posting_date: '' } }
export function resolutionStepPayload(form, key) {
 const raw = String(form.amount ?? '').trim()
 if (raw && !/^\d+(\.\d{1,2})?$/.test(raw)) throw new Error('金额须为非负数，最多两位小数')
 const amount = raw ? Math.round(Number(raw) * 100) : 0
 if (!Number.isSafeInteger(amount)) throw new Error('金额超出范围')
 return { choice: form.choice, amount_cents: amount, account: form.account.trim(), reference: form.reference.trim(), posting_date: form.posting_date, note: form.note.trim(), target_id: Number(form.target_id || 0), idempotency_key: key }
}
export function resolutionDependencyStatus(value) { return ({draft:'草稿',submitted:'已提交',released:'已释放',cancelled:'已取消',completed:'已完成',stopped:'已停止剩余生产',running:'生产中',reserved:'已预订',active:'有效',unsettled:'未结算',settled:'已结算',confirmed:'已确认'})[value] || value }
