<template>
  <section class="remainder-panel" aria-label="包装余料">
    <div class="heading"><h3>包装余料</h3><button type="button" :disabled="busy || retry.uncertain || recheckRetry.uncertain || resolveRetry.uncertain" @click="$emit('close')">关闭</button></div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <button v-if="refreshNeeded && detail" type="button" :disabled="busy" @click="openGroup(detail)">重新核对</button>
    <p v-if="message" role="status">{{ message }}</p>
    <template v-if="!detail">
      <p>同一计划的产品全部包装完成后统一处理。</p>
      <p v-if="!groups.length && !busy">暂无待处理的包装余料。</p>
      <div v-for="group in groups" :key="`${group.plan_id}:${group.product_id}`" class="group">
        <span>{{ group.product_name }} · 计划 {{ group.plan_id }} · 已完成 {{ group.completed_tasks }}/{{ group.total_tasks }}</span>
        <button type="button" :disabled="busy" @click="openGroup(group)">选择</button>
      </div>
    </template>
    <template v-else>
      <button type="button" :disabled="busy || retry.uncertain || recheckRetry.uncertain || resolveRetry.uncertain" @click="detail=null">返回列表</button>
      <h4>{{ detail.product_name }} · 生产计划 {{ detail.plan_id }}</h4>
      <p v-if="!detail.can_confirm" role="alert">{{ detail.blocked_reason || '还有包装任务未完成，完成后才能处理余料。' }}</p>
      <button v-if="detail.recheck_required || recheckRetry.uncertain" type="button" :disabled="busy || retry.uncertain || resolveRetry.uncertain" @click="recheckRemainder">{{ recheckRetry.uncertain ? '重试重新核对' : '重新核对余料' }}</button>
      <p v-if="reweighRequired">请根据当前库存重新称重，并逐项重新填写或确认数量。</p>
      <div v-if="detail.discrepancy_g > 0" class="discrepancy" role="alert">
        <strong>差额待核对：{{ detail.discrepancy_g }} g</strong>
        <p>请到仓库盘点或查看操作日志核对；差额不会自动调整库存或损耗。</p>
        <label v-for="item in detail.discrepancies || []" :key="item.id"><span><input v-model="resolution.ids" type="checkbox" :value="item.id" :disabled="busy || resolveRetry.uncertain || retry.uncertain || recheckRetry.uncertain" /> {{ item.batch_code || '来源未记录' }} · {{ item.warehouse }} · 货主 {{ item.owner_customer_id || '本公司' }} · 待核对 {{ item.quantity_g }} g</span></label>
        <label>核对说明<textarea v-model.trim="resolution.reason" rows="2" :disabled="busy || resolveRetry.uncertain || retry.uncertain || recheckRetry.uncertain" placeholder="填写盘点或单据核对情况" /></label>
        <p>确认仅记录核对情况，不调整库存或成本。</p>
        <button type="button" :disabled="busy || refreshNeeded || retry.uncertain || recheckRetry.uncertain || (!resolveRetry.uncertain && (!resolution.ids.length || !resolution.reason))" @click="resolveDiscrepancies">{{ resolveRetry.uncertain ? '重试核对差额' : '确认核对差额' }}</button>
      </div>
      <details><summary>关联工单（{{ detail.tasks?.length || 0 }}）</summary><p v-for="task in detail.tasks" :key="task.job_card_id">{{ task.work_order_no }} · {{ task.status === 'completed' ? '已完成' : task.status }}</p></details>
      <div v-for="source in detail.sources" :key="source.source_id" class="source">
        <p>{{ source.batch_code || '来源未记录' }} · {{ source.warehouse }} · 货主 {{ source.owner_customer_id || '本公司' }} · 账面可处理 {{ source.available_g }} g</p>
        <label>实际余料（g）<input v-model="quantities[source.source_id]" type="number" min="0" :max="source.available_g" step="1" :disabled="!detail.can_confirm || busy || retry.uncertain || recheckRetry.uncertain || resolveRetry.uncertain" @input="reweighed[source.source_id]=true" /></label>
      </div>
      <label>余料去向<select v-model="target" :disabled="!detail.can_confirm || busy || retry.uncertain || recheckRetry.uncertain || resolveRetry.uncertain"><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label>
      <p>余料在 WIP 且选择留在 WIP 时不会重复入库。库存有差异时需另行核对。</p>
      <button type="button" class="primary" :disabled="!detail.can_confirm || busy || refreshNeeded || recheckRetry.uncertain || resolveRetry.uncertain || (reweighRequired && (detail.sources || []).some(s => !reweighed[s.source_id]))" @click="submit">{{ retry.uncertain ? '重试本次处理' : '确认处理' }}</button>
    </template>
  </section>
</template>
<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { completionRetry } from '../lib/operation-completion.js'
const emit=defineEmits(['close','posted'])
const groups=ref([]),detail=ref(null),warehouses=ref([{code:'wip',name:'WIP在制仓'}]),target=ref('wip'),busy=ref(false),error=ref(''),message=ref(''),refreshNeeded=ref(false)
const drafts=reactive({}),retry=reactive(completionRetry()),recheckRetry=reactive(completionRetry()),resolveRetry=reactive(completionRetry()),reweighByGroup=reactive({}),reweighedByGroup=reactive({}),resolutions=reactive({})
const groupKey=computed(()=>detail.value ? `${detail.value.plan_id}:${detail.value.product_id}` : '')
const reweighRequired=computed(()=>!!reweighByGroup[groupKey.value])
const reweighed=computed(()=>reweighedByGroup[groupKey.value] ||= {})
const resolution=computed(()=>resolutions[groupKey.value] ||= {ids:[],reason:''})
const quantities=computed(()=>{if (!groupKey.value) return {};return drafts[groupKey.value] ||= {}})
async function load(){busy.value=true;error.value='';try{const [data,ws]=await Promise.all([apiGet('/api/packaging-remainders'),apiGet('/api/stock/warehouses')]);groups.value=data.groups||[];const rows=Array.isArray(ws)?ws:(ws.rows||ws.warehouses||[]);warehouses.value=rows.filter(w=>w.active!==false&&w.code);if(!warehouses.value.some(w=>w.code==='wip'))warehouses.value.unshift({code:'wip',name:'WIP在制仓'})}catch(e){error.value=`读取余料失败：${e.message}`}finally{busy.value=false}}
async function openGroup(group){busy.value=true;error.value='';try{detail.value=await apiGet(`/api/production-plans/${group.plan_id}/packaging-remainders?product_id=${group.product_id}`);refreshNeeded.value=false;if(detail.value.reweigh_required && !reweighByGroup[groupKey.value]){reweighByGroup[groupKey.value]=true;reweighedByGroup[groupKey.value]={}}for(const source of detail.value.sources||[]){if(!(source.source_id in quantities.value))quantities.value[source.source_id]=''}}catch(e){error.value=`读取余料失败：${e.message}`}finally{busy.value=false}}
async function recheckRemainder(){busy.value=true;error.value='';try{const body=recheckRetry.begin({product_id:detail.value.product_id,expected_version:detail.value.version},()=>crypto.randomUUID());detail.value=await apiSend(`/api/production-plans/${detail.value.plan_id}/packaging-remainders/recheck`,{body});recheckRetry.clear();refreshNeeded.value=false;reweighedByGroup[groupKey.value]={};for(const source of detail.value.sources||[]){if(!(source.source_id in quantities.value))quantities.value[source.source_id]=''}reweighByGroup[groupKey.value]=true;message.value='库存已重新核对，请重新称重。'}catch(e){recheckRetry.fail(e);error.value=recheckRetry.uncertain?'尚未收到核对结果，请重试本次操作。':e.message;if(!recheckRetry.uncertain && e.status===409)refreshNeeded.value=true}finally{busy.value=false}}
async function resolveDiscrepancies(){busy.value=true;error.value='';try{const body=resolveRetry.begin({product_id:detail.value.product_id,expected_version:detail.value.version,discrepancy_ids:[...resolution.value.ids],reason:resolution.value.reason},()=>crypto.randomUUID());detail.value=await apiSend(`/api/production-plans/${detail.value.plan_id}/packaging-remainders/resolve-discrepancies`,{body});resolveRetry.clear();refreshNeeded.value=false;resolution.value.ids=[];resolution.value.reason='';reweighedByGroup[groupKey.value]={};reweighByGroup[groupKey.value]=true;for(const source of detail.value.sources||[]){if(!(source.source_id in quantities.value))quantities.value[source.source_id]=''}message.value='差额核对已记录，请重新确认实际余料。'}catch(e){resolveRetry.fail(e);error.value=resolveRetry.uncertain?'尚未收到核对结果，请重试本次操作。':e.message;if(!resolveRetry.uncertain && e.status===409)refreshNeeded.value=true}finally{busy.value=false}}
async function submit(){busy.value=true;error.value='';try{const sources=(detail.value.sources||[]).map(s=>{const raw=quantities.value[s.source_id],qty=Number(raw);if(raw===''||raw==null||!Number.isInteger(qty)||qty<0||qty>Number(s.available_g))throw new Error(`请填写 ${s.batch_code || '来源批次'} 的有效余料数量`);return {source_id:s.source_id,quantity_g:qty}});const payload=retry.begin({product_id:detail.value.product_id,expected_version:detail.value.version,target_warehouse:target.value,sources,...(detail.value.reweigh_required ? {reweigh_confirmed:true} : {})},()=>crypto.randomUUID());const result=await apiSend(`/api/production-plans/${detail.value.plan_id}/packaging-remainders`,{body:payload});retry.clear();message.value=Number(result?.discrepancy_g||0)>0?`余料处理已记录，差额 ${result.discrepancy_g} g 待核对，请到仓库盘点或查看操作日志。`:Number(result?.outstanding_g||0)>0?`余料处理已记录，仍有 ${result.outstanding_g} g 待核对。`:'余料处理已记录';delete drafts[groupKey.value];delete reweighByGroup[groupKey.value];delete reweighedByGroup[groupKey.value];detail.value=null;emit('posted');await load()}catch(e){retry.fail(e);error.value=retry.uncertain?'尚未收到处理结果，请重试本次操作。':e.message;if(!retry.uncertain && e.status===409)refreshNeeded.value=true}finally{busy.value=false}}
onMounted(load)
</script>
<style scoped>
.remainder-panel{padding:20px;background:#f7faf8;border:1px solid #d7e4dc;border-radius:12px;margin:12px 0}.heading,.group{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}.source,.group{padding:12px 0;border-bottom:1px solid #d7e4dc}label{display:grid;gap:6px;max-width:480px;margin:12px 0}input,select,button{font:inherit;padding:9px;border:1px solid #cbd6d0;border-radius:7px}.primary{background:#2d915e;color:white}.error,.discrepancy{color:#ae3427}.discrepancy{padding:12px;border-radius:8px;background:#fff3ed}button:disabled{opacity:.5}
</style>
