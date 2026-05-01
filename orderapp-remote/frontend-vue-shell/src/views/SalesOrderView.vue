<template>
  <div class="page" :class="{ 'embedded-page': props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <h2>销售单</h2>
        <div class="actions">
          <button v-if="props.embedded" class="secondary" type="button" @click="emit('close')">关闭</button>
          <a v-else class="secondary link-button" href="/vue-shell?view=orders">返回订单列表</a>
          <button class="secondary" type="button" @click="openSettingsDrawer">销售单设置</button>
          <button class="secondary" type="button" @click="openCustomerDrawer" :disabled="!customerSummary.id">客户信息</button>
          <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !orderID">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
          <a v-if="documents.length" class="secondary link-button" :href="salesOrderDownloadUrl(orderID)" target="_blank" rel="noopener">下载最新版</a>
          <button class="primary" type="button" @click="generate" :disabled="generating || !orderID || !preview">{{ generating ? '生成中' : '确认生成 PDF' }}</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div class="summary">
        <span>订单 ID：{{ orderID || '-' }}</span>
        <span>客户：{{ customerSummary.name || '-' }}</span>
        <span>公司：{{ customerSummary.company_name || customerSummary.name || '-' }}</span>
        <span>版本数：{{ documents.length }}</span>
      </div>
      <details class="manual">
        <summary>销售单手册</summary>
        <ul>
          <li>首次生成销售单为 V1，同一订单再次生成会创建 V2，不覆盖旧文件。</li>
          <li>生成前先查看“销售单预览”；客户信息调整后可刷新预览，确认内容后再生成 PDF。</li>
          <li>公账收款信息在“公司设置”维护；预览会展示纳税人识别号、公司地址、户名、开户行和账号。</li>
          <li>客户公司地址过长时会自动换行；收款码会按数量自动放大或排列，减少空白区域。</li>
          <li>预览里的公章可直接拖动，松开后保存位置；位置只影响之后生成的新销售单。</li>
          <li>销售单内容按生成时的订单和设置保存快照，后续修改设置不会改动旧版本。</li>
          <li>需要给客户最新文件时使用“下载最新版”，需要追溯时下载指定历史版本。</li>
        </ul>
      </details>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>销售单预览 <span v-if="preview" class="version-tag">V{{ preview.next_version_no }}</span></h3>
        <button class="secondary" type="button" @click="loadPreview" :disabled="previewLoading || !orderID">{{ previewLoading ? '预览中' : '刷新预览' }}</button>
      </div>
      <div v-if="!preview" class="muted preview-empty">暂无预览</div>
      <div v-else class="preview-box">
        <div ref="previewSealStage" class="preview-title">
          <strong>{{ preview.snapshot.company_name }}</strong>
          <span>销售单 SALES ORDER</span>
          <img
            v-if="preview.snapshot.seal"
            class="seal-stamp-preview"
            :src="assetURL(preview.snapshot.seal)"
            alt="公章"
            :style="sealPositionStyle(preview.snapshot.seal)"
            title="拖动调整公章位置"
            @pointerdown.stop="startPreviewSealDrag"
          />
        </div>
        <div class="preview-meta">
          <span>订单号：{{ preview.snapshot.order_no }}</span>
          <span>订单日期：{{ preview.snapshot.order_date || '-' }}</span>
          <span>客户：{{ preview.snapshot.customer_name || '-' }}</span>
          <span>客户公司：{{ preview.snapshot.customer_company_name || preview.snapshot.customer_name || '-' }}</span>
          <span>联系电话：{{ preview.snapshot.customer_company_phone || '-' }}</span>
          <span>公司地址：{{ preview.snapshot.customer_company_address || '-' }}</span>
        </div>
        <table>
          <thead>
            <tr><th>商品</th><th>规格</th><th>数量</th><th>单价</th><th>小计</th></tr>
          </thead>
          <tbody>
            <tr v-for="(item, idx) in preview.snapshot.items" :key="`${item.name}-${idx}`">
              <td>{{ item.name }}</td>
              <td>{{ item.spec }}</td>
              <td>{{ item.qty }}{{ item.unit }}</td>
              <td>{{ item.unit_price }}</td>
              <td>{{ item.line_total }}</td>
            </tr>
          </tbody>
        </table>
        <div class="preview-total">
          <span>商品合计：{{ preview.snapshot.total_amount }}</span>
          <span>运费：{{ preview.snapshot.shipping }}</span>
          <span>优惠：{{ preview.snapshot.discount }}</span>
          <strong>应收：{{ preview.snapshot.grand_total }}</strong>
        </div>
        <div v-if="hasPaymentInfo(preview.snapshot)" class="payment-info-grid">
          <div class="payment-text-panel">
            <div v-if="preview.snapshot.payment_text" class="preview-notes compact-notes">
              <strong>收款方式</strong>
              <p>{{ preview.snapshot.payment_text }}</p>
            </div>
            <div v-if="hasBankAccount(preview.snapshot)" class="preview-notes compact-notes account-payment-preview">
              <strong>公账收款</strong>
              <p v-if="preview.snapshot.bank_account_name">户名：{{ preview.snapshot.bank_account_name }}</p>
              <p v-if="preview.snapshot.taxpayer_id">纳税人识别号：{{ preview.snapshot.taxpayer_id }}</p>
              <p v-if="preview.snapshot.company_address">地址：{{ preview.snapshot.company_address }}</p>
              <p v-if="preview.snapshot.bank_name">开户行：{{ preview.snapshot.bank_name }}</p>
              <p v-if="preview.snapshot.bank_account_no">账号：{{ preview.snapshot.bank_account_no }}</p>
            </div>
            <div v-if="preview.snapshot.note" class="preview-notes compact-notes">
              <strong>说明</strong>
              <p>{{ preview.snapshot.note }}</p>
            </div>
          </div>
          <div v-if="(preview.snapshot.payment_codes || []).length" class="payment-code-panel">
            <div
              class="payment-code-preview-list"
              :class="{
                'single-payment-code': (preview.snapshot.payment_codes || []).length === 1,
                'payment-code-stack': (preview.snapshot.payment_codes || []).length > 1,
              }"
            >
              <div v-for="code in preview.snapshot.payment_codes" :key="`${code.id}-${code.label}`" class="payment-code-preview payment-code-preview-card">
                <img :src="assetURL(code)" :alt="code.label || '收款码'" />
                <div>
                  <strong>{{ code.label || '收款码' }}</strong>
                  <span v-if="code.description">{{ code.description }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>历史版本</h3>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <table>
        <thead>
          <tr><th>版本</th><th>订单号</th><th>生成时间</th><th>操作人</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="doc in documents" :key="doc.id">
            <td>V{{ doc.version_no }}</td>
            <td>{{ doc.order_no }}</td>
            <td>{{ doc.created_at }}</td>
            <td>{{ doc.created_by || '-' }}</td>
            <td>{{ doc.is_latest ? '最新版' : '历史版本' }}</td>
            <td><a class="text-link" :href="doc.download_url" target="_blank" rel="noopener">下载</a></td>
          </tr>
          <tr v-if="!documents.length"><td colspan="6" class="muted">暂无销售单版本</td></tr>
        </tbody>
      </table>
    </section>

    <div v-if="drawerOpen" class="drawer-mask" @click.self="closeCustomerDrawer">
      <aside class="drawer" aria-label="客户信息">
        <div class="drawer-head">
          <h3>客户信息</h3>
          <button class="secondary" type="button" @click="closeCustomerDrawer">关闭</button>
        </div>
        <form class="drawer-form" @submit.prevent="saveCustomer">
          <label>
            <span>客户名</span>
            <input v-model.trim="customerForm.name" required />
          </label>
          <label>
            <span>公司名称</span>
            <input v-model.trim="customerForm.company_name" name="company_name" placeholder="不填则默认客户名" />
          </label>
          <label>
            <span>联系电话</span>
            <input v-model.trim="customerForm.company_phone" name="company_phone" />
          </label>
          <label>
            <span>联系人</span>
            <input v-model.trim="customerForm.contact" />
          </label>
          <label>
            <span>收货电话</span>
            <input v-model.trim="customerForm.phone" />
          </label>
          <label>
            <span>公司地址</span>
            <textarea v-model.trim="customerForm.company_address" name="company_address" rows="3"></textarea>
          </label>
          <label>
            <span>收货地址</span>
            <textarea v-model.trim="customerForm.address" rows="3"></textarea>
          </label>
          <div class="drawer-actions">
            <button class="primary" type="submit" :disabled="savingCustomer">{{ savingCustomer ? '保存中' : '保存客户信息' }}</button>
          </div>
        </form>
      </aside>
    </div>

    <div v-if="settingsDrawerOpen" class="drawer-mask settings-drawer-mask" @click.self="closeSettingsDrawer">
      <aside class="drawer settings-drawer" aria-label="销售单设置">
        <div class="drawer-head">
          <h3>销售单设置</h3>
          <button class="secondary" type="button" @click="closeSettingsDrawer">关闭</button>
        </div>
        <SalesOrderSettingsView embedded />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { salesOrderDownloadUrl } from '../lib/sales-order'
import SalesOrderSettingsView from './SalesOrderSettingsView.vue'

const props = defineProps({
  orderId: { type: [Number, String], default: 0 },
  embedded: { type: Boolean, default: false },
})

const emit = defineEmits(['close'])

const loading = ref(false)
const previewLoading = ref(false)
const generating = ref(false)
const error = ref('')
const message = ref('')
const documents = ref([])
const preview = ref(null)
const drawerOpen = ref(false)
const settingsDrawerOpen = ref(false)
const savingCustomer = ref(false)
const sealDragSaving = ref(false)
const previewSealStage = ref(null)
const customerSummary = reactive(emptyCustomer())
const customerForm = reactive(emptyCustomer())

const orderID = computed(() => Number(props.orderId || new URL(window.location.href).searchParams.get('order_id') || 0))

async function load() {
  if (!orderID.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/orders/${orderID.value}/sales-orders`)
    documents.value = data.rows || []
    assignCustomer(customerSummary, data.order?.customer || {})
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadPage() {
  await load()
  await loadPreview()
}

async function loadPreview() {
  if (!orderID.value) return
  previewLoading.value = true
  error.value = ''
  try {
    preview.value = await apiGet(`/api/orders/${orderID.value}/sales-order-preview`)
  } catch (err) {
    preview.value = null
    error.value = err.message || '加载销售单预览失败'
  } finally {
    previewLoading.value = false
  }
}

async function generate() {
  if (!orderID.value || !preview.value) return
  generating.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/orders/${orderID.value}/sales-orders`)
    message.value = `已生成 V${data.version_no}`
    await load()
    await loadPreview()
  } catch (err) {
    error.value = err.message || '生成失败'
  } finally {
    generating.value = false
  }
}

function emptyCustomer() {
  return {
    id: 0,
    name: '',
    raw_name: '',
    company_name: '',
    company_address: '',
    company_phone: '',
    contact: '',
    phone: '',
    address: '',
    default_source_id: null,
    default_order_type_id: null,
    active: true,
  }
}

function assignCustomer(target, data = {}) {
  Object.assign(target, {
    id: Number(data.id || 0),
    name: data.name || '',
    raw_name: data.raw_name || '',
    company_name: data.company_name || '',
    company_address: data.company_address || '',
    company_phone: data.company_phone || '',
    contact: data.contact || '',
    phone: data.phone || '',
    address: data.address || '',
    default_source_id: data.default_source_id || null,
    default_order_type_id: data.default_order_type_id || null,
    active: data.active !== false,
  })
}

function assetURL(ref = {}) {
  if (ref.url) return ref.url
  return ref.object_key ? `/assets/${ref.object_key}` : ''
}

function sealPositionStyle(seal = {}) {
  const scale = previewSealScale()
  const x = Number(seal.x_mm || 32)
  const y = Number(seal.y_mm || 22)
  const w = Number(seal.width_mm || 42)
  return {
    left: `${x * scale}px`,
    top: `${y * scale}px`,
    width: `${w * scale}px`,
    height: `${w * 0.62 * scale}px`,
  }
}

function hasBankAccount(snapshot = {}) {
  return Boolean(snapshot.bank_account_name || snapshot.taxpayer_id || snapshot.company_address || snapshot.bank_name || snapshot.bank_account_no)
}

function hasPaymentInfo(snapshot = {}) {
  return Boolean(snapshot.payment_text || snapshot.note || hasBankAccount(snapshot) || (snapshot.payment_codes || []).length)
}

function previewSealScale() {
  return 2.2
}

function startPreviewSealDrag(event) {
  const seal = preview.value?.snapshot?.seal
  if (!seal || !previewSealStage.value || sealDragSaving.value) return
  event.preventDefault()
  const scale = previewSealScale()
  const originX = Number(seal.x_mm || 32)
  const originY = Number(seal.y_mm || 22)
  const startX = event.clientX
  const startY = event.clientY
  const update = (clientX, clientY) => {
    seal.x_mm = Math.max(1, Math.round(originX + (clientX - startX) / scale))
    seal.y_mm = Math.max(1, Math.round(originY + (clientY - startY) / scale))
  }
  update(event.clientX, event.clientY)
  const move = (moveEvent) => update(moveEvent.clientX, moveEvent.clientY)
  const up = async () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    await savePreviewSealPosition()
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up, { once: true })
}

async function savePreviewSealPosition() {
  const seal = preview.value?.snapshot?.seal
  if (!seal) return
  sealDragSaving.value = true
  error.value = ''
  try {
    await apiSend('/api/settings/sales-order/seal-position', {
      body: {
        seal_x_mm: Number(seal.x_mm || 32),
        seal_y_mm: Number(seal.y_mm || 22),
        seal_width_mm: Number(seal.width_mm || 42),
      },
    })
    message.value = '公章位置已保存'
  } catch (err) {
    error.value = err.message || '保存公章位置失败'
  } finally {
    sealDragSaving.value = false
  }
}

async function openCustomerDrawer() {
  if (!customerSummary.id) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/customers/${customerSummary.id}`)
    assignCustomer(customerForm, data.customer || {})
    drawerOpen.value = true
  } catch (err) {
    error.value = err.message || '加载客户信息失败'
  } finally {
    loading.value = false
  }
}

function closeCustomerDrawer() {
  drawerOpen.value = false
}

function openSettingsDrawer() {
  settingsDrawerOpen.value = true
}

async function closeSettingsDrawer() {
  settingsDrawerOpen.value = false
  await loadPreview()
}

async function saveCustomer() {
  if (!customerForm.id) return
  savingCustomer.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/customers/${customerForm.id}`, {
      method: 'PUT',
      body: {
        name: customerForm.name,
        raw_name: customerForm.raw_name,
        company_name: customerForm.company_name,
        company_address: customerForm.company_address,
        company_phone: customerForm.company_phone,
        contact: customerForm.contact,
        phone: customerForm.phone,
        address: customerForm.address,
        default_source_id: customerForm.default_source_id,
        default_order_type_id: customerForm.default_order_type_id,
        active: customerForm.active,
      },
    })
    assignCustomer(customerForm, data.customer || {})
    assignCustomer(customerSummary, data.customer || {})
    message.value = '客户信息已保存'
    await loadPreview()
  } catch (err) {
    error.value = err.message || '保存客户信息失败'
  } finally {
    savingCustomer.value = false
  }
}

onMounted(loadPage)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.embedded-page { padding: 14px; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 1180px; }
.embedded-page .panel { max-width: none; }
.panel-head, .actions, .summary { display: flex; align-items: center; gap: 12px; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
.actions { flex-wrap: wrap; justify-content: flex-end; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
button, .link-button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 8px 12px; font: inherit; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.summary { color: #555; }
.manual { border-top: 1px solid #edf0f5; padding-top: 10px; margin-top: 12px; color: #4b5563; font-size: 13px; }
.manual summary { cursor: pointer; font-weight: 700; color: #111827; }
.manual ul { margin: 8px 0 0; padding-left: 18px; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; }
th { background: #fbfaf8; }
.text-link { color: #1f4f82; text-decoration: none; }
.muted { color: #666; text-align: center; }
.version-tag { display: inline-block; margin-left: 6px; border: 1px solid #d0d0d0; border-radius: 999px; padding: 1px 7px; color: #555; font-size: 12px; vertical-align: middle; }
.preview-empty { padding: 18px 0; }
.preview-box { border: 1px solid #e5ded3; border-radius: 8px; padding: 14px; background: #fffdf9; position: relative; }
.preview-title { position: relative; display: flex; justify-content: space-between; gap: 12px; border-bottom: 2px solid #1f1f1f; padding: 22px 0 10px; margin-bottom: 10px; min-height: 82px; }
.preview-title strong { font-size: 18px; }
.preview-title span { font-weight: 800; }
.seal-stamp-preview { position: absolute; object-fit: contain; opacity: .86; cursor: move; touch-action: none; }
.preview-meta { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px 14px; margin-bottom: 12px; color: #555; }
.preview-meta span { min-width: 0; overflow-wrap: anywhere; line-height: 1.45; }
.preview-total { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 14px; padding-top: 12px; }
.preview-total strong { font-size: 18px; }
.preview-notes { border-top: 1px solid #eee2d4; margin-top: 12px; padding-top: 10px; color: #555; }
.preview-notes p { margin: 4px 0; white-space: pre-line; }
.payment-info-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(260px, 360px); gap: 18px; border-top: 1px solid #eee2d4; margin-top: 12px; padding-top: 12px; }
.payment-text-panel { display: grid; gap: 10px; align-content: start; }
.payment-code-panel { align-self: start; }
.compact-notes { border-top: 0; margin-top: 0; padding-top: 0; }
.account-payment-preview p { margin: 2px 0; }
.payment-code-preview-list { display: grid; grid-template-columns: 1fr; justify-content: stretch; gap: 14px; }
.payment-code-preview { display: grid; grid-template-columns: 132px minmax(0, 1fr); gap: 12px; align-items: center; justify-items: start; text-align: left; }
.payment-code-preview img { width: 132px; height: 132px; object-fit: contain; border: 1px solid #eee2d4; border-radius: 6px; background: #fff; }
.single-payment-code .payment-code-preview { grid-template-columns: 1fr; justify-items: center; text-align: center; }
.single-payment-code .payment-code-preview img { width: 168px; height: 168px; }
.payment-code-preview strong, .payment-code-preview span { display: block; }
.payment-code-preview span { color: #666; margin-top: 4px; white-space: pre-line; font-size: 12px; line-height: 1.35; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
.drawer-mask { position: fixed; inset: 0; z-index: 40; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.drawer { width: min(520px, 100vw); height: 100%; overflow: auto; background: #fff; border-left: 1px solid #e6e0d8; padding: 16px; box-shadow: -10px 0 24px rgba(0, 0, 0, .12); }
.settings-drawer-mask { z-index: 60; }
.settings-drawer { width: min(760px, 100vw); background: #f8f7f4; }
.drawer-head, .drawer-actions { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 14px; }
.drawer-form { display: grid; gap: 12px; }
.drawer-form label span { display: block; color: #555; font-size: 12px; margin-bottom: 5px; }
.drawer-form input, .drawer-form textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 8px 10px; font: inherit; background: #fff; }
.drawer-form input { height: 38px; }
.drawer-form textarea { min-height: 82px; resize: vertical; }
.drawer-actions { justify-content: flex-end; margin: 4px 0 0; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: stretch; flex-direction: column; }
  .actions { justify-content: flex-start; }
  table { min-width: 760px; }
  .panel { overflow: auto; }
  .preview-meta { grid-template-columns: 1fr; }
  .payment-info-grid { grid-template-columns: 1fr; }
  .drawer { width: 100vw; }
}
</style>
