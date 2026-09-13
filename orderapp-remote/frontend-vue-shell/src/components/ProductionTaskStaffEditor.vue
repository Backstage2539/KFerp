<template>
  <section class="task-staff-editor" aria-label="调整工序人员">
    <div class="editor-heading"><strong>调整工序人员</strong><button type="button" class="text-action" :disabled="saving" @click="close">取消</button></div>
    <p class="muted">{{ row.operation }} · 人员调整会同步到排程和工位，已排时间保持不变。</p>
    <p v-if="loading">正在读取当前安排…</p>
    <p v-if="error" role="alert" class="warning">{{ error }} <button v-if="conflicted" type="button" class="text-action" @click="load">重新读取</button></p>
    <template v-if="row.id && !loading">
      <ProductionStaffFields v-model="draft" :employees="row.eligible_employees" :disabled="saving" @configure="configure" />
      <div v-if="preview?.conflicts?.length" class="warning" role="alert"><strong>人员时间重叠，请核对</strong><p v-for="conflict in preview.conflicts" :key="`${conflict.employee_id}:${conflict.other_job_card_id}`">{{ conflict.message }}</p></div>
      <button class="primary" type="button" :disabled="saving || !row.eligible_employees?.length" @click="save">{{ saving ? '正在保存…' : preview?.conflicts?.length ? '确认重叠并保存人员' : '保存人员' }}</button>
    </template>
  </section>
</template>
<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { staffPatch, validateStaff } from '../lib/production-staff'
import ProductionStaffFields from './ProductionStaffFields.vue'
const props = defineProps({ jobCardId: { type: Number, required: true }, workOrderId: { type: Number, required: true } })
const emit = defineEmits(['saved', 'cancel'])
const row = ref({}), draft = ref({}), loading = ref(false), saving = ref(false), error = ref(''), preview = ref(null), conflicted = ref(false)
let requestID = '', savedDraft = ''
watch(draft, () => { preview.value = null; requestID = '' }, { deep: true, flush: 'sync' })
async function load() {
 loading.value = true; error.value = ''; conflicted.value = false
 try { const data = await apiGet(`/api/production-schedule?scope=task&page=1&job_card_id=${props.jobCardId}`); row.value = data.rows?.[0] || {}; if (!row.value.id) throw new Error('未找到工序任务'); if (row.value.arrangement_state === 'history') throw new Error('该工单已结束，人员安排仅供查看'); draft.value = { ...row.value }; savedDraft = JSON.stringify(draft.value) } catch (err) { error.value = err.message; row.value = {} } finally { loading.value = false }
}
const dirty = computed(() => !!row.value.id && JSON.stringify(draft.value) !== savedDraft)
function canLeave() { if (saving.value) return false; if (!dirty.value) return true; if (!window.confirm('人员修改尚未保存，确认离开？')) return false; savedDraft = JSON.stringify(draft.value); return true }
function guard(event) { if (!canLeave()) { event.preventDefault(); event.stopImmediatePropagation() } }
function unloadGuard(event) { if (dirty.value || saving.value) { event.preventDefault(); event.returnValue = '' } }
defineExpose({ canLeave })
function close() { if (dirty.value && !window.confirm('人员修改尚未保存，确认取消？')) return; emit('cancel') }
function configure() { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'productionConfig', params: { tab: 'operations', operation_id: row.value.operation_id }, returnNavigation: { key: 'productionSchedule', params: { job_card_id: props.jobCardId }, label: '返回人员安排' } } })) }
async function save() {
 if (saving.value) return
 error.value = validateStaff(draft.value.assigned_employee_id, row.value.eligible_employees)
 if (error.value) return
 saving.value = true
 try {
  const items = [staffPatch(draft.value)]
  if (!requestID) requestID = crypto.randomUUID()
  if (!preview.value) { preview.value = await apiSend('/api/production-schedule/preview', { body: { items } }); if (preview.value.conflicts?.length) return }
  await apiSend('/api/production-schedule/batch', { body: { items, request_id: requestID, preview_token: preview.value.preview_token } })
  savedDraft = JSON.stringify(draft.value); emit('saved')
 } catch (err) { error.value = err.message; conflicted.value = err.code === 'version_conflict'; if (err.code === 'confirmation_required') preview.value = null } finally { saving.value = false }
}
watch(() => props.jobCardId, load)
onMounted(() => { load(); window.addEventListener('kferp:before-navigate', guard); window.addEventListener('beforeunload', unloadGuard) })
onBeforeUnmount(() => { window.removeEventListener('kferp:before-navigate', guard); window.removeEventListener('beforeunload', unloadGuard) })
</script>
<style scoped>
.task-staff-editor{width:100%;min-width:0;border:1px solid #dce6df;border-radius:10px;background:#f8fbf9;padding:15px}.editor-heading{display:flex;justify-content:space-between;gap:10px}.muted{color:#738077;font-size:12px;margin:7px 0 14px}.primary{margin-top:14px;border:1px solid #2f8f5b;border-radius:8px;background:#2f8f5b;color:white;padding:9px 14px;min-height:38px;font:inherit;cursor:pointer}.primary:disabled{opacity:.5;cursor:not-allowed}.warning{background:#fff7e7;border:1px solid #eed19a;border-radius:8px;padding:10px;color:#965b1d;font-size:13px}.text-action{border:0;background:none;color:#2672ac;padding:0;font:inherit;cursor:pointer}
</style>
