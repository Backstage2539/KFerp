<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { fixedPricePath } from '../lib/wechat-official'
import { groupPriceEntries, priceEntryPurposeLabels } from '../lib/wechat-price-entry-groups'

const props = defineProps({ publicationId: { type: Number, default: 0 } })
const rows = ref([])
const versions = ref({})
const loadedVersions = ref({})
const search = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)
const preview = ref(null)

const groups = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return groupPriceEntries(rows.value)
    .map((group) => ({
      ...group,
      entries: group.entries.filter((row) =>
        !query || `${group.name} ${row.table_name} ${row.version} ${row.owner_key}`.toLocaleLowerCase().includes(query),
      ),
    }))
    .filter((group) => group.entries.length)
})

async function load() {
  busy.value = true
  error.value = ''
  try {
    const query = props.publicationId ? `?publication_id=${props.publicationId}` : ''
    rows.value = (await apiGet(`/api/customer-portal/admin/wechat/entries${query}`)).rows || []
    versions.value = {}
    loadedVersions.value = {}
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function loadVersions(row) {
  if (loadedVersions.value[row.key]) return
  try {
    versions.value[row.key] = (
      await apiGet(`/api/customer-portal/admin/wechat/entries/${row.key}/versions`)
    ).rows || []
    loadedVersions.value[row.key] = true
  } catch (e) {
    error.value = e.message
  }
}

function selectedVersion(row) {
  return (versions.value[row.key] || []).find((version) => Number(version.id) === Number(row.publication_id))
}

function changeVersion(row) {
  const version = selectedVersion(row)
  if (!version) return
  row.version = version.version
  row.owner_type = version.owner_type
  row.owner_key = version.owner_key
  row.status = version.status
  if (version.owner_type !== 'official') row.visibility = 'authenticated'
}

async function save(row) {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    Object.assign(
      row,
      await apiSend(`/api/customer-portal/admin/wechat/entries/${row.key}`, {
        method: 'PUT',
        body: row,
      }),
    )
    notice.value = '入口已保存，公众号路径保持不变。'
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function copy(row) {
  try {
    await navigator.clipboard.writeText(fixedPricePath(row))
    notice.value = '已复制小程序跳转路径'
  } catch {
    error.value = '复制失败，请选择路径手动复制'
  }
}

async function showPreview(row) {
  try {
    preview.value = await apiGet(`/api/customer-portal/admin/wechat/entries/${row.key}/preview`)
  } catch (e) {
    error.value = e.message
  }
}

watch(() => props.publicationId, load)
onMounted(load)
</script>

<template>
  <section class="wechat-entries">
    <div class="head">
      <div>
        <h3>价格表入口</h3>
        <p>每种商品类型固定有“批发”和“一件代发”两个入口；入口目标由管理员手动选择。</p>
        <p>新增价格表或发布新版本不会增加入口，也不会自动切换已选版本。</p>
      </div>
      <button type="button" :disabled="busy" @click="load">刷新</button>
    </div>
    <input
      v-if="!publicationId"
      v-model="search"
      placeholder="搜索商品类型、已选价格表、版本或客户编号"
      aria-label="搜索价格表入口"
    />
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="notice" role="status" class="success">{{ notice }}</p>
    <p v-if="!busy && !groups.length">暂无已配置商品类型的价格表入口。</p>

    <article v-for="group in groups" :key="group.key" class="type-group">
      <header>
        <h3>{{ group.name }}</h3>
        <small>商品类型固定标识：{{ group.key }}</small>
      </header>
      <article v-for="row in group.entries" :key="row.key" class="entry">
        <div class="entry-title">
          <h4>{{ priceEntryPurposeLabels[row.purpose] || row.purpose }}</h4>
          <span>{{ row.enabled ? '已启用' : '已停用' }}</span>
        </div>
        <div class="form-grid">
          <label>
            展示价格表及版本
            <select
              v-model.number="row.publication_id"
              @focus="loadVersions(row)"
              @change="changeVersion(row)"
            >
              <option :value="0">请选择同类型的已发布价格表版本</option>
              <option
                v-if="row.publication_id && !(versions[row.key] || []).some((version) => Number(version.id) === Number(row.publication_id))"
                :value="row.publication_id"
              >
                {{ row.version || '当前选择' }} · {{ row.table_name || '价格表' }} · {{ row.owner_type === 'official' ? '公共表' : `客户专属 #${row.owner_key}` }}
              </option>
              <option
                v-for="version in versions[row.key] || []"
                :key="version.id"
                :value="version.id"
              >
                {{ version.version }} · {{ version.name || '价格表' }} ·
                {{ version.owner_type === 'official' ? '公共表' : `客户专属 #${version.owner_key}` }}
              </option>
            </select>
          </label>
          <label>
            可见范围
            <select v-model="row.visibility">
              <option value="authenticated">认证客户可见</option>
              <option v-if="row.owner_type === 'official'" value="public">公开浏览</option>
            </select>
          </label>
          <label class="enabled">
            <input v-model="row.enabled" type="checkbox" :disabled="!row.publication_id" />
            启用入口
          </label>
        </div>
        <p v-if="row.publication_id">
          {{ row.owner_type === 'official' ? '公共价格表' : `客户专属 · ${row.owner_key}` }}
          · {{ row.status === 'published' ? `${row.table_name || '价格表'} · ${row.version} 已发布` : `版本不可用：${row.status}` }}
        </p>
        <p v-else class="muted">待配置：选择同一商品类型下的价格表及发布版本后，才能启用入口。</p>
        <div class="path">
          <code>{{ fixedPricePath(row) }}</code>
          <button type="button" @click="copy(row)">复制路径</button>
        </div>
        <div class="actions">
          <button type="button" :disabled="busy || !row.publication_id" @click="save(row)">保存入口</button>
          <button type="button" :disabled="!row.publication_id" @click="showPreview(row)">预览已保存内容</button>
        </div>
      </article>
    </article>

    <div v-if="preview" class="preview">
      <div class="head">
        <h3>{{ preview.title || '价格表' }} · {{ preview.version_no }}</h3>
        <button type="button" @click="preview = null">关闭预览</button>
      </div>
      <section v-for="group in preview.groups || []" :key="group.category">
        <h4>{{ group.category }}</h4>
        <div v-for="(item, i) in group.items" :key="i" class="preview-row">
          <b>{{ item.name }}</b>
          <span>{{ item.flavor }}</span>
          <span v-for="price in item.prices || []" :key="price.label">
            {{ price.label }} {{ price.value }}
          </span>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.wechat-entries { display: grid; gap: 16px; color: #25312b; min-width: 0; }
.head, .actions, .path, .entry-title { display: flex; justify-content: space-between; gap: 12px; align-items: center; flex-wrap: wrap; }
h3, h4, p { margin: 0; }
p { font-size: 13px; color: #66746c; line-height: 1.7; }
.type-group, .entry, .preview { border: 1px solid #dfe7e2; border-radius: 12px; padding: 18px; display: grid; gap: 14px; background: white; min-width: 0; }
.type-group > header { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; flex-wrap: wrap; }
.type-group > header small, .muted { color: #78857d; font-size: 12px; }
.entry { background: #fbfcfb; padding: 14px; }
.entry-title span { color: #66746c; font-size: 12px; }
.form-grid { display: grid; grid-template-columns: minmax(220px, 2fr) minmax(150px, 1fr) minmax(120px, .7fr); gap: 14px; }
label { display: grid; gap: 8px; font-size: 13px; min-width: 0; }
.enabled { display: flex; align-items: center; }
input, select, button { font: inherit; border: 1px solid #cbd6ce; border-radius: 7px; padding: 9px; background: white; min-width: 0; }
button { cursor: pointer; color: #1e5a3b; }
button:disabled { opacity: 0.5; cursor: not-allowed; }
.path { background: #f4f7f5; padding: 10px; border-radius: 8px; }
.path code { overflow-wrap: anywhere; font-size: 12px; }
.error { color: #a12b2b; }
.success { color: #216940; }
.preview-row { display: flex; gap: 12px; flex-wrap: wrap; border-bottom: 1px solid #eee; padding: 12px 0; }
@media (max-width: 760px) {
  .type-group, .entry, .preview { padding: 12px; }
  .form-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>
