<template>
  <section class="shipping-fee-panel">
    <div class="heading"><div><h2>运费模板</h2><p>按收件地区、商品净重和包装附加重计算；不足一个续重单位按一个计费。</p></div><button type="button" @click="newTemplate">新增模板</button></div>
    <div v-if="error" class="error" role="alert">{{ error }}</div><div v-if="message" class="ok" role="status">{{ message }}</div>
    <div class="layout">
      <div class="templates"><button v-for="row in templates" :key="row.id" type="button" class="template" :class="{ selected: form.id === row.id }" @click="edit(row)"><strong>{{ row.name }}</strong><span>V{{ row.revision }} · {{ row.active ? '启用' : '停用' }}</span></button><p v-if="!templates.length">尚无运费模板</p></div>
      <form class="editor" @submit.prevent="save">
        <div class="fields"><label>模板名称<input v-model.trim="form.name" required placeholder="如 云南客户代发" /></label><label>每单包装附加重（克）<input v-model.number="form.packaging_weight_g" type="number" min="0" step="1" required /></label><label class="check"><input v-model="form.active" type="checkbox" />启用模板</label></div>
        <div class="rule-head"><h3>地区运费规则</h3><button type="button" class="secondary" @click="addRule">添加地区</button></div>
        <p class="hint">省份填写完整名称，输入 * 表示全国默认。城市、区县留空表示该省全部；同一地区优先使用最具体的规则。免费运费请明确填写 0。</p>
        <div class="rules-scroll"><table><thead><tr><th>省份</th><th>城市（可选）</th><th>区县（可选）</th><th>首重（克）</th><th>首费（元）</th><th>续重（克）</th><th>续费（元）</th><th>操作</th></tr></thead><tbody><tr v-for="(rule, index) in form.rules" :key="index"><td><input v-model.trim="rule.province" aria-label="省份" placeholder="云南省 / *" required /></td><td><input v-model.trim="rule.city" aria-label="城市" placeholder="全部城市" /></td><td><input v-model.trim="rule.district" aria-label="区县" placeholder="全部区县" /></td><td><input v-model.number="rule.first_weight_g" aria-label="首重克数" type="number" min="1" step="1" required /></td><td><input v-model="rule.first_fee" aria-label="首费元" type="number" min="0" step="0.01" required /></td><td><input v-model.number="rule.additional_weight_g" aria-label="续重克数" type="number" min="1" step="1" required /></td><td><input v-model="rule.additional_fee" aria-label="续费元" type="number" min="0" step="0.01" required /></td><td><button type="button" class="remove" :disabled="form.rules.length === 1" @click="form.rules.splice(index, 1)">删除</button></td></tr></tbody></table></div>
        <p class="hint">计费重＝商品规格净重×包装件数＋每单包装附加重；运费＝首费＋向上取整((计费重−首重)÷续重)×续费（首重以内仅收首费）。</p>
        <div class="actions"><button type="submit" :disabled="saving || loading">{{ saving ? '保存中…' : '保存运费模板' }}</button><button type="button" class="secondary" @click="load" :disabled="saving">刷新</button></div>
      </form>
    </div>
  </section>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { shippingTemplateForm, shippingTemplatePayload } from '../lib/fulfillment-settings'
const templates = ref([]), form = ref(shippingTemplateForm()), error = ref(''), message = ref(''), saving = ref(false), loading = ref(false)
function edit(row) { form.value = shippingTemplateForm(row); error.value = ''; message.value = '' }
function newTemplate() { edit({}) }
function addRule() { form.value.rules.push(shippingTemplateForm().rules[0]) }
async function load() { loading.value = true; error.value = ''; try { const data = await apiGet('/api/settings/logistics/shipping-templates'); templates.value = data.templates || []; if (form.value.id) { const current = templates.value.find(r => r.id === form.value.id); if (current) form.value = shippingTemplateForm(current) } } catch (err) { error.value = err.message || '运费模板读取失败' } finally { loading.value = false } }
async function save() { saving.value = true; error.value = ''; message.value = ''; try { const payload = shippingTemplatePayload(form.value); const saved = await apiSend(`/api/settings/logistics/shipping-templates${payload.id ? `/${payload.id}` : ''}`, { method: payload.id ? 'PUT' : 'POST', body: payload }); form.value = shippingTemplateForm(saved); await load(); message.value = '运费模板已保存，之后的新报价使用此版本' } catch (err) { error.value = err.message || '运费模板保存失败' } finally { saving.value = false } }
onMounted(load)
</script>
<style scoped>
.shipping-fee-panel{margin-top:18px;padding:18px;border:1px solid #dce5df;border-radius:8px;background:#fff;max-width:1440px;color:#24372b}.heading,.rule-head,.actions{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:14px}.heading p,.hint{color:#66786c;font-size:12px;line-height:1.65}h2,h3,p{margin:0 0 8px}h2{font-size:20px}h3{font-size:15px}.layout{display:grid;grid-template-columns:210px minmax(0,1fr);gap:20px}.templates{display:flex;flex-direction:column;gap:8px;border-right:1px solid #e2e9e3;padding-right:14px}.template{display:grid;gap:6px;text-align:left;background:#fff;color:#294c37;border-color:#d8e4da;white-space:normal}.template span{font-size:12px;color:#7a887e}.template.selected{background:#eef6ef;border-color:#3d7652}.fields{display:grid;grid-template-columns:1fr 1fr auto;align-items:center;gap:12px;margin-bottom:18px}label{display:grid;gap:7px;font-size:12px}.check{display:flex;align-items:center}.check input{width:16px}input{width:100%;box-sizing:border-box;min-height:36px;min-width:0;padding:7px;border:1px solid #cbd9ce;border-radius:6px}.rules-scroll{overflow-x:auto;margin:12px 0}table{border-collapse:collapse;width:100%;min-width:850px;font-size:12px}th,td{padding:8px 5px;border-bottom:1px solid #e4ece5;text-align:left}th{color:#57695b;background:#f5f8f4}td input{min-width:85px}button{min-height:36px;padding:7px 12px;border:1px solid #2f6c48;border-radius:6px;background:#2f6c48;color:#fff;font:inherit;cursor:pointer}button:disabled{opacity:.5;cursor:default}.secondary{background:#fff;color:#2f6c48}.remove{background:#fff;color:#9b3f34;border-color:#dbbdb7}.actions{justify-content:flex-start;margin-top:14px}.error,.ok{padding:10px;border-radius:6px;font-size:13px;margin-bottom:12px}.error{background:#fff1ee;color:#a82c20}.ok{background:#edf7ee;color:#286442}@media(max-width:900px){.layout{grid-template-columns:1fr}.templates{border-right:0;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));padding-right:0}.fields{grid-template-columns:1fr}.heading{align-items:flex-start}}
</style>
