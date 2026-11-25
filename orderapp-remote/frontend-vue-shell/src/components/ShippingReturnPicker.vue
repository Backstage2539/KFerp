<template>
  <Teleport to="body">
    <div class="return-mask" @click.self="close" @keydown.esc="close">
      <section role="dialog" aria-modal="true" aria-labelledby="return-picker-title">
        <header>
          <div>
            <h3 id="return-picker-title">选择快递单 · {{ orderNo }}</h3>
            <p>{{ rows[0]?.customer_name }}<span v-if="rows[0]?.order_date"> · 订单日期 {{ rows[0].order_date }}</span></p>
          </div>
          <button ref="closeButton" type="button" :disabled="busy" @click="close">关闭</button>
        </header>
        <label v-if="batches.length > 1" class="batch-select">
          回传批次
          <select :value="activeBatch" :disabled="busy" @change="changeBatch(Number($event.target.value))">
            <option v-for="id in batches" :key="id" :value="id">批次 #{{ id }}</option>
          </select>
        </label>
        <p v-if="batchRows.length">请核对收寄信息，勾选属于本订单的运单。同批多票可一起确认发货。</p>
        <p v-else role="status">当前已无可匹配的运单，请关闭并核对本次回传明细。</p>
        <p v-if="error" role="alert">{{ error }}</p>
        <div v-if="batchRows.length" class="scroll">
          <table>
            <thead><tr><th>选择</th><th>运单号 / 结算重量</th><th>快递下单 / 揽件时间</th><th>收件人 / 电话</th><th>寄件人 / 电话</th><th>实际物流成本</th></tr></thead>
            <tbody>
              <tr v-for="r in batchRows" :key="r.id">
                <td><input v-model="selectedIDs" type="checkbox" :value="r.id" :aria-label="'选择运单 ' + r.tracking_no" :disabled="busy || !canWrite || r.can_select === false" /></td>
                <td>
                  <strong>{{ r.tracking_no }}</strong><small>{{ r.settlement_weight_kg ? r.settlement_weight_kg + ' kg' : '重量未记录' }}</small>
                  <small v-if="r.can_select === false" class="blocked">{{ r.block_reason || '尚未满足发货条件' }}</small>
                </td>
                <td>{{ r.time || '—' }}<small>{{ r.pickup_time || '揽件时间未记录' }}</small></td>
                <td>{{ r.receiver || '—' }}<small>{{ r.phone || '—' }}</small></td>
                <td>{{ r.sender || '—' }}<small>{{ r.sender_phone || '—' }}</small></td>
                <td>¥{{ (r.actual_cents / 100).toFixed(2) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <footer v-if="batchRows.length">
          <div>
            <strong>已选 {{ selectedRows.length }} 票<span v-if="selectedRows.length"> · {{ weightLabel }}</span> · 实际物流成本 ¥{{ (selectedCents / 100).toFixed(2) }}</strong>
            <small v-if="selectedRows.length && selectedRows.length < rows.length">确认发货后，未勾选的运单不能再通过回传归入本订单。</small>
          </div>
          <button type="button" class="confirm" :disabled="busy || !canWrite || !validSelection" @click="submit">
            {{ busy ? '确认中…' : '确认发货' }}
          </button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>
<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { apiSend } from '../api/client'

const props = defineProps({
  rows: { type: Array, default: () => [] },
  orderId: { type: Number, required: true },
  orderNo: { type: String, default: '' },
  canWrite: { type: Boolean, default: true },
})
const emit = defineEmits(['close', 'saved'])
const selectedIDs = ref([])
const selectedBatch = ref(0)
const busy = ref(false)
const error = ref('')
const closeButton = ref(null)
let trigger
onMounted(() => { trigger = document.activeElement; closeButton.value?.focus() })
onUnmounted(() => { if (trigger?.isConnected) trigger.focus() })

const batches = computed(() => [...new Set(props.rows.map(r => Number(r.batch_id)))].sort((a, b) => b - a))
const activeBatch = computed(() => batches.value.includes(selectedBatch.value) ? selectedBatch.value : batches.value[0])
const batchRows = computed(() => props.rows.filter(r => Number(r.batch_id) === activeBatch.value))
const selectedRows = computed(() => batchRows.value.filter(r => selectedIDs.value.includes(r.id)))
const validSelection = computed(() => selectedRows.value.length > 0
  && selectedIDs.value.length === selectedRows.value.length
  && selectedRows.value.every(r => r.can_select !== false && r.version && r.version === selectedRows.value[0].version))
const selectedCents = computed(() => selectedRows.value.reduce((sum, r) => sum + r.actual_cents, 0))
const weightLabel = computed(() => {
  let grams = 0
  let missing = 0
  for (const row of selectedRows.value) {
    const kg = String(row.settlement_weight_kg ?? '').trim()
    if (/^\d+(?:\.\d{1,3})?$/.test(kg)) grams += Math.round(Number(kg) * 1000)
    else missing++
  }
  const kg = (grams / 1000).toFixed(3)
  if (!missing) return `结算重量 ${kg} kg`
  if (missing === selectedRows.value.length) return `结算重量未记录（${missing} 票）`
  return `已知结算重量 ${kg} kg（${missing} 票未记录）`
})


watch(() => batchRows.value.map(r => [r.id, r.version, r.can_select].join(':')).join('|'), () => {
  selectedIDs.value = []
})
function changeBatch(id) {
  selectedBatch.value = id
  selectedIDs.value = []
  error.value = ''
}
function close() {
  if (!busy.value) emit('close')
}
async function submit() {
  if (busy.value || props.canWrite === false || !validSelection.value) return
  busy.value = true
  error.value = ''
  const ids = selectedRows.value.map(r => r.id)
  const body = ids.length === 1 ? { candidate_id: ids[0] } : { candidate_ids: ids }
  body.version = selectedRows.value[0].version
  try {
    await apiSend(`/api/orders/${props.orderId}/shipping-return-selection`, { body })
    emit('saved')
  } catch (err) {
    error.value = err.message || '确认发货失败'
  } finally {
    busy.value = false
  }
}
</script>
<style scoped>
.return-mask { font-family: system-ui, sans-serif; position: fixed; inset: 0; z-index: 1200; background: #0006; display: grid; place-items: center; padding: 16px; }
section { box-sizing: border-box; background: #fff; border-radius: 10px; width: min(100%, 1000px); max-height: 90vh; overflow: auto; padding: 22px; color: #202621; }
header, footer, .batch-select { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
h3 { margin: 0; font-size: 18px; }
p, small { font-size: 13px; color: #526257; }
small { display: block; margin-top: 4px; }
header p { margin: 4px 0 0; }
.batch-select { justify-content: start; margin: 14px 0; font-size: 14px; }
.scroll { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; min-width: 710px; }
th, td { text-align: left; padding: 10px 8px; border-bottom: 1px solid #dce5df; font-size: 13px; }
th { background: #f3f6f4; white-space: nowrap; }
input { width: 17px; height: 17px; accent-color: #286248; }
footer { flex-wrap: wrap; margin-top: 15px; font-size: 14px; }
button, select { cursor: pointer; border: 1px solid #cad3cb; background: white; border-radius: 5px; padding: 7px 12px; font: inherit; }
.confirm { background: #266444; color: white; border-color: #266444; }
button:disabled { opacity: .5; cursor: not-allowed; }
[role=alert], .blocked { color: #9c3826; }
</style>
