<template>
  <Teleport to="body">
    <div class="virtual-warehouse-mask" @click.self="close" @keydown="onKeydown">
      <aside ref="dialog" class="virtual-warehouse-drawer" role="dialog" aria-modal="true" :aria-labelledby="titleID" tabindex="-1">
        <header><div><h3 :id="titleID">虚拟仓库配置</h3><p>管理可绑定客户的子仓库。</p></div><button type="button" class="secondary" :disabled="saving" @click="close" aria-label="关闭虚拟仓库配置">关闭</button></header>
        <div v-if="error" class="error" role="alert">{{ error }}</div>
        <div v-if="message" class="ok" role="status">{{ message }}</div>
        <section class="warehouse-editor">
          <h4>{{ form.code ? '编辑仓库' : '新增仓库' }}</h4>
          <form @submit.prevent="save">
            <div class="fields">
              <label><span>仓库名称</span><input v-model.trim="form.name" required maxlength="120" :disabled="saving" placeholder="如 客户代发成品仓" /></label>
              <label><span>仓库类型</span><select v-model="form.kind" :disabled="saving || Boolean(form.code)"><option v-if="!['finished', 'raw', 'packaging'].includes(form.kind)" :value="form.kind">{{ kindLabel(form.kind) }}</option><option value="finished">成品仓</option><option value="raw">原料仓</option><option value="packaging">包材仓</option></select></label>
            </div>
            <label><span>绑定客户</span><SearchableSelect v-model="form.customer_id" :options="customers" :disabled="saving" placeholder="搜索客户，留空为未绑定" :option-label="customerLabel" /></label>
            <label><span>说明</span><textarea v-model.trim="form.description" rows="2" maxlength="1000" :disabled="saving" /></label>
            <p v-if="form.code" class="hint">仓库编码：{{ form.code }}。有业务历史的仓库不能更换绑定客户。</p>
            <div class="actions"><button type="submit" class="primary" :disabled="saving || !form.name.trim()">{{ saving ? '保存中…' : form.code ? '保存仓库' : '新增仓库' }}</button><button v-if="form.code" type="button" class="secondary" :disabled="saving" @click="newWarehouse">取消编辑</button></div>
          </form>
        </section>
        <section class="warehouse-list">
          <div class="list-title"><h4>虚拟仓库下的子仓库</h4><button type="button" class="secondary" :disabled="saving" @click="newWarehouse">新增</button></div>
          <div class="filters">
            <label><span>客户范围</span><select v-model="customerFilter" :disabled="loading || saving" @change="loadPage(1)"><option value="">全部客户</option><option v-if="props.customerId > 0" :value="String(props.customerId)">当前客户</option><option value="0">未绑定客户</option></select></label>
            <label><span>搜索仓库</span><input v-model.trim="query" :disabled="saving" placeholder="名称 / 编码" @keyup.enter="loadPage(1)" /></label>
            <button type="button" class="secondary" :disabled="loading || saving" @click="loadPage(1)">查询</button>
          </div>
          <p v-if="loading" role="status">正在加载仓库…</p>
          <div v-for="warehouse in rows" :key="warehouse.code" class="warehouse-row">
            <div><strong>{{ warehouse.name }}</strong><small>{{ kindLabel(warehouse.kind) }} · {{ warehouse.customer_name || (warehouse.customer_id ? `客户 ${warehouse.customer_id}` : '未绑定客户') }}</small><small>{{ warehouse.description || warehouse.code }}</small></div>
            <div class="row-actions"><button type="button" class="secondary" :disabled="saving" @click="edit(warehouse)">编辑</button><button type="button" class="danger" :disabled="saving" @click="pendingDeletion = warehouse">删除</button></div>
            <div v-if="pendingDeletion?.code === warehouse.code" class="delete-confirm" role="alert"><span>确认删除“{{ warehouse.name }}”？库存或业务引用未清理时无法删除，历史记录保留。</span><div class="actions"><button type="button" class="danger" :disabled="saving" @click="remove(warehouse)">确认删除</button><button type="button" class="secondary" :disabled="saving" @click="pendingDeletion = null">取消</button></div></div>
          </div>
          <p v-if="!loading && !rows.length" class="empty">当前范围暂无虚拟子仓，可在上方新增。</p>
          <PaginationControls :page="page" :page-size="limit" :total="total" :disabled="loading || saving" @change="changePage" />
        </section>
      </aside>
    </div>
  </Teleport>
</template>
<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import SearchableSelect from './SearchableSelect.vue'
import PaginationControls from './PaginationControls.vue'
import { apiGet, apiSend } from '../api/client'
import { fetchAllCustomerOptions } from '../api/view-context'
const props = defineProps({ customerId: { type: Number, default: 0 } })
const emit = defineEmits(['close', 'changed'])
const rows = ref([]), customers = ref([]), customerFilter = ref(props.customerId > 0 ? String(props.customerId) : ''), query = ref(''), page = ref(1), limit = ref(10), total = ref(0), loading = ref(false), saving = ref(false), error = ref(''), message = ref(''), pendingDeletion = ref(null), dialog = ref(null)
const titleID = `virtual-warehouse-title-${Math.random().toString(36).slice(2)}`
const form = ref(emptyForm())
let sequence = 0, active = true, previousFocus, previousOverflow
function emptyForm() { return { code: '', name: '', kind: 'finished', description: '', customer_id: Number(props.customerId || 0), revision: 0 } }
function customerLabel(row) { return [row.name, row.company_name, row.phone].filter(Boolean).join(' · ') }
function kindLabel(kind) { return ({ finished: '成品仓', raw: '原料仓', packaging: '包材仓', customer_finished: '成品仓', customer_processing: '客户仓' })[kind] || kind }
function newWarehouse() { form.value = emptyForm(); error.value = ''; message.value = ''; pendingDeletion.value = null }
function edit(row) { form.value = { ...emptyForm(), ...row, customer_id: Number(row.customer_id || 0) }; error.value = ''; message.value = ''; pendingDeletion.value = null }
function close() { if (!saving.value) emit('close') }
function onKeydown(event) {
  if (event.key === 'Escape') { event.preventDefault(); close(); return }
  if (event.key !== 'Tab' || !dialog.value) return
  const controls = [...dialog.value.querySelectorAll('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]')].filter(el => el.getClientRects().length)
  const first = controls[0], last = controls.at(-1)
  if (!first) { event.preventDefault(); dialog.value.focus() }
  else if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && (document.activeElement === last || document.activeElement === dialog.value)) { event.preventDefault(); first.focus() }
}
async function loadPage(nextPage = 1) {
  const request = ++sequence
  loading.value = true; error.value = ''
  const params = new URLSearchParams({ page: String(nextPage), limit: String(limit.value) })
  if (customerFilter.value !== '') params.set('customer_id', customerFilter.value)
  if (query.value) params.set('q', query.value)
  try {
    const data = await apiGet(`/api/stock/virtual-warehouses?${params}`)
    if (!active || request !== sequence) return
    rows.value = data.rows || []; total.value = Number(data.total || 0); page.value = Number(data.page || nextPage)
  } catch (err) { if (active && request === sequence) error.value = err.message || '仓库加载失败' }
  finally { if (request === sequence) loading.value = false }
}
function changePage(value) { limit.value = value.pageSize; loadPage(value.page) }
async function save() {
  if (saving.value || !form.value.name.trim()) return
  saving.value = true; error.value = ''; message.value = ''
  const editing = Boolean(form.value.code)
  const body = { name: form.value.name.trim(), description: form.value.description.trim(), customer_id: Number(form.value.customer_id || 0) }
  if (editing) body.expected_revision = Number(form.value.revision)
  else body.kind = form.value.kind
  try {
    const warehouse = await apiSend(editing ? `/api/stock/virtual-warehouses/${encodeURIComponent(form.value.code)}` : '/api/stock/virtual-warehouses', { method: editing ? 'PUT' : 'POST', body })
    if (!active) return
    form.value = { ...emptyForm(), ...warehouse }; message.value = editing ? '仓库已保存' : '仓库已新增'; emit('changed', { action: editing ? 'update' : 'create', warehouse })
    await loadPage(editing ? page.value : 1)
  } catch (err) { if (active) error.value = err.message || '仓库保存失败' }
  finally { saving.value = false }
}
async function remove(warehouse) {
  if (saving.value) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    await apiSend(`/api/stock/virtual-warehouses/${encodeURIComponent(warehouse.code)}?expected_revision=${Number(warehouse.revision)}`, { method: 'DELETE' })
    if (!active) return
    if (form.value.code === warehouse.code) form.value = emptyForm()
    pendingDeletion.value = null; message.value = '仓库已删除，历史记录保留'; emit('changed', { action: 'delete', warehouse })
    await loadPage(rows.value.length === 1 && page.value > 1 ? page.value - 1 : page.value)
  } catch (err) { if (active) error.value = err.message || '仓库删除失败' }
  finally { saving.value = false }
}
onMounted(async () => {
  previousFocus = document.activeElement; previousOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'
  await nextTick(); dialog.value?.focus()
  await Promise.all([loadPage(1), fetchAllCustomerOptions().then(value => { if (active) customers.value = value })])
})
onBeforeUnmount(() => { active = false; sequence++; if (typeof document !== 'undefined') { document.body.style.overflow = previousOverflow || ''; previousFocus?.focus?.() } })
</script>
<style scoped>
*{box-sizing:border-box}.virtual-warehouse-mask{position:fixed;inset:0;background:rgba(15,30,22,.3);display:flex;justify-content:flex-end;z-index:1200}.virtual-warehouse-drawer{width:min(720px,100%);height:100%;overflow:auto;overscroll-behavior:contain;background:#fff;padding:22px;box-shadow:-8px 0 28px #15382422;color:#244334}.virtual-warehouse-drawer:focus{outline:none}header,.list-title{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}h3{margin:0;font-size:20px}h4{margin:0 0 14px;font-size:15px}p{font-size:12px;line-height:1.6;color:#617469}header p{margin:6px 0 20px}.warehouse-editor{background:#f6faf7;border:1px solid #dce7df;border-radius:10px;padding:16px;margin-bottom:24px}label{display:grid;gap:6px;font-size:12px;margin-bottom:12px;min-width:0}.fields{display:grid;grid-template-columns:1fr 130px;gap:12px}input,select,textarea{font:inherit;width:100%;min-width:0;border:1px solid #cbd9cf;background:#fff;border-radius:6px;padding:9px}textarea{resize:vertical}.actions,.row-actions{display:flex;gap:8px;flex-wrap:wrap}button{font:inherit;font-size:12px;cursor:pointer;min-height:36px;padding:8px 12px;border-radius:6px;white-space:nowrap}button:disabled{opacity:.5;cursor:default}.primary{background:#28624a;color:#fff;border:1px solid #28624a}.secondary{background:#fff;color:#28624a;border:1px solid #c3d4c9}.danger{background:#fff;color:#ac3129;border:1px solid #e7bfb9}.filters{display:grid;grid-template-columns:130px 1fr auto;align-items:end;gap:10px;margin-bottom:12px}.filters label{margin:0}.warehouse-row{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px;border-bottom:1px solid #e5ece7;padding:15px 0}.warehouse-row strong{font-size:14px;overflow-wrap:anywhere}.warehouse-row small{display:block;color:#65776c;font-size:12px;margin-top:5px;overflow-wrap:anywhere}.row-actions{align-self:start}.delete-confirm{grid-column:1/-1;display:grid;gap:10px;padding:12px;background:#fff5f1;border-radius:6px;font-size:12px;line-height:1.6}.error,.ok{font-size:13px;border-radius:6px;padding:10px;margin-bottom:12px;line-height:1.6}.error{color:#ab3028;background:#fff0eb}.ok{color:#28624a;background:#edf8ef}.empty{text-align:center;padding:20px 0}.hint{margin:0 0 12px}@media(max-width:600px){.virtual-warehouse-drawer{padding:16px}.fields,.filters{grid-template-columns:1fr}.filters button{justify-self:start}.warehouse-row{grid-template-columns:1fr}.row-actions{justify-content:flex-start}}
</style>
