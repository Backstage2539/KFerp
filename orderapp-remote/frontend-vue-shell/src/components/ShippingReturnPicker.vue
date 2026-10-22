<template>
  <div v-if="rows.length">
<button class="pick" @click="open=true">选择快递单（{{rows.length}}）</button>
<Teleport to="body">
<div v-if="open" class="return-mask" @click.self="open=false">
<section role="dialog" aria-modal="true" aria-label="选择快递单">
<header>
<h3>选择快递单 · {{orderNo}}</h3>
<button @click="open=false" :disabled="busy">关闭</button>
</header>
<p>选定后一起回填单号、确认运费并完成发货。请核对收寄双方及时间。</p>
<p v-if="error" role="alert">{{error}}</p>
<div class="scroll">
<table>
<thead>
<tr>
<th>下单时间</th>
<th>单号</th>
<th>收件人</th>
<th>收件电话</th>
<th>寄件人</th>
<th>寄件电话</th>
<th>实际物流成本</th>
<th>操作</th>
</tr>
</thead>
<tbody>
<tr v-for="r in rows" :key="r.id">
<td>{{r.time||'—'}}</td>
<td>{{r.tracking_no}}</td>
<td>{{r.receiver}}</td>
<td>{{r.phone}}</td>
<td>{{r.sender}}</td>
<td>{{r.sender_phone}}</td>
<td>¥{{(r.actual_cents/100).toFixed(2)}}</td>
<td>
<button :disabled="busy" @click="select(r)">使用此单</button>
</td>
</tr>
</tbody>
</table>
</div>
</section>
</div>
</Teleport>
</div>
</template>
<script setup>
import {ref} from 'vue'
import {apiSend} from '../api/client'
const props=defineProps({rows:{type:Array,default:()=>[]},orderId:{type:Number,required:true},orderNo:{type:String,default:''}})
const emit=defineEmits(['saved']),open=ref(false),busy=ref(false),error=ref('')
async function select(r){if(busy.value)return;busy.value=true;error.value='';try{await apiSend(`/api/orders/${props.orderId}/shipping-return-selection`,{body:{candidate_id:r.id,version:r.version}});open.value=false;emit('saved')}catch(e){error.value=e.message}finally{busy.value=false}}
</script>
<style scoped>
.pick{color:#286248;background:#edf6f0;border:1px solid #b7d0bf;border-radius:5px;padding:5px 8px;white-space:nowrap}.return-mask{font-family:system-ui,sans-serif;position:fixed;inset:0;z-index:1200;background:#0005;display:flex;justify-content:center;align-items:center;padding:20px}.return-mask section{background:#fff;border-radius:10px;width:1100px;max-width:95vw;padding:24px;max-height:85vh;overflow:auto}header{display:flex;align-items:center;justify-content:space-between}.scroll{overflow:auto}table{border-collapse:collapse;width:100%;min-width:900px}th,td{text-align:left;padding:12px;border-bottom:1px solid #dce5df;font-size:13px}button{cursor:pointer;padding:7px 10px}p{font-size:13px;color:#607568}[role=alert]{color:#b33428}
</style>
