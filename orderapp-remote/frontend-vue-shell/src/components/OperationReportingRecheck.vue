<template>
  <section class="recheck" aria-label="核对工序报工类型">
    <button v-if="!preview" type="button" :disabled="busy" @click="load">{{ busy ? '正在核对…' : '核对最新报工类型' }}</button>
    <template v-else>
      <strong>确认报工类型变更</strong>
      <p v-if="!preview.changes?.length">当前工序报工类型已是最新。</p>
      <ul v-else><li v-for="change in preview.changes" :key="change.seq">{{ change.operation }}：{{ label(change.from) }} → {{ label(change.to) }}</li></ul>
      <p v-if="preview.changes?.length">确认后会清除受影响工序的原产能安排，请重新安排工位产能后再开工。</p>
      <button v-if="preview.changes?.length" type="button" :disabled="busy" @click="confirm">{{ busy ? '正在保存…' : '确认采用' }}</button>
      <button type="button" :disabled="busy" @click="preview=null">取消</button>
    </template>
    <p v-if="error" role="alert">{{ error }}</p>
  </section>
</template>
<script setup>
import {ref, computed} from 'vue'
import {apiGet, apiSend} from '../api/client'
const props=defineProps({kind:{type:String,required:true},id:{type:Number,required:true}})
const emit=defineEmits(['updated'])
const preview=ref(null),busy=ref(false),error=ref('')
const endpoint=computed(()=>props.kind==='work_order'?`/api/work-orders/${props.id}/reporting-recheck`:`/api/production-plan-items/${props.id}/reporting-recheck`)
const labels={legacy:'原规则',batch:'批次加工',incoming:'来料加工',piece:'按件加工'}
function label(value){return labels[value]||value}
async function load(){busy.value=true;error.value='';try{preview.value=await apiGet(endpoint.value)}catch(e){error.value=e.message}finally{busy.value=false}}
async function confirm(){busy.value=true;error.value='';try{await apiSend(endpoint.value,{body:{version:preview.value.version}});preview.value=null;emit('updated')}catch(e){error.value=e.message}finally{busy.value=false}}
</script>
<style scoped>
.recheck{padding:12px;border:1px solid #d9e5de;border-radius:8px;background:#f7faf8}.recheck p{margin:8px 0}.recheck ul{padding-left:20px}.recheck button{padding:8px 12px;margin-right:8px;border:1px solid #b8c9bf;background:#fff;border-radius:6px;cursor:pointer}.recheck [role=alert]{color:#a32e25}
</style>
