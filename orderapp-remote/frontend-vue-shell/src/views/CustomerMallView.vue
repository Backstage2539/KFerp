<template>
  <section class="panel">
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="message">{{ message }}</p>
    <h3>商城商品</h3>
    <table>
      <thead>
        <tr>
          <th>商品</th>
          <th>规格</th>
          <th>单价</th>
          <th>数量</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="row in products" :key="row.id">
        <tr>
          <td>{{ row.title || row.product_name }}</td>
          <td>{{ row.spec_name || `${row.spec_g}g` }}</td>
          <td>{{ Number(row.mall_price || row.unit_price).toFixed(2) }}</td>
          <td><input v-model.number="counts[row.id]" type="number" min="0" step="1" /></td>
        </tr>
        <tr v-if="counts[row.id] > 0">
          <td colspan="4"><OrderOperationNotes ref="operationNoteEditors" v-model="operationNotes[row.id]" v-model:context="operationNoteContexts[row.id]" :options-url="`/api/customer-processing/portal/mall/operation-options?customer_id=${props.customerId}&mall_product_id=${row.id}`" :disabled="saving" /></td>
        </tr>
        </template>
        <tr v-if="!products.length">
          <td colspan="4">暂无上架商品</td>
        </tr>
      </tbody>
    </table>
    <form @submit.prevent="submit">
      <label>收件人<input v-model.trim="form.recipient_name" required /></label
      ><label>电话<input v-model.trim="form.recipient_phone" required /></label
      ><label>地址<input v-model.trim="form.recipient_address" required /></label
      ><label>备注<input v-model.trim="form.note" /></label
      ><button :disabled="saving || !items.length">提交商城订单</button>
    </form>
  </section>
</template>
<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import OrderOperationNotes from '../components/OrderOperationNotes.vue'
const props = defineProps({ customerId: { type: Number, required: true } })
const products = ref([]),
  counts = reactive({}),
  operationNotes = reactive({}),
  operationNoteContexts = reactive({}),
  operationNoteEditors = ref([]),
  error = ref(''),
  message = ref(''),
  saving = ref(false),
  form = reactive({ recipient_name: '', recipient_phone: '', recipient_address: '', note: '' })
const items = computed(() =>
  products.value
    .filter((p) => counts[p.id] > 0)
    .map((p) => ({ mall_product_id: p.id, product_id: p.product_id, bom_spec_id: p.bom_spec_id || 0, bom_variant_id: p.bom_variant_id || 0, qty: counts[p.id], sales_unit: p.sales_units?.[0] || '', operation_notes: operationNotes[p.id] }))
)
onMounted(async () => {
  try {
    products.value =
      (await apiGet(`/api/customer-processing/portal/mall?customer_id=${props.customerId}`)).products || []
  } catch (e) {
    error.value = e.message
  }
})
async function submit() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    for (const editor of operationNoteEditors.value) {
      if (!await editor.validate()) { error.value = '请核对商品的工序备注；已填内容已保留。'; return }
    }
    const result = await apiSend(`/api/customer-processing/portal/mall/orders?customer_id=${props.customerId}`, {
      body: { ...form, items: items.value }
    })
    message.value = `订单已提交 ${result.order_no || ''}`
    for (const key of Object.keys(counts)) delete counts[key]
    for (const key of Object.keys(operationNotes)) delete operationNotes[key]
    for (const key of Object.keys(operationNoteContexts)) delete operationNoteContexts[key]
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
</script>
<style scoped>
.panel {
  background: white;
  padding: 20px;
}
table {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  padding: 12px;
  border-bottom: 1px solid #ddd;
  text-align: left;
}
form {
  display: grid;
  gap: 12px;
  margin-top: 20px;
}
label {
  display: grid;
  gap: 5px;
}
input {
  padding: 8px;
  border: 1px solid #ccd3cd;
  border-radius: 5px;
}
td input[type='number'] { width: 88px; max-width: 100%; }
button {
  padding: 10px;
}
</style>
