<template>
  <section class="conversion-manager">
    <header><h3>规格换算</h3><button type="button" @click="$emit('close')">关闭</button></header>
    <button type="button" @click="helpOpen = !helpOpen">{{ helpOpen ? '收起说明' : '规格换算说明' }}</button>
    <OperationManualView v-if="helpOpen" view-key="specConversionManual" />
    <p>人工配置每个规格相当于多少基准单位。模板规格可供使用同一模板来源和库存单位的商品共用。</p>
    <p v-if="error" class="failure" role="alert">{{ error }}</p>
    <div class="toolbar"><input v-model="search" placeholder="搜索换算组" aria-label="搜索换算组" /><button v-if="canWrite" type="button" @click="create">新增换算组</button><button type="button" @click="load">刷新</button></div>
    <div v-for="group in pageGroups" :key="group.id" class="group-row"><div><b>{{ group.name }}</b><small>基准单位 {{ group.base_unit }} · 修订 {{ group.revision }} · {{ group.active ? '使用中' : '已停用' }}</small></div><button type="button" @click="edit(group)">{{ canWrite ? '编辑' : '查看' }}</button><button v-if="canWrite && group.active" type="button" :disabled="saving" @click="disable(group)">停用</button></div>
    <nav><button type="button" :disabled="page <= 1" @click="page--">上一页</button><span>{{ page }} / {{ pages }}</span><button type="button" :disabled="page >= pages" @click="page++">下一页</button></nav>
    <form v-if="draft" @submit.prevent="save">
      <label>名称<input v-model.trim="draft.name" required :disabled="!canWrite" /></label>
      <label>基准单位<select v-model="draft.base_unit" required :disabled="!canWrite"><option value="">请选择单位</option><option v-for="unit in catalog.units" :key="unit.code" :value="unit.code">{{ unit.name }}（{{ unit.code }}）</option></select></label>
      <label>查找可关联规格<input v-model.trim="specSearch" placeholder="商品或规格名称" :disabled="!canWrite" /></label>
      <div v-for="(member, index) in draft.members" :key="index" class="member-row">
        <label>关联规格<select :value="memberKey(member)" required :disabled="!canWrite" @change="selectMember(index, $event.target.value)"><option value="">选择明确的规格关联</option><option v-if="memberKey(member) && !memberOptions.some(o => o.key === memberKey(member))" :value="memberKey(member)">{{ member.label }}（{{ member.inventory_unit }}）</option><option v-for="option in memberOptions" :key="option.key" :value="option.key">{{ option.label }}</option></select></label>
        <label>1 {{ member.inventory_unit || '库存单位' }} =<input v-model.trim="member.base_quantity" required inputmode="decimal" placeholder="手工填写数量" :disabled="!canWrite" /><small>{{ draft.base_unit || '基准单位' }}</small></label>
        <button v-if="canWrite" type="button" @click="draft.members.splice(index, 1)">移除</button>
      </div>
      <button v-if="canWrite" type="button" @click="draft.members.push({ base_quantity: '' })">添加规格关系</button>
      <label><input v-model="draft.active" type="checkbox" :disabled="!canWrite" />启用</label>
      <p>例如 1 袋 454g = 0.454 KG，1 袋 500g = 0.5 KG。请按实际关系填写，系统不会从规格名称推算。</p>
      <button v-if="canWrite" class="primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存换算组' }}</button>
    </form>
  </section>
</template>
<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import OperationManualView from '../views/OperationManualView.vue'
const helpOpen = ref(false)
import { apiGet, apiSend } from '../api/client'
const emit = defineEmits(['saved', 'close'])
const catalog = ref({ groups: [], specs: [], units: [] })
const draft = ref(null), error = ref(''), saving = ref(false), canWrite = ref(false), search = ref(''), specSearch = ref(''), page = ref(1)
const filtered = computed(() => catalog.value.groups.filter(g => g.name.includes(search.value)))
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / 10)))
const pageGroups = computed(() => filtered.value.slice((page.value - 1) * 10, page.value * 10))
watch(search, () => { page.value = 1 })
function memberKey(m) { return m.bom_spec_id ? `spec:${m.bom_spec_id}:${m.inventory_unit}` : m.template_id ? `template:${m.template_id}:${m.template_key}:${m.inventory_unit}` : '' }
const memberOptions = computed(() => {
  const seen = new Map()
  for (const s of catalog.value.specs) {
    if (specSearch.value && !`${s.product_name} ${s.spec_name}`.includes(specSearch.value)) continue
    const member = s.template_id && s.template_key ? { template_id: s.template_id, template_key: s.template_key, inventory_unit: s.inventory_unit } : { bom_spec_id: s.bom_spec_id, inventory_unit: s.inventory_unit }
    const key = memberKey(member)
    if (!seen.has(key)) seen.set(key, { key, member, label: `${s.product_name} / ${s.spec_name} · ${s.inventory_unit}${member.template_id ? '（模板共用）' : '（本规格）'}` })
  }
  return [...seen.values()]
})
function create() { draft.value = { name: '', base_unit: '', active: true, members: [{ base_quantity: '' }, { base_quantity: '' }] }; error.value = '' }
function edit(group) { draft.value = JSON.parse(JSON.stringify(group)); specSearch.value = ''; error.value = '' }
function selectMember(index, key) { const option = memberOptions.value.find(o => o.key === key); if (option) draft.value.members[index] = { ...option.member, label: option.label, base_quantity: draft.value.members[index].base_quantity } }
async function load() { try { catalog.value = await apiGet('/api/product-settings/spec-conversion-groups'); page.value = Math.min(page.value, pages.value); error.value = '' } catch (e) { error.value = e.message } }
async function saveGroup(group) { saving.value = true; error.value = ''; try { const saved = await apiSend('/api/product-settings/spec-conversion-groups', { body: { group } }); await load(); draft.value = null; emit('saved', saved) } catch (e) { error.value = e.message } finally { saving.value = false } }
function save() { return saveGroup(draft.value) }
function disable(group) { return saveGroup({ ...group, active: false }) }
onMounted(async () => { await load(); try { const actor = await apiGet('/api/auth/me'); canWrite.value = Boolean(actor.basic_auth_admin || actor.permissions?.includes('*') || actor.permissions?.includes('products.write')) } catch { canWrite.value = false } })
</script>
<style scoped>
.conversion-manager{padding:20px;background:#fff;color:#24382e;max-width:900px;margin:auto}header,.toolbar,.group-row,nav{display:flex;gap:12px;align-items:center;justify-content:space-between}header h3{margin:0}.toolbar{margin:18px 0}.group-row{border-bottom:1px solid #e3e8e5;padding:12px 0}.group-row div{flex:1}small{display:block;color:#63776b}nav{justify-content:flex-end;margin:12px 0}form{border-top:1px solid #ddd;padding-top:18px}label{display:block;margin:12px 0}input,select{max-width:100%;padding:8px;border:1px solid #b8c9be;border-radius:6px}label>input:not([type=checkbox]),label>select{display:block;width:100%}input[type=checkbox]{width:auto}button{padding:8px 12px;cursor:pointer}button:disabled{opacity:.5}.member-row{display:grid;grid-template-columns:minmax(0,2fr) minmax(0,1fr) auto;gap:12px;align-items:center}.failure{color:#b42318}p{color:#5d7065;line-height:1.6}@media(max-width:650px){.member-row{grid-template-columns:1fr}.toolbar{flex-wrap:wrap}}
</style>
