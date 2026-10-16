<template>
  <div class="conversion-backdrop" @click.self="$emit('close')"><section class="conversion-drawer" role="dialog" aria-modal="true" aria-label="临时换规格">
    <header><h3>临时换规格</h3><button type="button" @click="$emit('close')">关闭</button></header>
    <p>价格表 {{ row.bean_list_version_no }} · {{ row.product_name }}。档位由你选择，数量变化保持此档位。</p>
    <label>来源规格<select v-model="sourceID" @change="rowKey = ''; groupID = 0; targetID = 0"><option :value="0">请选择</option><option v-for="s in sources" :key="s.bom_spec_id" :value="Number(s.bom_spec_id)">{{ s.spec_label || s.spec_name }}</option></select></label>
    <label>来源档位<select v-model="rowKey"><option value="">请选择档位</option><option v-for="t in tiers" :key="t.priceRowKey" :value="t.priceRowKey">{{ t.tierLabel || t.rangeLabel }} · {{ t.unitPrice }}{{ t.priceUnit?.suffix }}</option></select></label>
    <label>换算组<select v-model="groupID" @change="targetID = 0"><option :value="0">请选择</option><option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}（修订 {{ g.revision }}）</option></select></label>
    <div class="actions"><button type="button" @click="load">刷新换算组</button><button v-if="canManage" type="button" @click="managing = !managing">快捷维护换算组</button></div>
    <SpecConversionManager v-if="managing" @close="managing = false" @saved="groupSaved" />
    <label>实际下单规格<select v-model="targetID"><option :value="0">请选择</option><option v-for="s in targets" :key="s.bom_spec_id" :value="s.bom_spec_id">{{ s.spec_name }}（{{ s.inventory_unit }}）</option></select></label>
    <p v-if="!targets.length">目标规格须在当前默认已发布 BOM 中，并已关联到所选换算组。</p>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <div v-if="quote" class="quote"><p>{{ conversionDescription(quote) }}</p><b>单价 ¥{{ quote.unit_price }} × {{ row.qty }} = ¥{{ quote.line_total }}</b><p>单价四舍五入到分；生产、库存和发货按 {{ quote.target_spec_name }}。</p></div>
    <button class="primary" type="button" :disabled="!quote || busy" @click="$emit('apply', quote)">{{ busy ? '正在报价…' : '使用此规格和价格' }}</button>
  </section></div>
</template>
<script setup>
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { conversionDescription, conversionTargets, latestConversionRequest } from '../lib/spec-conversion'
import SpecConversionManager from './SpecConversionManager.vue'
const props = defineProps({ row: { type: Object, required: true }, family: { type: Object, default: () => ({}) }, customerId: { type: Number, required: true } })
defineEmits(['close', 'apply'])
const catalog = ref({ groups: [], specs: [] }), sourceID = ref(Number(props.row.spec_conversion?.source_bom_spec_id || props.row.bom_spec_id || 0)), rowKey = ref(props.row.selected_price_row_key || ''), groupID = ref(Number(props.row.spec_conversion?.group_id || 0)), targetID = ref(Number(props.row.spec_conversion ? props.row.bom_spec_id : 0)), quote = ref(null), busy = ref(false), error = ref(''), canManage = ref(false), managing = ref(false)
const request = latestConversionRequest(); let timer, catalogSequence = 0
const sources = computed(() => [...new Map((catalog.value.source_tiers || []).map(s => [s.bom_spec_id, s])).values()])
const source = computed(() => sources.value.find(s => Number(s.bom_spec_id) === sourceID.value))
const tiers = computed(() => (catalog.value.source_tiers || []).filter(t => t.bom_spec_id === sourceID.value).map(t => ({ ...t, priceRowKey: t.price_row_key, tierLabel: t.tier_label, unitPrice: t.unit_price, priceUnit: { suffix: `/${t.inventory_unit}` } })))
const groups = computed(() => catalog.value.groups.filter(g => conversionTargets(g, sources.value).some(s => s.bom_spec_id === sourceID.value)))
const group = computed(() => groups.value.find(g => g.id === groupID.value))
const targets = computed(() => conversionTargets(group.value, catalog.value.specs).filter(s => s.bom_spec_id !== sourceID.value))
async function load() { const current = ++catalogSequence; request.cancel(); clearTimeout(timer); quote.value = null; busy.value = false; try { const result = await apiGet(`/api/order/spec-conversions?customer_id=${props.customerId}&product_id=${props.row.product_id}&publication_id=${props.row.bean_list_publication_id}`); if (current !== catalogSequence) return; catalog.value = result; error.value = ''; schedule() } catch (e) { if (current === catalogSequence) error.value = e.message } }
function schedule() { clearTimeout(timer); const token = request.next(); quote.value = null; error.value = ''; busy.value = false; if (!source.value || !group.value || !targets.value.some(s => s.bom_spec_id === targetID.value) || !tiers.value.some(t => t.priceRowKey === rowKey.value)) return; const target = targets.value.find(s => s.bom_spec_id === targetID.value); const body = { customer_id: props.customerId, product_id: props.row.product_id, publication_id: props.row.bean_list_publication_id, source_bom_spec_id: sourceID.value, source_bom_variant_id: tiers.value.find(t => t.priceRowKey === rowKey.value).bom_variant_id, price_row_key: rowKey.value, group_id: group.value.id, revision: group.value.revision, target_bom_spec_id: target.bom_spec_id, target_bom_variant_id: target.bom_variant_id, quantity: Number(props.row.qty) }; busy.value = true; timer = setTimeout(async () => { try { const result = await apiSend('/api/order/spec-conversions/quote', { body }); if (request.current(token)) quote.value = result } catch (e) { if (request.current(token)) error.value = e.message } finally { if (request.current(token)) busy.value = false } }, 250) }
watch(() => [sourceID.value, rowKey.value, groupID.value, targetID.value, props.row.qty, props.customerId, props.row.product_id, props.row.bean_list_publication_id], schedule)
watch(() => [props.customerId, props.row.product_id, props.row.bean_list_publication_id], () => { sourceID.value = 0; rowKey.value = ''; groupID.value = 0; targetID.value = 0; catalog.value = { groups: [], specs: [] }; load() })
async function groupSaved(saved) { managing.value = false; await load(); groupID.value = saved.id }
onMounted(async () => { await load(); try { const actor = await apiGet('/api/auth/me'); canManage.value = Boolean(actor.basic_auth_admin || actor.permissions?.includes('*') || actor.permissions?.includes('products.write')) } catch {} })
onBeforeUnmount(() => { catalogSequence++; request.cancel(); clearTimeout(timer) })
</script>
<style scoped>
.conversion-backdrop{position:fixed;inset:0;background:#0006;z-index:1400;display:flex;justify-content:flex-end}.conversion-drawer{background:#fff;width:min(760px,100vw);height:100%;overflow:auto;padding:24px;box-sizing:border-box}header,.actions{display:flex;gap:12px;justify-content:space-between;align-items:center}label{display:block;margin:18px 0}select{display:block;width:100%;padding:10px;border:1px solid #bdcdc3;border-radius:6px;margin-top:6px}button{padding:10px 14px;cursor:pointer}.quote{padding:16px;background:#edf7f0;border-radius:8px;margin:20px 0}p{line-height:1.65;color:#51685a}.error{color:#b42318}
</style>
