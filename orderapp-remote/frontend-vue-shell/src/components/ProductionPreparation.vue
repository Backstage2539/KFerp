<template>
  <div class="preparation" aria-label="备料情况">
    <p class="preparation-note">先用 WIP 现场库存，其余生成领料建议。供给落实后可提交，领料齐套后才能开工。</p>
    <div v-if="!sources.length" class="empty">暂无备料项</div>
    <details v-for="source in sources" :id="`component-source-${source.id}`" :key="source.id" class="preparation-row" :class="{shortage: source.preparation_status === 'shortage'}">
      <summary>
        <div class="material"><strong>{{ source.component_name }}</strong><small>{{ itemName(source) }}<span v-if="source.allocation_mode === 'manual'"> · 已手工调整</span></small></div>
        <dl><div><dt>需求</dt><dd>{{ amount(source, 'demand') }}</dd></div><div><dt>WIP 已有</dt><dd>{{ amount(source, 'wip_covered') }}</dd></div><div><dt>待领</dt><dd>{{ amount(source, 'transfer') }}</dd></div><div><dt>真实缺口</dt><dd :class="{missing: source.shortage_g || source.shortage_units}">{{ amount(source, 'shortage') }}</dd></div></dl>
        <span class="state">{{ stateLabel(source) }}</span>
        <span class="expand-label">查看明细</span>
      </summary>
      <div class="preparation-detail">
        <p v-if="source.upstream_g || source.upstream_units" class="upstream">待生产入库 {{ amount(source, 'upstream') }}<span v-for="workOrder in upstreamOrders(source)" :key="workOrder.supplier_work_order_id"> · {{ workOrder.supplier_work_order_no }}</span></p>
        <p v-if="source.adjustment_message" role="status" class="warning">{{ source.adjustment_message }}</p>
        <ul class="allocations">
          <li v-for="(allocation, index) in source.allocations || []" :key="`${allocation.warehouse}-${allocation.owner_customer_id}-${index}`">
            <div><strong>{{ allocation.warehouse_name || allocation.warehouse }}</strong><small>{{ allocation.owner_name || '工厂' }} · {{ allocation.warehouse === 'wip' ? '现场已有' : '领入 WIP' }}</small></div>
            <strong>{{ quantity(allocation.qty_g, allocation.qty_units, source) }}</strong>
            <div v-if="allocation.batches?.length" class="batches"><span v-for="batch in allocation.batches" :key="batch.batch_id">{{ batch.batch_code }} · {{ quantity(batch.qty_g, batch.qty_units, source) }}</span></div>
          </li>
        </ul>
        <p v-if="!(source.allocations || []).length" class="empty">暂无可领现货{{ source.upstream_g || source.upstream_units ? '，等待生产入库' : '，请补齐供给后刷新' }}</p>
        <div v-if="editable" class="adjust-actions">
          <button v-if="editing !== source.id" type="button" :disabled="saving" @click="beginAdjustment(source)">调整来源与数量</button>
          <button v-if="source.allocation_mode === 'manual' && editing !== source.id" type="button" :disabled="saving" @click="$emit('adjust', source, null)">恢复自动建议</button>
        </div>
        <form v-if="editable && editing === source.id" class="adjustment" @submit.prevent="applyAdjustment(source)">
          <p>填写需要固定的数量，其余需求由系统自动补齐。这里保存建议，不会转仓。</p>
          <label v-for="row in edits" :key="`${row.warehouse}-${row.owner_customer_id}`">
            <span>{{ row.warehouse_name || row.warehouse }} · {{ row.owner_name || '工厂' }}<small>可用 {{ quantity(row.available_g, row.available_units, source) }}</small></span>
            <input v-model.number="row.quantity" type="number" min="0" :step="isWeight(source) ? '0.001' : '1'" :aria-label="`${row.warehouse_name || row.warehouse} 分配数量`" />
            <span>{{ displayUnit(source) }}</span>
          </label>
          <p v-if="editError" role="alert" class="missing">{{ editError }}</p>
          <div class="adjust-actions"><button type="submit" :disabled="saving">保存调整</button><button type="button" @click="editing = null">取消</button></div>
        </form>
      </div>
    </details>
    <div v-if="!editable && issueOrders.length" class="issue-orders"><strong>按工单领料</strong><button v-for="order in issueOrders" :key="order.id" type="button" @click="$emit('issue', order)">去领料 · {{ order.work_order_no }}</button></div>
  </div>
</template>
<script setup>
import { computed, ref } from 'vue'
const props = defineProps({sources:{type:Array,default:()=>[]},items:{type:Array,default:()=>[]},supplyAllocations:{type:Array,default:()=>[]},workOrders:{type:Array,default:()=>[]},editable:{type:Boolean,default:false},saving:{type:Boolean,default:false}})
const emit=defineEmits(['adjust','issue'])
const editing=ref(null), edits=ref([]), editError=ref('')
const isWeight=(s)=>Number(s.demand_g || s.required_g || 0)>0
const displayUnit=(s)=>isWeight(s)?(['kg','g'].includes(s.unit)?s.unit:'g'):(s.unit||'件')
const quantity=(g,n,s)=>`${Number((isWeight(s)?Number(g||0)/(displayUnit(s)==='kg'?1000:1):Number(n||0)).toFixed(6))} ${displayUnit(s)}`
const amount=(s,key)=>quantity(s[`${key}_g`] ?? (key==='demand'?s.required_g:0),s[`${key}_units`] ?? (key==='demand'?s.required_units:0),s)
const itemName=(s)=>{const i=props.items.find(i=>Number(i.id)===Number(s.production_plan_item_id));return i?.output_name||i?.product_name||''}
const stateLabel=(s)=>s.preparation_status==='shortage'?'缺料':s.preparation_status==='awaiting_production'?'待生产入库':s.preparation_status==='awaiting_transfer'?'供给已落实':'现场已齐套'
const upstreamOrders=(s)=>props.supplyAllocations.filter(a=>Number(a.production_plan_item_id)===Number(s.production_plan_item_id)&&Number(a.material_id)===Number(s.component_id))
const issueOrders=computed(()=>props.workOrders.filter(w=>['released','running','partially_completed'].includes(w.status)&&props.sources.some(s=>Number(s.production_plan_item_id)===Number(w.production_plan_item_id)&&(s.transfer_g>0||s.transfer_units>0))))
function beginAdjustment(source){editing.value=source.id;editError.value='';edits.value=(source.options||[]).map(o=>{const a=(source.manual_allocations||[]).find(a=>a.warehouse===o.warehouse&&a.owner_customer_id===o.owner_customer_id);return {...o,quantity:a?Number(isWeight(source)?a.qty_g/(displayUnit(source)==='kg'?1000:1):a.qty_units):0}})}
function applyAdjustment(source){
 const allocations=edits.value.filter(r=>Number(r.quantity)>0).map(r=>({warehouse:r.warehouse,owner_customer_id:r.owner_customer_id,qty_g:isWeight(source)?Math.round(Number(r.quantity)*(displayUnit(source)==='kg'?1000:1)):0,qty_units:isWeight(source)?0:Number(r.quantity)}))
 if(edits.value.some(r=>!Number.isFinite(Number(r.quantity))||Number(r.quantity)<0)||allocations.some(a=>!Number.isInteger(a.qty_units))){editError.value='请填写有效数量，计件物料须为整数';return}
 if(allocations.reduce((v,a)=>v+a.qty_g,0)>Number(source.required_g||0)||allocations.reduce((v,a)=>v+a.qty_units,0)>Number(source.required_units||0)){editError.value='固定数量不能超过本项待落实需求';return}
 emit('adjust',source,allocations);editing.value=null
}
</script>
<style scoped>
.preparation{font-size:14px;color:#18352e;min-width:0}.preparation-note{color:#657771;font-size:13px;margin:0 0 16px;line-height:1.6}.preparation-row{border:1px solid #dfe7e3;border-radius:10px;margin:9px 0;background:#fff;scroll-margin-top:220px}.preparation-row.shortage{border-color:#eab88c}.preparation-row summary{cursor:pointer;display:grid;grid-template-columns:minmax(120px,1fr) minmax(240px,2fr) auto;gap:14px;padding:15px;align-items:center;list-style:none}.preparation-row summary::-webkit-details-marker{display:none}.material{display:grid;gap:5px;overflow-wrap:anywhere}.material small,dt,.expand-label{font-size:12px;color:#657771}.preparation-row dl{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin:0}.preparation-row dd{margin:5px 0 0;font-weight:700;white-space:nowrap}.state{font-size:12px;color:#196145;background:#eaf5ef;border-radius:20px;padding:5px 9px;white-space:nowrap}.shortage .state{background:#fff1e2;color:#9e501a}.expand-label{grid-column:1/-1;text-align:right;line-height:1}.preparation-row[open] .expand-label{display:none}.preparation-detail{padding:0 15px 16px;border-top:1px solid #eef1ef}.allocations{list-style:none;margin:0;padding:0}.allocations li{display:grid;grid-template-columns:1fr auto;gap:8px;padding:12px 0;border-bottom:1px solid #eef1ef}.allocations small{display:block;color:#657771;margin-top:4px}.batches{grid-column:1/-1;display:flex;gap:6px;flex-wrap:wrap;color:#657771;font-size:12px}.batches span{padding:4px 8px;border-radius:4px;background:#f5f8f6}.missing,.warning{color:#a34a1a}.upstream{padding:10px;background:#f4f7f5}.adjust-actions,.issue-orders{display:flex;gap:10px;flex-wrap:wrap;margin-top:12px}.preparation button{padding:8px 12px;background:#fff;border:1px solid #c5d8ce;border-radius:6px;color:#1e6a50;cursor:pointer}.preparation button:disabled{opacity:.5;cursor:wait}.adjustment{background:#f5f8f6;margin-top:12px;padding:12px;border-radius:8px}.adjustment p{font-size:12px;color:#657771}.adjustment label{display:grid;grid-template-columns:1fr 90px 28px;align-items:center;gap:8px;margin:10px 0}.adjustment label small{display:block;color:#657771}.adjustment input{width:100%;min-width:0;box-sizing:border-box;padding:8px;border:1px solid #c5d8ce;border-radius:5px}.empty{color:#657771;padding:12px}.issue-orders{align-items:center}.preparation button:focus-visible,.preparation summary:focus-visible{outline:2px solid #1e6a50;outline-offset:3px}
@media(max-width:850px){.preparation-row summary{grid-template-columns:1fr auto}.preparation-row dl{grid-column:1/-1;grid-row:2}.state{grid-column:2;grid-row:1}.preparation-row dd{white-space:normal}.preparation-detail{padding:0 12px 12px}}
</style>
