<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div><h2>采购入库</h2><p>采购单记录预计数量和预计单价；确认收货后才更新库存、批次成本和物料最近采购入库价。</p></div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="notice">{{ message }}</div>
      <div class="grid two">
        <form class="box" @submit.prevent="saveSupplier">
          <h3>供应商</h3>
          <label><span>名称</span><input v-model.trim="supplierForm.name" required /></label>
          <label><span>联系人</span><input v-model.trim="supplierForm.contact" /></label>
          <label><span>电话</span><input v-model.trim="supplierForm.phone" /></label>
          <label><span>地址</span><input v-model.trim="supplierForm.address" /></label>
          <button class="primary" type="submit" :disabled="saving">保存供应商</button>
        </form>

        <form class="box" @submit.prevent="createOrder">
          <h3>采购单</h3>
          <label><span>供应商</span><select v-model.number="orderForm.supplier_id" required><option :value="0">请选择</option><option v-for="s in suppliers" :key="s.id" :value="s.id">{{ s.name }}</option></select></label>
          <label><span>物料</span><select v-model.number="orderForm.material_id" required @change="applyOrderMaterialDefaults"><option :value="0">请选择</option><option v-for="m in purchasableMaterials" :key="m.id" :value="m.id">{{ m.name }} · 最近 {{ m.purchase_price || 0 }}元/{{ materialUnit(m.id) }}</option></select></label>
          <label><span>预计数量（{{ selectedPurchaseMaterialUnit }}）</span><input v-model.number="orderForm.qty" type="number" :min="selectedPurchaseMaterialUsesCount ? 1 : 0.001" :step="selectedPurchaseMaterialUsesCount ? 1 : 0.001" required /></label>
          <label><span>库存单位</span><input v-model="orderForm.unit_code" disabled /></label>
          <label><span>预计单价（元/{{ selectedPurchaseMaterialUnit }}）</span><input v-model.number="orderForm.unit_cost" type="number" min="0" step="0.0001" /></label>
          <label><span>预计入库仓</span><select v-model="orderForm.target_warehouse"><option v-for="w in purchaseWarehouses" :key="w.code" :value="w.code">{{ w.name }}</option></select></label>
          <label class="wide"><span>备注</span><input v-model.trim="orderForm.note" /></label>
          <small class="form-help wide">采购单不会提前改变正式库存成本；kg、袋、件、盒等物料均按物料档案库存单位采购。</small>
          <button class="primary" type="submit" :disabled="saving || !orderForm.material_id">创建采购单</button>
        </form>
      </div>
    </section>

    <section class="panel">
      <h3>采购单</h3>
      <div class="table-wrap">
        <table>
          <thead><tr><th>单号</th><th>供应商</th><th>物料</th><th>预计数量</th><th>预计单价</th><th>目标仓</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in orders" :key="row.id">
              <td>{{ row.order_no }}</td><td>{{ row.supplier_name || supplierName(row.supplier_id) }}</td><td>{{ row.material_name || materialName(row.material_id) }}</td>
              <td>{{ purchaseQuantity(row) }} {{ row.unit_code || materialUnit(row.material_id) }}</td><td>{{ row.unit_cost }} 元/{{ row.unit_code || materialUnit(row.material_id) }}</td>
              <td>{{ warehouseName(row.target_warehouse) }}</td><td>{{ row.status }}</td>
              <td><button class="secondary" type="button" @click="openReceiptConfirmation(row)" :disabled="saving || row.status === 'received' || !isPurchasableMaterialByID(row.material_id)">确认收货</button></td>
            </tr>
            <tr v-if="!orders.length"><td colspan="8" class="muted">暂无采购单</td></tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <h3>收货记录</h3>
      <div class="table-wrap"><table><thead><tr><th>收货单</th><th>采购单</th><th>供应商</th><th>物料</th><th>实际数量</th><th>最终单价</th><th>入库仓</th><th>库存批次</th><th>时间</th></tr></thead><tbody>
        <tr v-for="row in receipts" :key="row.id"><td>{{ row.receipt_no }}</td><td>{{ row.purchase_order_id || '-' }}</td><td>{{ row.supplier_name }}</td><td>{{ row.material_name || materialName(row.material_id) }}</td><td>{{ purchaseQuantity(row) }} {{ row.unit_code || materialUnit(row.material_id) }}</td><td>{{ row.unit_cost }} 元/{{ row.unit_code || materialUnit(row.material_id) }}</td><td>{{ warehouseName(row.target_warehouse) }}</td><td>{{ row.stock_batch_code }}</td><td>{{ row.created_at }}</td></tr>
        <tr v-if="!receipts.length"><td colspan="9" class="muted">暂无收货记录</td></tr>
      </tbody></table></div>
    </section>

    <div v-if="receiptForm.open" class="drawer-mask" @click.self="closeReceiptConfirmation">
      <aside class="drawer" aria-label="收货确认">
        <div class="panel-head"><div><h3>收货确认</h3><p>{{ receiptForm.order_no }} · {{ materialName(receiptForm.material_id) }}</p></div><button class="secondary" type="button" @click="closeReceiptConfirmation">关闭</button></div>
        <label><span>实际数量（{{ receiptForm.unit_code }}）</span><input v-model.number="receiptForm.qty" type="number" :min="isWeightUnit(receiptForm.unit_code) ? 0.001 : 1" :step="isWeightUnit(receiptForm.unit_code) ? 0.001 : 1" /></label>
        <label><span>最终单价（元/{{ receiptForm.unit_code }}）</span><input v-model.number="receiptForm.unit_cost" type="number" min="0" step="0.0001" /></label>
        <label><span>目标仓库</span><select v-model="receiptForm.target_warehouse"><option v-for="w in purchaseWarehouses" :key="w.code" :value="w.code">{{ w.name }}</option></select></label>
        <label><span>备注</span><input v-model.trim="receiptForm.note" /></label>
        <div class="drawer-actions"><button class="primary" type="button" @click="confirmReceipt" :disabled="saving">确认收货并过账</button></div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { isSemiFinishedMaterial } from '../lib/material-receipts'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const suppliers = ref([])
const materials = ref([])
const warehouses = ref([])
const orders = ref([])
const receipts = ref([])
const supplierForm = reactive({ name: '', contact: '', phone: '', address: '' })
const orderForm = reactive({ supplier_id: 0, material_id: 0, qty: 0, unit_code: '', qty_units: 0, unit_cost: 0, target_warehouse: 'raw_materials', note: '' })
const receiptForm = reactive({ open: false, purchase_order_id: 0, order_no: '', supplier_id: 0, supplier_name: '', material_id: 0, qty: 0, unit_code: '', qty_units: 0, unit_cost: 0, target_warehouse: 'raw_materials', note: '' })
const purchasableMaterials = computed(() => materials.value.filter((material) => !isSemiFinishedMaterial(material)))
const selectedPurchaseMaterialUnit = computed(() => materialUnit(orderForm.material_id))
const selectedPurchaseMaterialUsesCount = computed(() => !isWeightUnit(selectedPurchaseMaterialUnit.value))
const purchaseWarehouses = computed(() => warehouses.value.filter((row) => row.active !== false && String(row.kind || '').toLowerCase() !== 'finished'))

function supplierName(id) { return suppliers.value.find((row) => Number(row.id) === Number(id))?.name || '' }
function materialName(id) { return materials.value.find((row) => Number(row.id) === Number(id))?.name || '' }
function materialByID(id) { return materials.value.find((row) => Number(row.id) === Number(id)) || null }
function materialUnit(id) { const material = materialByID(id); return String(material?.unit || material?.Unit || '').trim() || '库存单位' }
function isPurchasableMaterialByID(id) { const material = materialByID(id); return Boolean(material && !isSemiFinishedMaterial(material)) }
function warehouseName(code) { return warehouses.value.find((row) => row.code === code)?.name || code || '-' }
function purchaseQuantity(row = {}) { return Number(row.qty || 0) || (Number(row.qty_units || 0) || Number(row.qty_g || 0) / 1000) }
function isWeightUnit(unit) { return ['g', 'kg', 'lb', 'oz', '克', '千克', '公斤', '磅', '盎司'].includes(String(unit || '').trim().toLowerCase()) }
function defaultWarehouseForMaterial(material = {}) { return String(material.kind || '').toLowerCase() === 'pack' ? 'packaging' : 'raw_materials' }

function applyOrderMaterialDefaults() {
  const material = materialByID(orderForm.material_id)
  orderForm.unit_code = materialUnit(orderForm.material_id)
  orderForm.target_warehouse = defaultWarehouseForMaterial(material || {})
  orderForm.unit_cost = Number(material?.purchase_price || 0)
}

async function loadAll() {
  loading.value = true; error.value = ''
  try {
    const [supplierData, materialData, warehouseData, orderData, receiptData] = await Promise.all([
      apiGet('/api/purchase/suppliers'), apiGet('/api/materials?limit=500'), apiGet('/api/stock/warehouses'), apiGet('/api/purchase/orders'), apiGet('/api/purchase/receipts'),
    ])
    suppliers.value = supplierData.rows || []; materials.value = materialData.rows || []; warehouses.value = warehouseData.rows || []; orders.value = orderData.rows || []; receipts.value = receiptData.rows || []
  } catch (err) { error.value = err.message || '加载失败' } finally { loading.value = false }
}

async function saveSupplier() {
  saving.value = true; message.value = ''; error.value = ''
  try {
    await apiSend('/api/purchase/suppliers', { body: supplierForm })
    Object.assign(supplierForm, { name: '', contact: '', phone: '', address: '' }); message.value = '供应商已保存'; await loadAll()
  } catch (err) { error.value = err.message || '保存失败' } finally { saving.value = false }
}

async function createOrder() {
  saving.value = true; message.value = ''; error.value = ''
  try {
    await apiSend('/api/purchase/orders', { body: orderForm })
    Object.assign(orderForm, { material_id: 0, qty: 0, unit_code: '', qty_units: 0, unit_cost: 0, target_warehouse: 'raw_materials', note: '' })
    message.value = '采购单已创建；正式成本将在确认收货后更新'; await loadAll()
  } catch (err) { error.value = err.message || '创建失败' } finally { saving.value = false }
}

function openReceiptConfirmation(row) {
  Object.assign(receiptForm, {
    open: true, purchase_order_id: row.id, order_no: row.order_no, supplier_id: row.supplier_id,
    supplier_name: row.supplier_name || supplierName(row.supplier_id), material_id: row.material_id,
    qty: purchaseQuantity(row), unit_code: row.unit_code || materialUnit(row.material_id), qty_units: 0,
    unit_cost: Number(row.unit_cost || 0), target_warehouse: row.target_warehouse || defaultWarehouseForMaterial(materialByID(row.material_id) || {}),
    note: `采购单 ${row.order_no} 收货`,
  })
}
function closeReceiptConfirmation() { receiptForm.open = false }

async function confirmReceipt() {
  saving.value = true; message.value = ''; error.value = ''
  try {
    const body = { ...receiptForm }; delete body.open; delete body.order_no
    await apiSend('/api/purchase/receipts', { body })
    closeReceiptConfirmation(); message.value = '已完成收货过账并更新物料最近采购入库价'; await loadAll()
  } catch (err) { error.value = err.message || '收货失败' } finally { saving.value = false }
}

onMounted(loadAll)
</script>

<style scoped>
*{box-sizing:border-box}.page{padding:18px;color:#171717}.panel{border:1px solid #e2e2dc;border-radius:8px;background:#fff;padding:14px;margin-bottom:14px}.panel-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.panel-head p{margin:4px 0 0;color:#666;font-size:13px}h2,h3{margin:0 0 12px}.grid.two{display:grid;grid-template-columns:repeat(2,minmax(260px,1fr));gap:14px}.box{border:1px solid #ece7df;border-radius:8px;padding:12px;display:grid;grid-template-columns:1fr 1fr;gap:9px}.box h3,.box button,.box .wide{grid-column:1/-1}label{display:block;margin-bottom:9px}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}.form-help{display:block;color:#666;font-size:12px;line-height:1.5}input,select{width:100%;height:38px;border:1px solid #cfc8bf;border-radius:6px;padding:7px 9px;font:inherit;background:#fff}button{height:36px;border-radius:6px;border:1px solid #1f1f1f;padding:0 12px;font:inherit;cursor:pointer}button:disabled{opacity:.55;cursor:not-allowed}.primary{background:#1f1f1f;color:#fff}.secondary{background:#fff;color:#1f1f1f}.notice{background:#eef8f1;border:1px solid #b9dfc4;color:#1f6b38;border-radius:6px;padding:9px;margin:12px 0}.error{background:#fff0f0;border:1px solid #e6b7b7;border-radius:6px;padding:9px;margin:12px 0;color:#8a1f1f}.table-wrap{overflow:auto}table{width:100%;min-width:940px;border-collapse:collapse}th,td{border-bottom:1px solid #eee8df;padding:9px 8px;text-align:left;font-size:14px}th{background:#fbfaf8}.muted{color:#666;text-align:center}.drawer-mask{position:fixed;inset:0;background:rgba(17,24,39,.35);z-index:90;display:flex;justify-content:flex-end}.drawer{width:min(460px,96vw);height:100%;background:#fff;padding:18px;box-shadow:-12px 0 32px rgba(15,23,42,.2);overflow:auto}.drawer-actions{display:flex;justify-content:flex-end;border-top:1px solid #eee;padding-top:14px}@media(max-width:900px){.page{padding:12px}.grid.two,.box{grid-template-columns:1fr}.box h3,.box button,.box .wide{grid-column:auto}}
</style>
