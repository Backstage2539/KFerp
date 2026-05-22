<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>物流设置</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>

      <div class="settings-layout">
        <div class="company-list">
          <div class="list-head">
            <h3>物流公司</h3>
            <button class="secondary" type="button" @click="newCompany">新增公司</button>
          </div>
          <button
            v-for="company in companies"
            :key="company.id"
            type="button"
            class="company-card"
            :class="{ active: Number(companyForm.id) === Number(company.id) }"
            @click="editCompany(company)"
          >
            <strong>{{ company.name }}</strong>
            <span>{{ company.products?.length || 0 }} 个产品</span>
            <small v-if="company.active === false">已停用</small>
          </button>
          <div v-if="!companies.length" class="muted">暂无物流公司</div>
        </div>

        <div class="editor">
          <div class="form-grid">
            <label><span>公司名称</span><input v-model.trim="companyForm.name" placeholder="如 顺丰" /></label>
            <label><span>排序</span><input v-model.number="companyForm.sort" type="number" /></label>
            <label class="check-row"><input v-model="companyForm.active" type="checkbox" /> <span>启用</span></label>
          </div>
          <div class="actions">
            <button class="primary" type="button" @click="saveCompany" :disabled="saving">保存公司</button>
          </div>

          <div class="product-head">
            <h3>物流公司产品列表</h3>
            <button class="secondary" type="button" @click="newProduct" :disabled="!companyForm.id">新增产品</button>
          </div>
          <div class="form-grid product-form">
            <label><span>产品名称</span><input v-model.trim="productForm.name" placeholder="如 顺丰小件" /></label>
            <label><span>排序</span><input v-model.number="productForm.sort" type="number" /></label>
            <label class="check-row"><input v-model="productForm.active" type="checkbox" /> <span>启用</span></label>
          </div>
          <div class="actions">
            <button class="primary" type="button" @click="saveProduct" :disabled="saving || !companyForm.id">保存产品</button>
          </div>

          <table>
            <thead>
              <tr><th>产品名称</th><th>排序</th><th>状态</th><th>操作</th></tr>
            </thead>
            <tbody>
              <tr v-for="product in selectedProducts" :key="product.id">
                <td>{{ product.name }}</td>
                <td>{{ product.sort }}</td>
                <td>{{ product.active === false ? '停用' : '启用' }}</td>
                <td><button class="secondary" type="button" @click="editProduct(product)">编辑</button></td>
              </tr>
              <tr v-if="!selectedProducts.length"><td colspan="4" class="muted">当前公司暂无产品</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const companies = ref([])

const companyForm = reactive(emptyCompany())
const productForm = reactive(emptyProduct())

const selectedCompany = computed(() => companies.value.find((item) => Number(item.id) === Number(companyForm.id)) || null)
const selectedProducts = computed(() => selectedCompany.value?.products || [])

function emptyCompany() {
  return { id: 0, name: '', sort: 0, active: true }
}

function emptyProduct() {
  return { id: 0, company_id: 0, name: '', sort: 0, active: true }
}

function assignCompany(company) {
  Object.assign(companyForm, emptyCompany(), company || {})
  companyForm.active = company?.active !== false
  newProduct()
}

function assignProduct(product) {
  Object.assign(productForm, emptyProduct(), product || { company_id: companyForm.id })
  productForm.company_id = Number(productForm.company_id || companyForm.id || 0)
  productForm.active = product?.active !== false
}

function newCompany() {
  ok.value = false
  assignCompany(null)
}

function editCompany(company) {
  ok.value = false
  assignCompany(company)
}

function newProduct() {
  ok.value = false
  assignProduct(null)
}

function editProduct(product) {
  ok.value = false
  assignProduct(product)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/settings/logistics')
    companies.value = data.companies || []
    const current = companies.value.find((item) => Number(item.id) === Number(companyForm.id)) || companies.value[0] || null
    assignCompany(current)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function saveCompany() {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    const method = companyForm.id ? 'PUT' : 'POST'
    const url = companyForm.id ? `/api/settings/logistics/companies/${companyForm.id}` : '/api/settings/logistics/companies'
    const data = await apiSend(url, { method, body: companyForm })
    ok.value = true
    await load()
    if (data.company?.id) editCompany(data.company)
  } catch (err) {
    error.value = err.message || '保存公司失败'
  } finally {
    saving.value = false
  }
}

async function saveProduct() {
  if (!companyForm.id) return
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    productForm.company_id = Number(companyForm.id)
    const method = productForm.id ? 'PUT' : 'POST'
    const url = productForm.id ? `/api/settings/logistics/products/${productForm.id}` : '/api/settings/logistics/products'
    await apiSend(url, { method, body: productForm })
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '保存产品失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; max-width: 1180px; }
.panel-head, .list-head, .actions, .product-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
.settings-layout { display: grid; grid-template-columns: 320px 1fr; gap: 14px; align-items: start; }
.company-list { border-right: 1px solid #eee8df; padding-right: 14px; }
.company-card { width: 100%; height: auto; display: grid; gap: 4px; text-align: left; border-color: #e0d8ce; margin-bottom: 8px; padding: 10px; background: #fff; }
.company-card.active { border-color: #1f1f1f; background: #fbfaf8; }
.company-card span { color: #555; font-size: 13px; }
.company-card small { color: #9f1239; font-size: 12px; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(160px, 1fr)); gap: 10px; margin-bottom: 12px; }
.product-form { padding-top: 4px; }
.check-row { flex-direction: row; align-items: center; gap: 8px; padding-top: 22px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; }
.check-row input { width: 16px; height: 16px; padding: 0; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
table { width: 100%; border-collapse: collapse; margin-top: 8px; }
th, td { border-bottom: 1px solid #eee8df; padding: 10px; text-align: left; }
th { background: #faf8f5; font-weight: 700; }
.muted { color: #666; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .settings-layout, .form-grid { grid-template-columns: 1fr; }
  .company-list { border-right: 0; border-bottom: 1px solid #eee8df; padding-right: 0; padding-bottom: 12px; }
  .check-row { padding-top: 0; }
}
</style>
