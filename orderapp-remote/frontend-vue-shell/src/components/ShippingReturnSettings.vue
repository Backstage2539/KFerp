<template>
  <section class="return-settings">
<h2>物流回传模板</h2>
<p>订单列表上传寄件表时，按姓名和手机号前三位、后四位匹配。单号和实际成本必填，0 元有效。</p>
<p v-if="error" role="alert">{{error}}</p>
<p v-if="message" role="status">{{message}}</p>
<article v-for="t in templates" :key="t.id">
<div class="fields">
<label>模板名称<input v-model.trim="t.name" />
</label>
<label>工作表<input v-model.trim="t.sheet" />
</label>
<label>标题行<input v-model.number="t.header_row" type="number" min="1" max="100" />
</label>
<label>快递公司<select v-model.number="t.carrier_id">
<option :value="0">请选择</option>
<option v-for="c in companies" :key="c.id" :value="c.id">{{c.name}}</option>
</select>
</label>
<label>
<input v-model="t.active" type="checkbox" />启用</label>
</div>
<div class="fields">
<label v-for="(c,k) in t.columns" :key="k">{{labels[k]}}<input v-model.trim="c.column" placeholder="列字母，如 C" aria-label="列字母" />
<input v-model.trim="c.header" placeholder="该列表头文字" aria-label="表头文字" />
</label>
</div>
</article>
<button :disabled="busy" @click="save">保存回传模板</button>
<button :disabled="busy||!templates.length" @click="copy">复制为新模板</button>
</section>
</template>
<script setup>
import {onMounted,ref} from 'vue'
import {apiGet,apiSend} from '../api/client'
defineProps({companies:{type:Array,default:()=>[]}})
const templates=ref([]),error=ref(''),message=ref(''),busy=ref(false)
const labels={tracking:'运单号',amount:'实际物流成本',receiver:'收件人',phone:'收件电话',sender:'寄件人',sender_phone:'寄件电话',time:'下单时间'}
async function load(){try{templates.value=(await apiGet('/api/settings/logistics/shipping-return-templates')).templates||[]}catch(e){error.value=e.message}}
function copy(){const t=JSON.parse(JSON.stringify(templates.value[0]));t.id=`return-${Date.now()}`;t.name+='（副本）';t.revision=0;templates.value.push(t)}
async function save(){busy.value=true;error.value='';message.value='';try{const r=await apiSend('/api/settings/logistics/shipping-return-templates',{method:'PUT',body:{templates:templates.value}});templates.value=r.templates;message.value='回传模板已保存'}catch(e){error.value=e.message}finally{busy.value=false}}
onMounted(load)
</script>
<style scoped>
.return-settings{background:#fff;border:1px solid #dce5df;border-radius:8px;padding:20px;margin:18px 0}h2{font-size:20px}p{font-size:13px;color:#607568}.fields{display:flex;gap:12px;flex-wrap:wrap;margin:12px 0}label{display:grid;gap:6px;font-size:13px;max-width:185px}input,select{min-width:0;padding:8px;border:1px solid #cbd9cf;border-radius:6px}article{border-bottom:1px solid #dce5df;padding-bottom:10px;margin-bottom:12px}button{padding:8px 12px;margin-right:10px;background:#286248;color:#fff;border:0;border-radius:6px}[role=alert]{color:#a32d25}
</style>
