<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { fixedPricePath } from '../lib/wechat-official'
const props = defineProps({ publicationId: { type: Number, default: 0 } })
const rows = ref([]),
  versions = ref({}),
  search = ref(''),
  error = ref(''),
  notice = ref(''),
  busy = ref(false),
  preview = ref(null)
const visible = computed(() =>
  rows.value.filter(
    (r) =>
      !search.value ||
      `${r.name} ${r.version} ${r.owner_key}`.includes(search.value),
  ),
)
async function load() {
  busy.value = true
  error.value = ''
  try {
    rows.value =
      (
        await apiGet(
          `/api/customer-portal/admin/wechat/entries${props.publicationId ? `?publication_id=${props.publicationId}` : ''}`,
        )
      ).rows || []
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
async function loadVersions(row) {
  try {
    versions.value[row.key] =
      (
        await apiGet(
          `/api/customer-portal/admin/wechat/entries/${row.key}/versions`,
        )
      ).rows || []
  } catch (e) {
    error.value = e.message
  }
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
    error.value = '复制失败，请选择下方路径手动复制'
  }
}
async function showPreview(row) {
  try {
    preview.value = await apiGet(
      `/api/customer-portal/admin/wechat/entries/${row.key}/preview`,
    )
  } catch (e) {
    error.value = e.message
  }
}
onMounted(load)
</script>
<template>
  <section class="wechat-entries">
    <div class="head">
      <div>
        <h3>固定豆单入口</h3>
        <p>选择已发布版本后保存。公众号中的跳转路径不随版本变化。</p>
      </div>
      <button type="button" :disabled="busy" @click="load">刷新</button>
    </div>
    <input
      v-if="!publicationId"
      v-model="search"
      placeholder="搜索价格表名称、版本或客户编号"
      aria-label="搜索固定入口"
    />
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="notice" role="status" class="success">{{ notice }}</p>
    <p v-if="!busy && !visible.length">暂无已发布价格表入口。</p>
    <article v-for="row in visible" :key="row.key" class="entry">
      <div class="form-grid">
        <label>
          入口名称
          <input v-model="row.name" maxlength="50" />
        </label>
        <label>
          展示版本
          <select v-model="row.publication_id" @focus="loadVersions(row)">
            <option
              v-if="
                !(versions[row.key] || []).some(
                  (v) => v.id === row.publication_id,
                )
              "
              :value="row.publication_id"
            >
              {{ row.version }} · 当前选择
            </option>
            <option
              v-for="v in versions[row.key] || []"
              :key="v.id"
              :value="v.id"
            >
              {{ v.version }} · {{ v.name || '价格表' }}
            </option>
          </select>
        </label>
        <label>
          可见范围
          <select v-model="row.visibility">
            <option value="authenticated">认证客户可见</option>
            <option v-if="row.owner_type === 'official'" value="public">
              所有人可见
            </option>
          </select>
        </label>
        <label class="enabled">
          <input v-model="row.enabled" type="checkbox" />
          启用入口
        </label>
      </div>
      <p>
        {{
          row.owner_type === 'official'
            ? '公共价格表'
            : `客户专属 · ${row.owner_key}`
        }}
        ·
        {{
          row.status === 'published' ? '已发布' : `版本不可用：${row.status}`
        }}
      </p>
      <div class="path">
        <code>{{ fixedPricePath(row) }}</code>
        <button type="button" @click="copy(row)">复制路径</button>
      </div>
      <div class="actions">
        <button type="button" :disabled="busy" @click="save(row)">
          保存入口
        </button>
        <button type="button" @click="showPreview(row)">预览已保存内容</button>
      </div>
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
.wechat-entries {
  display: grid;
  gap: 16px;
  color: #25312b;
}
.head,
.actions,
.path {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}
h3,
h4,
p {
  margin: 0;
}
p {
  font-size: 13px;
  color: #66746c;
  line-height: 1.7;
}
.entry,
.preview {
  border: 1px solid #dfe7e2;
  border-radius: 12px;
  padding: 20px;
  display: grid;
  gap: 14px;
  background: white;
}
.form-grid {
  display: grid;
  grid-template-columns: 2fr 2fr 1fr 1fr;
  gap: 14px;
}
label {
  display: grid;
  gap: 8px;
  font-size: 13px;
}
.enabled {
  display: flex;
  align-items: center;
}
input,
select,
button {
  font: inherit;
  border: 1px solid #cbd6ce;
  border-radius: 7px;
  padding: 9px;
  background: white;
  min-width: 0;
}
button {
  cursor: pointer;
  color: #1e5a3b;
}
button:disabled {
  opacity: 0.5;
}
.path {
  background: #f4f7f5;
  padding: 10px;
  border-radius: 8px;
}
.path code {
  overflow-wrap: anywhere;
  font-size: 12px;
}
.error {
  color: #a12b2b;
}
.success {
  color: #216940;
}
.preview-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  border-bottom: 1px solid #eee;
  padding: 12px 0;
}
@media (max-width: 850px) {
  .form-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
