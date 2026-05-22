<template>
  <div class="seal-settings-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>公章设置</h2>
        <div class="actions">
          <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="notice error">{{ error }}</div>
      <div v-if="ok" class="notice ok">已保存</div>

      <div class="seal-toolbar">
        <label>
          <span>当前公章</span>
          <select v-model.number="selectedSealID" :disabled="loading || !seals.length" @change="selectSeal">
            <option :value="0">未选择</option>
            <option v-for="seal in seals" :key="seal.id" :value="seal.id">{{ seal.filename || `公章 ${seal.id}` }}</option>
          </select>
        </label>
        <div class="current-seal">当前：{{ settings.seal?.filename || '未设置' }}</div>
        <input type="file" accept="image/*" @change="handleSealFile" />
        <button class="primary" type="button" @click="uploadSeal" :disabled="uploadingSeal || !sealFile">上传公章</button>
        <button class="secondary" type="button" @click="removeSealBackground" :disabled="removingSealBackground || !settings.seal">{{ removingSealBackground ? '处理中' : '去除背景' }}</button>
      </div>

      <p class="hint">销售单、出库单和合同盖章共用这一套公章资产；公章位置和大小在对应单据预览或合同盖章页中拖动调整。</p>
      <div v-if="seals.length" class="seal-list">
        <div v-for="seal in seals" :key="seal.id" class="seal-card" :class="{ active: Number(seal.id) === Number(selectedSealID) }">
          <img v-if="seal.url" :src="seal.url" alt="公章预览" />
          <div>
            <strong>{{ seal.filename || `公章 ${seal.id}` }}</strong>
            <span>{{ seal.created_at || '-' }}</span>
          </div>
        </div>
      </div>
      <div v-else class="muted">暂无公章，请先上传。</div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const props = defineProps({
  embedded: { type: Boolean, default: false },
})

const emit = defineEmits(['close', 'updated'])

const loading = ref(false)
const uploadingSeal = ref(false)
const removingSealBackground = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const settings = ref({})
const seals = ref([])
const selectedSealID = ref(0)
const sealFile = ref(null)

function assignSettings(data = {}) {
  settings.value = data || {}
  selectedSealID.value = Number(data?.seal?.id || selectedSealID.value || 0)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [settingsData, sealData] = await Promise.all([
      apiGet('/api/settings/sales-order'),
      apiGet('/api/settings/sales-order/seals'),
    ])
    seals.value = sealData.rows || []
    selectedSealID.value = Number(sealData.current_id || settingsData?.seal?.id || 0)
    assignSettings(settingsData)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function handleSealFile(event) {
  sealFile.value = event.target.files?.[0] || null
}

async function selectSeal() {
  if (!selectedSealID.value) return
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    assignSettings(await apiSend('/api/settings/sales-order/seal/select', {
      body: { asset_id: selectedSealID.value },
    }))
    ok.value = true
    await load()
    emit('updated')
  } catch (err) {
    error.value = err.message || '选择公章失败'
  } finally {
    saving.value = false
  }
}

async function uploadSeal() {
  if (!sealFile.value) return
  uploadingSeal.value = true
  error.value = ''
  ok.value = false
  try {
    const body = new FormData()
    body.append('file', sealFile.value)
    await apiSend('/api/settings/sales-order/seal', { body })
    sealFile.value = null
    await load()
    ok.value = true
    emit('updated')
  } catch (err) {
    error.value = err.message || '上传失败'
  } finally {
    uploadingSeal.value = false
  }
}

async function removeSealBackground() {
  if (!settings.value?.seal) return
  removingSealBackground.value = true
  error.value = ''
  ok.value = false
  try {
    await apiSend('/api/settings/sales-order/seal/remove-background')
    await load()
    ok.value = true
    emit('updated')
  } catch (err) {
    error.value = err.message || '去除公章背景失败'
  } finally {
    removingSealBackground.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.seal-settings-page { padding: 18px; color: #171717; }
.seal-settings-page.embedded { padding: 14px; }
.panel { background: #fff; border: 1px solid #e6e0d8; border-radius: 8px; padding: 16px; }
.panel-head, .actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
button { border: 1px solid #d7cec3; border-radius: 6px; background: #fff; padding: 8px 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.primary { background: #1f6f4a; border-color: #1f6f4a; color: #fff; }
.secondary { background: #fff; color: #171717; }
input, select { width: 100%; border: 1px solid #d8d1c8; border-radius: 6px; padding: 9px 10px; font: inherit; background: #fff; color: #171717; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.seal-toolbar { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(160px, 1fr) minmax(180px, 1fr) auto auto; gap: 10px; align-items: end; margin-bottom: 12px; }
.current-seal { color: #555; font-size: 13px; align-self: center; }
.hint { color: #4b5563; font-size: 13px; line-height: 1.5; margin: 4px 0 12px; }
.seal-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 10px; }
.seal-card { display: grid; grid-template-columns: 54px minmax(0, 1fr); gap: 10px; align-items: center; border: 1px solid #eee8df; border-radius: 8px; padding: 8px; background: #fffdf9; }
.seal-card.active { border-color: #1f6f4a; box-shadow: 0 0 0 1px #1f6f4a inset; }
.seal-card img { width: 54px; height: 54px; object-fit: contain; }
.seal-card strong, .seal-card span { display: block; min-width: 0; overflow-wrap: anywhere; }
.seal-card span { color: #666; font-size: 12px; margin-top: 3px; }
.notice { padding: 10px 12px; border-radius: 7px; margin-bottom: 12px; }
.notice.ok { background: #eef8f1; border: 1px solid #cfe8d4; color: #1f6f4a; }
.notice.error { background: #fff1f1; border: 1px solid #f0caca; color: #9d1c1c; }
.muted { color: #888; text-align: center; }
@media (max-width: 900px) {
  .seal-settings-page { padding: 12px; }
  .panel-head, .actions { align-items: stretch; flex-direction: column; }
  .seal-toolbar { grid-template-columns: 1fr; }
}
</style>
