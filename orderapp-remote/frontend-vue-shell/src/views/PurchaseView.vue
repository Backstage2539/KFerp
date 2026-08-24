<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>采购入库</h2>
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
          <label><span>物料</span><select v-model.number="orderForm.material_id" required><option :value="0">请选择</option><option v-for="m in purchasableMaterials" :key="m.id" :value="m.id">{{ m.name }} · {{ m.purchase_price || 0 }}元/{{ materialUnit(m.id) }}</option></select></label>
          <label><span>数量(g)</span><input v-model.number="orderForm.qty_g" type="number" min="1" required /></label>
          <label><span>单价（元/{{ selectedPurchaseMaterialUnit }}）</span><input v-model.number="orderForm.unit_cost" type="number" min="0" step="0.0001" /></label>
          <label><span>备注</span><input v-model.trim="orderForm.note" /></label>
          <small class="form-help">当前采购单按克记录数量，仅支持重量物料；件、袋、盒等物料请使用原料入库按库存单位录入。</small>
          <button class="primary" type="submit" :disabled="saving || !isSelectedPurchaseMaterialWeight">创建采购单</button>
        </form>
      </div>
    </section>

    <section class="panel">
      <h3>待收货采购单</h3>
      <div class="table-wrap">
        <table>
          <thead><tr><th>单号</th><th>供应商</th><th>物料</th><th>数量(g)</th><th>单价</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in orders" :key="row.id">
              <td>{{ row.order_no }}</td>
              <td>{{ row.supplier_name || supplierName(row.supplier_id) }}</td>
              <td>{{ row.material_name || materialName(row.material_id) }}</td>
              <td>{{ row.qty_g }}</td>
              <td>{{ row.unit_cost }} 元/{{ materialUnit(row.material_id) }}</td>
              <td>{{ row.status }}</td>
              <td><button class="secondary" type="button" @click="receiveOrder(row)" :disabled="saving || row.status === 'received' || !isPurchasableMaterialByID(row.material_id)" :title="isPurchasableMaterialByID(row.material_id) ? '' : '半成品只能生产入库；其他非重量物料请改用原料入库'">收货入库</button></td>
            </tr>
            <tr v-if="!orders.length"><td colspan="7" class="muted">暂无采购单</td></tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <h3>收货记录</h3>
      <div class="table-wrap">
        <table>
          <thead><tr><th>收货单</th><th>采购单</th><th>供应商</th><th>物料</th><th>数量(g)</th><th>单价</th><th>库存批次</th><th>时间</th></tr></thead>
          <tbody>
            <tr v-for="row in receipts" :key="row.id">
              <td>{{ row.receipt_no }}</td>
              <td>{{ row.purchase_order_id || '-' }}</td>
              <td>{{ row.supplier_name }}</td>
              <td>{{ row.material_name || materialName(row.material_id) }}</td>
              <td>{{ row.qty_g }}</td>
              <td>{{ row.unit_cost }} 元/{{ materialUnit(row.material_id) }}</td>
              <td>{{ row.stock_batch_code }}</td>
              <td>{{ row.created_at }}</td>
            </tr>
            <tr v-if="!receipts.length"><td colspan="8" class="muted">暂无收货记录</td></tr>
          </tbody>
        </table>
      </div>
    </section>
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
const orders = ref([])
const receipts = ref([])

const supplierForm = reactive({ name: '', contact: '', phone: '', address: '' })
const orderForm = reactive({ supplier_id: 0, material_id: 0, qty_g: 0, unit_cost: 0, note: '' })
const purchasableMaterials = computed(() => materials.value.filter((material) => isMaterialWeight(material) && !isSemiFinishedMaterial(material)))
const selectedPurchaseMaterialUnit = computed(() => materialUnit(orderForm.material_id))
const isSelectedPurchaseMaterialWeight = computed(() => isPurchasableMaterialByID(orderForm.material_id))

function supplierName(id) {
  return suppliers.value.find((row) => Number(row.id) === Number(id))?.name || ''
}

function materialName(id) {
  return materials.value.find((row) => Number(row.id) === Number(id))?.name || ''
}

function materialUnit(id) {
  const material = materials.value.find((row) => Number(row.id) === Number(id))
  const inventoryUnit = String(material?.unit || material?.Unit || '').trim()
  return inventoryUnit || '库存单位'
}

function isMaterialWeight(material) {
  const inventoryUnit = String(material?.unit || material?.Unit || '').trim().toLowerCase()
  return ['g', 'kg', 'lb', 'oz', '克', '千克'].includes(inventoryUnit)
}

function isPurchasableMaterialByID(id) {
  const material = materials.value.find((row) => Number(row.id) === Number(id))
  return isMaterialWeight(material) && !isSemiFinishedMaterial(material)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [supplierData, materialData, orderData, receiptData] = await Promise.all([
      apiGet('/api/purchase/suppliers'),
      apiGet('/api/materials?limit=500'),
      apiGet('/api/purchase/orders'),
      apiGet('/api/purchase/receipts'),
    ])
    suppliers.value = supplierData.rows || []
    materials.value = materialData.rows || []
    orders.value = orderData.rows || []
    receipts.value = receiptData.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function saveSupplier() {
  saving.value = true
  message.value = ''
  error.value = ''
  try {
    await apiSend('/api/purchase/suppliers', { body: supplierForm })
    supplierForm.name = ''
    supplierForm.contact = ''
    supplierForm.phone = ''
    supplierForm.address = ''
    message.value = '供应商已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function createOrder() {
  if (!isSelectedPurchaseMaterialWeight.value) {
    error.value = '半成品只能通过生产入库；采购单当前仅支持其他重量物料。'
    return
  }
  saving.value = true
  message.value = ''
  error.value = ''
  try {
    await apiSend('/api/purchase/orders', { body: orderForm })
    orderForm.material_id = 0
    orderForm.qty_g = 0
    orderForm.unit_cost = 0
    orderForm.note = ''
    message.value = '采购单已创建'
    await loadAll()
  } catch (err) {
    error.value = err.message || '创建失败'
  } finally {
    saving.value = false
  }
}

async function receiveOrder(row) {
  if (!isPurchasableMaterialByID(row.material_id)) {
    error.value = '半成品只能通过生产入库；其他非重量物料请使用原料入库按库存单位录入。'
    return
  }
  saving.value = true
  message.value = ''
  error.value = ''
  try {
    await apiSend('/api/purchase/receipts', {
      body: {
        purchase_order_id: row.id,
        supplier_id: row.supplier_id,
        supplier_name: row.supplier_name || supplierName(row.supplier_id),
        material_id: row.material_id,
        qty_g: row.qty_g,
        unit_cost: row.unit_cost,
        note: `采购单 ${row.order_no} 收货`,
      },
    })
    message.value = '已收货入库并更新物料采购价'
    await loadAll()
  } catch (err) {
    error.value = err.message || '收货失败'
  } finally {
    saving.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e2e2dc; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
h2, h3 { margin: 0 0 12px; }
.grid.two { display: grid; grid-template-columns: repeat(2, minmax(260px, 1fr)); gap: 14px; }
.box { border: 1px solid #ece7df; border-radius: 8px; padding: 12px; }
label { display: block; margin-bottom: 9px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.form-help { display: block; color: #666; font-size: 12px; line-height: 1.5; margin: -1px 0 9px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 36px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.notice { background: #eef8f1; border: 1px solid #b9dfc4; color: #1f6b38; border-radius: 6px; padding: 9px; margin: 12px 0; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin: 12px 0; color: #8a1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 860px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfaf8; }
.muted { color: #666; text-align: center; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .grid.two { grid-template-columns: 1fr; }
}
</style>
