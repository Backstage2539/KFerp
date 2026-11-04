const factor = unit => ({g:1,克:1,kg:1000,千克:1000,公斤:1000,mg:.001,毫克:.001})[String(unit).toLowerCase()] || 0
export function startSuggestedMaterials(preview,input) {
 return (preview.materials||[]).map(m=>{
  const scale=Number(preview.input_qty)>0?Number(input)/Number(preview.input_qty):0
  const q=preview.recipe_ratio_pct>0 ? Number(input)*factor(preview.input_unit)*Number(m.ratio_pct)/100/factor(m.unit) : Number(m.qty)*scale
  return {...m,qty:factor(m.unit)?Math.round(q*factor(m.unit))/factor(m.unit):Math.ceil(q-1e-9)}
 })
}
export function startDraft(preview) {return {actual_input_qty:Number(preview.input_qty||0),materials:(preview.materials||[]).map(m=>({...m})),difference_reason:''}}
export function startPayload(preview,draft,id) {
 const qty=Number(draft.actual_input_qty)
 if(!Number.isFinite(qty)||qty<=0||(!factor(preview.input_unit)&&!Number.isInteger(qty)))throw new Error('请填写有效的本批实际投入数量')
 const expected=startSuggestedMaterials(preview,qty)
 if(draft.materials.length!==expected.length)throw new Error('用料清单已变化，请刷新')
 let changed=false,total=0
 const materials=expected.map((m,i)=>{
  const value=Number(draft.materials[i].qty)
  if(!Number.isFinite(value)||value<0||(!factor(m.unit)&&!Number.isInteger(value)))throw new Error('请填写有效的物料数量')
  changed ||= Math.abs(value-m.qty)>1e-9
  total+=value*factor(m.unit)
  return {...m,qty:value}
 })
 if(preview.recipe_ratio_pct>0 && Math.abs(total-qty*factor(preview.input_unit))>1e-6)throw new Error('各项实际用料合计必须等于本批投入')
 if(changed&&!String(draft.difference_reason||'').trim())throw new Error('实际用料与建议不同，请填写差异原因')
 return {request_id:id,expected_version:preview.version,input_unit:preview.input_unit,actual_input_qty:qty,materials,difference_reason:draft.difference_reason||''}
}
