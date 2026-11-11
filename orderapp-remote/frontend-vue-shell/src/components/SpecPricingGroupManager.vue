<template>
  <section class="spec-group-manager">
    <div class="spec-group-heading"><div><strong>规格计价组</strong><p>绑定一份规格模板，为每个规格选择价格计算模板。</p></div><button v-if="canWrite" class="primary" type="button" @click="startCreate">新建计价组</button></div>
    <p v-if="error" class="spec-group-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="spec-group-notice" role="status">{{ notice }}</p>
    <div class="spec-group-toolbar"><input v-model.trim="query" type="search" aria-label="搜索计价组" placeholder="搜索计价组" /><button class="secondary" type="button" :disabled="loading" @click="load">刷新</button></div>
    <div class="spec-group-table-wrap"><table class="spec-group-table"><thead><tr><th>计价组名称</th><th>规格模板</th><th>规格</th><th>状态</th><th>操作</th></tr></thead><tbody>
      <tr v-for="group in filteredGroups" :key="group.id"><td><button type="button" class="spec-group-link" @click="startEdit(group)">{{ group.name }}</button></td><td>{{ templateLabel(group.spec_template_id) }}</td><td>{{ groupSpecLabels(group) }}</td><td>{{ group.active ? '启用' : '停用' }}</td><td><button class="secondary" type="button" @click="startEdit(group)">{{ canWrite ? '编辑' : '查看' }}</button></td></tr>
      <tr v-if="!filteredGroups.length"><td colspan="5">{{ loading ? '正在加载…' : '暂无规格计价组' }}</td></tr>
    </tbody></table></div>
    <div v-if="editing" class="spec-group-mask" @click.self="closeEditor" @keydown.esc.stop.prevent="closeEditor"><aside class="spec-group-drawer" role="dialog" aria-modal="true" aria-label="规格计价组编辑">
      <header><div><h3>{{ form.id ? (canWrite ? '编辑规格计价组' : '查看规格计价组') : '新建规格计价组' }}</h3><p>一个组绑定一个规格模板，每个规格分别选择价格计算模板。</p></div><button class="secondary" type="button" @click="closeEditor">关闭</button></header>
      <div class="spec-group-body">
        <p v-if="editorError" class="spec-group-error" role="alert">{{ editorError }}</p>
        <label>计价组名称<input v-model.trim="form.name" :disabled="!canWrite" /></label>
        <label class="spec-group-check"><input v-model="form.active" type="checkbox" :disabled="!canWrite" />启用</label>
        <label>规格模板<select v-model.number="form.spec_template_id" :disabled="!canWrite" @change="changeTemplate"><option :value="0">请选择规格模板</option><option v-for="template in templates" :key="template.id" :value="Number(template.id)">{{ template.name }}</option></select></label>
        <p class="spec-group-hint">维护位置：商品 → 规格模板 <button type="button" class="spec-group-link" @click="viewTemplate">查看规格模板</button></p>
        <label>已发布版本<select v-model.number="form.spec_template_version_id" :disabled="!canWrite || !selectedTemplate" @change="changeVersion"><option :value="0">请选择版本</option><option v-for="version in selectedTemplate?.versions || []" :key="version.id" :value="Number(version.id)">{{ version.version_no }}</option></select></label>
        <strong>组内规格与价格模板</strong>
        <div class="spec-group-table-wrap"><table class="spec-group-table"><thead><tr><th><input type="checkbox" aria-label="选择全部规格" :checked="allSelected" :indeterminate.prop="someSelected" :disabled="!canWrite || !variants.length" @change="selectAll($event.target.checked)" /></th><th>规格</th><th>价格计算模板</th></tr></thead><tbody>
          <tr v-for="variant in variants" :key="variantKey(variant)"><td><input type="checkbox" :aria-label="`选择${variant.name}`" :checked="!!selectedItem(variant)" :disabled="!canWrite" @change="toggleVariant(variant, $event.target.checked)" /></td><td>{{ variant.name }} <small>{{ variant.inventory_unit }}</small></td><td><select :value="selectedItem(variant)?.pricing_rule_id || 0" :disabled="!canWrite || !selectedItem(variant)" :aria-label="`${variant.name}价格计算模板`" @change="setRule(variant, $event.target.value)"><option :value="0">请选择价格计算模板</option><option v-for="rule in pricingRules" :key="rule.id" :value="Number(rule.id)">{{ rule.name }}</option></select></td></tr>
          <tr v-if="!variants.length"><td colspan="3">请选择已发布规格模板版本</td></tr>
        </tbody></table></div>
        <p class="spec-group-hint">已选 {{ form.items.length }} 个规格。草稿跟随计价组更新，已发布价格保持不变。</p>
      </div><footer><button class="secondary" type="button" @click="closeEditor">取消</button><button v-if="canWrite" class="primary" type="button" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存计价组' }}</button></footer>
    </aside></div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client.js'
import { actorHasFullViewAccess } from '../lib/menu-permissions.js'
import { buildSpecPricingGroupPayload, validateSpecPricingGroup, canDisableUnchangedSpecPricingGroup, saveSpecPricingGroupReturnDraft, consumeSpecPricingGroupReturnDraft } from '../lib/spec-pricing-groups.js'

const props = defineProps({ viewParams: { type: Object, default: () => ({}) }, returnView: { type: String, default: 'productPriceManagement' } })
const emit = defineEmits(['saved'])
const groups = ref([]), templates = ref([]), pricingRules = ref([]), query = ref(''), error = ref(''), notice = ref(''), editorError = ref('')
const loading = ref(false), saving = ref(false), editing = ref(false), canWrite = ref(false)
const emptyForm = () => ({ id: 0, name: '', spec_template_id: 0, spec_template_version_id: 0, revision: 0, active: true, items: [] })
const form = ref(emptyForm())
const filteredGroups = computed(() => groups.value.filter((row) => String(row.name || '').toLowerCase().includes(query.value.toLowerCase())))
const selectedTemplate = computed(() => templates.value.find((row) => Number(row.id) === Number(form.value.spec_template_id)))
const selectedVersion = computed(() => selectedTemplate.value?.versions?.find((row) => Number(row.id) === Number(form.value.spec_template_version_id)))
const variants = computed(() => selectedVersion.value?.variants || [])
const variantKey = (row) => `${String(row.spec_key || '').toLowerCase()}\u0000${String(row.inventory_unit || '').toLowerCase()}`
const selectedItem = (variant) => form.value.items.find((item) => variantKey(item) === variantKey(variant))
const allSelected = computed(() => variants.value.length > 0 && variants.value.every((row) => selectedItem(row)))
const someSelected = computed(() => !allSelected.value && variants.value.some((row) => selectedItem(row)))
function templateLabel(id) { return templates.value.find((row) => Number(row.id) === Number(id))?.name || `模板 #${id}` }
function groupSpecLabels(group) { const version = templates.value.find((t) => Number(t.id) === Number(group.spec_template_id))?.versions?.find((v) => Number(v.id) === Number(group.spec_template_version_id)); return (group.items || []).map((i) => version?.variants?.find((v) => variantKey(v) === variantKey(i))?.name || i.spec_key).join(' / ') || '—' }
function startCreate() { form.value = emptyForm(); editorError.value = ''; editing.value = true }
function startEdit(group) { form.value = { ...emptyForm(), ...group, items: (group.items || []).map((item) => ({ ...item })) }; editorError.value = ''; editing.value = true }
function closeEditor() { editing.value = false; editorError.value = '' }
function changeTemplate() { form.value.spec_template_version_id = Number(selectedTemplate.value?.versions?.[0]?.id || 0); form.value.items = [] }
function changeVersion() { form.value.items = [] }
function toggleVariant(variant, checked) { if (checked && !selectedItem(variant)) form.value.items.push({ spec_key: variant.spec_key, inventory_unit: variant.inventory_unit, pricing_rule_id: 0 }); else if (!checked) form.value.items = form.value.items.filter((item) => variantKey(item) !== variantKey(variant)) }
function selectAll(checked) { for (const variant of variants.value) toggleVariant(variant, checked) }
function setRule(variant, id) { const item = selectedItem(variant); if (item) item.pricing_rule_id = Number(id || 0) }
async function load() { loading.value = true; error.value = ''; try { const [result, options] = await Promise.all([apiGet('/api/spec-pricing-groups'), apiGet('/api/spec-pricing-groups/options')]); groups.value = result.groups || []; templates.value = options.templates || []; pricingRules.value = options.pricing_rules || [] } catch (err) { error.value = err.message || '加载计价组失败' } finally { loading.value = false } }
async function save() { const previous = groups.value.find((group) => Number(group.id) === Number(form.value.id)); editorError.value = canDisableUnchangedSpecPricingGroup(form.value, previous) ? '' : validateSpecPricingGroup(form.value, templates.value, pricingRules.value); if (editorError.value) return; saving.value = true; try { const payload = buildSpecPricingGroupPayload(form.value); const result = await apiSend(payload.id ? `/api/spec-pricing-groups/${payload.id}` : '/api/spec-pricing-groups', { method: payload.id ? 'PUT' : 'POST', body: payload }); editing.value = false; notice.value = '计价组已保存'; await load(); emit('saved', result.group) } catch (err) { editorError.value = err.message || '保存计价组失败' } finally { saving.value = false } }
function viewTemplate() { const key = saveSpecPricingGroupReturnDraft(form.value); const params = Object.fromEntries(Object.entries(props.viewParams || {}).filter(([name, value]) => name !== 'return_navigation' && value != null && ['string', 'number', 'boolean'].includes(typeof value))); window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'productSpecTemplates', params: { spec_template_id: form.value.spec_template_id, spec_template_version_id: form.value.spec_template_version_id }, returnNavigation: { key: props.returnView, label: '返回规格计价组', params: { ...params, spec_pricing_group_return_key: key, price_management_tab: 'spec-pricing-groups' } } } })) }
onMounted(async () => { try { const actor = await apiGet('/api/auth/me'); canWrite.value = Boolean(actorHasFullViewAccess(actor) || actor.permissions?.includes('*') || actor.permissions?.includes('products.write')) } catch { canWrite.value = false }; await load(); const returnKey = props.viewParams.spec_pricing_group_return_key; const restored = consumeSpecPricingGroupReturnDraft(returnKey); if (returnKey && typeof window !== 'undefined') { const url = new URL(window.location.href); url.searchParams.delete('spec_pricing_group_return_key'); url.searchParams.delete('price_management_tab'); window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`) }; if (restored) { form.value = restored; editing.value = true } })
</script>

<style scoped>
.spec-group-manager { color:#202b3a; } .spec-group-heading { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; } .spec-group-heading p,.spec-group-drawer header p,.spec-group-hint { margin:6px 0 12px; color:#596b7c; font-size:13px; } .spec-group-toolbar { display:flex; gap:8px; margin:14px 0; } .spec-group-toolbar input { max-width:320px; flex:1; } .spec-group-table-wrap { overflow-x:auto; } .spec-group-table { width:100%; border-collapse:collapse; text-align:left; } .spec-group-table th,.spec-group-table td { border-bottom:1px solid #e2e8ef; padding:10px; } .spec-group-table th { background:#f5f7f9; } .spec-group-table select { width:100%; min-width:180px; } .spec-group-link { border:0; padding:0; background:none; color:#23734a; cursor:pointer; text-align:left; } .spec-group-error { color:#b42318; } .spec-group-notice { color:#17763a; } .spec-group-mask { position:fixed; inset:0; z-index:80; display:flex; justify-content:flex-end; background:rgba(0,0,0,.23); } .spec-group-drawer { width:min(760px,96vw); height:100%; background:white; box-shadow:-12px 0 32px rgba(0,0,0,.16); display:flex; flex-direction:column; } .spec-group-drawer header,.spec-group-drawer footer { display:flex; justify-content:space-between; gap:12px; padding:16px; border-bottom:1px solid #e2e8ef; } .spec-group-drawer footer { justify-content:flex-end; border-top:1px solid #e2e8ef; border-bottom:0; } .spec-group-drawer h3 { margin:0; } .spec-group-body { padding:16px; overflow:auto; flex:1; display:grid; align-content:start; gap:12px; } .spec-group-body label:not(.spec-group-check) { display:grid; gap:6px; } .spec-group-check { display:flex; align-items:center; gap:6px; } .spec-group-body input:not([type=checkbox]),.spec-group-body select,.spec-group-toolbar input { padding:8px; border:1px solid #cbd5df; border-radius:5px; } .spec-group-body small { color:#64748b; } @media (max-width:600px) { .spec-group-drawer { width:100%; } .spec-group-table th,.spec-group-table td { padding:7px; } }
</style>
