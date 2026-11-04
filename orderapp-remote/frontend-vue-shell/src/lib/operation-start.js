const factor = unit => ({g:1,gram:1,grams:1,克:1,kg:1000,kilogram:1000,kilograms:1000,千克:1000,公斤:1000,lb:453.59237,lbs:453.59237,pound:453.59237,pounds:453.59237,磅:453.59237,oz:28.349523125,ounce:28.349523125,ounces:28.349523125,盎司:28.349523125})[String(unit).trim().toLowerCase()] || 0
const rounded = n => Math.round(n*1e8)/1e8
export function startSuggestedMaterials(preview,input) {
 const materials=preview.materials||[]
 if(preview.recipe_ratio_pct>0){
  const total=Math.round(Number(input)*factor(preview.input_unit)),ratio=materials.reduce((sum,m)=>sum+Number(m.ratio_pct||0),0)
  const parts=materials.map((m,i)=>{const amount=total*Number(m.ratio_pct||0)/ratio,grams=Math.floor(amount);return {i,grams,fraction:amount-grams}})
  let left=total-parts.reduce((sum,p)=>sum+p.grams,0)
  for(const p of [...parts].sort((a,b)=>b.fraction-a.fraction||a.i-b.i)){if(left--<=0)break;p.grams++}
  return materials.map((m,i)=>({...m,qty:rounded(parts[i].grams/factor(m.unit))}))
 }
 return materials.map(m=>{const scale=Number(preview.input_qty)>0?Number(input)/Number(preview.input_qty):0,q=Number(m.qty)*scale;return {...m,qty:factor(m.unit)?rounded(Math.round(q*factor(m.unit))/factor(m.unit)):Math.ceil(q-1e-9)}})
}
export function startDraft(preview) {return {actual_input_qty:Number(preview.input_qty||0),materials:(preview.materials||[]).map(m=>({...m})),difference_reason:''}}
export function startPayload(preview,draft,id) {
 const qty=Number(draft.actual_input_qty)
 if(!Number.isFinite(qty)||qty<=0||(!factor(preview.input_unit)&&!Number.isInteger(qty)))throw new Error('请填写有效的本批实际投入数量')
 const expected=startSuggestedMaterials(preview,qty)
 if(draft.materials.length!==expected.length)throw new Error('用料清单已变化，请刷新')
 let changed=Math.abs(qty-Number(preview.input_qty||0))>1e-9,total=0
 const materials=expected.map((m,i)=>{
  const value=Number(draft.materials[i].qty)
  if(!Number.isFinite(value)||value<0||(!factor(m.unit)&&!Number.isInteger(value))||(factor(m.unit)&&value>0&&Math.round(value*factor(m.unit))<1))throw new Error('请填写有效的物料数量')
  changed ||= Math.abs(value-m.qty)>1e-9
  total+=Math.round(value*factor(m.unit))
  return {...m,qty:value}
 })
 if(preview.recipe_ratio_pct>0 && total!==Math.round(qty*factor(preview.input_unit)))throw new Error('各项实际用料合计必须等于本批投入')
 if(changed&&!String(draft.difference_reason||'').trim())throw new Error('实际用料与建议不同，请填写差异原因')
 return {request_id:id,expected_version:preview.version,input_unit:preview.input_unit,actual_input_qty:qty,materials,difference_reason:draft.difference_reason||''}
}
