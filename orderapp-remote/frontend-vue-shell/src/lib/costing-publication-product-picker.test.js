import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

const source = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
function codeFor(name) {
  const start = source.indexOf(`async function ${name}(`)
  assert.ok(start >= 0, `${name} 不存在`)
  const end = source.indexOf('\nasync function ', start + 1)
  return source.slice(start, end > 0 ? end : undefined)
}
function setup({ noDefault = false, saveSucceeds = true } = {}) {
  const state = { activeNamed: {value:{key:'old'}}, batch:{value:{active_table_key:'old'}}, error:{value:''}, selected:[], createCalls:0 }
  const bindings = {
    activeBeanListCustomerID:{value:517}, priceListGenerationDraftBaseKey:()=> 'scope', activePriceListTypeKey:{value:'type'},
    loadCustomerProductAliases:async()=>{}, refreshPriceListData:async()=>true, priceListRefresh:{value:{error:''}},
    pdfAvailableItems:{value:[{id:1},{id:2}]}, priceListParentProductID:item=>item.id,
    namedPriceTableBatch:state.batch, activeNamedPriceTable:state.activeNamed,
    addNamedPriceTable:async()=>{state.createCalls++;state.activeNamed.value={key:'new'};state.batch.value.active_table_key='new'},
    defaultPriceListProductSpecSelections:families=>noDefault ? [{parent_product_id:families[0].id}] : families.map(row=>({parent_product_id:row.id})),
    updatePdfProductSpecSelections:rows=>{state.selected=rows}, nextTick:async()=>{}, persistNamedPriceTableBatch:async()=>true,
    saveNamedPriceTableBatch:async()=>saveSucceeds, error:state.error,
  }
  const fn = new Function(...Object.keys(bindings), `${codeFor('createDraftFromPublicationProducts')}\nreturn createDraftFromPublicationProducts`)(...Object.values(bindings))
  return {fn,state}
}

test('来源商品缺少默认规格时不能静默漏选并报告草稿成功',async()=>{
 const {fn}=setup({noDefault:true})
 await assert.rejects(fn({source:{id:7},productIds:[1,2],tableName:'新表'}), /规格|商品/)
})
test('保存草稿未执行或失败时不能报告成功',async()=>{
 const {fn}=setup({saveSucceeds:false})
 await assert.rejects(fn({source:{id:7},productIds:[1,2],tableName:'新表'}), /保存|草稿/)
})
