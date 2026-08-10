<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import {
  fetchResaleBeanListEditor,
  fetchResaleBeanLists,
  publishResaleBeanList,
  saveResaleBeanListDraft,
  type BeanListProductSummary,
  type BeanListSummary,
  type ResaleBeanListCategoryDraft,
  type ResaleBeanListCommand,
  type ResaleBeanListEditor,
  type ResaleBeanListItemOverride,
  type ResaleBeanListPage,
  type ResaleGradientTemplate,
} from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { useSessionStore } from '../../stores/session'
import {
  buildResaleBeanListPublishPayload,
  defaultResaleBeanListDraft,
  resaleBeanListItemKey,
  resaleCardsPerRowOptions,
  resaleStyleColorPresets,
} from '../../utils/resaleBeanList'
import { miniappThemeClass } from '../../utils/themes'

type SourceOption = {
  id: number
  type: 'factory_supply' | 'customer_resale'
  label: string
  row: BeanListSummary
}

type DraftCategoryView = ResaleBeanListCategoryDraft & {
  items: BeanListProductSummary[]
}

type MiniInputEvent = {
  detail?: {
    value?: string
  }
  target?: {
    value?: string
  }
}

const session = useSessionStore()
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const resalePage = ref<ResaleBeanListPage | null>(null)
const editor = ref<ResaleBeanListEditor | null>(null)
const resaleDraft = ref<ResaleBeanListCommand | null>(null)
const selectedSourceID = ref(0)
const editingItemKey = ref('')
const editingHighlightTerms = ref('')

const themeClass = computed(() => miniappThemeClass(session.themeKey))
const gradientTemplates = computed<ResaleGradientTemplate[]>(() => editor.value?.gradient_templates || resalePage.value?.gradient_templates || [])

const sourceOptions = computed<SourceOption[]>(() => {
  const page = resalePage.value
  if (!page) return []
  const factory = (page.factory_supply_bean_lists || []).map((row) => ({
    id: row.id,
    type: 'factory_supply' as const,
    label: `工厂价格表 - ${row.list_type_label || row.title || row.list_type} ${row.version_no || ''}`.trim(),
    row,
  }))
  const customer = (page.customer_resale_bean_lists || []).map((row) => ({
    id: row.id,
    type: 'customer_resale' as const,
    label: `我的已发布价格表 - ${row.list_type_label || row.title || row.list_type} ${row.version_no || ''}`.trim(),
    row,
  }))
  return [...factory, ...customer]
})

const sourceLabels = computed(() => sourceOptions.value.map((item) => item.label))
const sourcePickerValue = computed(() => Math.max(0, sourceOptions.value.findIndex((item) => item.id === selectedSourceID.value)))
const selectedSourceLabel = computed(() => sourceOptions.value[sourcePickerValue.value]?.label || '请选择复制来源')

const templateLabels = computed(() =>
  gradientTemplates.value.length
    ? gradientTemplates.value.map((row) => `${row.name}${row.display_unit ? ` / ${row.display_unit}` : ''}`)
    : ['不使用阶梯价模板'],
)
const templatePickerValue = computed(() => {
  const templateID = resaleDraft.value?.gradient_template_id || 0
  const index = gradientTemplates.value.findIndex((row) => row.id === templateID)
  return Math.max(0, index)
})
const selectedTemplateLabel = computed(() => {
  const templateID = resaleDraft.value?.gradient_template_id || 0
  const row = gradientTemplates.value.find((item) => item.id === templateID)
  return row ? `${row.name}${row.display_unit ? ` / ${row.display_unit}` : ''}` : '不使用阶梯价模板'
})

const resaleDraftItems = computed<BeanListProductSummary[]>(() => {
  const source = editor.value?.source
  if (!source) return []
  const rows: BeanListProductSummary[] = []
  for (const group of source.groups || []) {
    for (const item of group.items || []) rows.push(item)
  }
  return rows
})

const resaleSelectedAll = computed(() => {
  if (!resaleDraft.value || !resaleDraftItems.value.length) return false
  const selected = new Set(resaleDraft.value.selected_item_codes || [])
  return resaleDraftItems.value.every((item) => selected.has(resaleBeanListItemKey(item)))
})

const resaleSelectedCount = computed(() => resaleDraft.value?.selected_item_codes?.length || 0)
const draftCategoryViews = computed<DraftCategoryView[]>(() => {
  const draft = resaleDraft.value
  if (!draft) return []
  const itemMap = new Map<string, BeanListProductSummary>()
  for (const item of resaleDraftItems.value) {
    const key = resaleBeanListItemKey(item)
    if (key) itemMap.set(key, item)
  }
  const views: DraftCategoryView[] = []
  for (const category of draft.category_drafts || []) {
    if (category.deleted) continue
    const view = category as DraftCategoryView
    view.items = (category.item_codes || []).map((key) => itemMap.get(key)).filter(Boolean) as BeanListProductSummary[]
    views.push(view)
  }
  return views
})

const editingItem = computed(() => resaleDraftItems.value.find((item) => resaleBeanListItemKey(item) === editingItemKey.value))

async function loadWorkspace() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const page = await fetchResaleBeanLists(session.token)
    resalePage.value = page
    if (!editor.value && sourceOptions.value.length) {
      await openResaleEditor(sourceOptions.value[0].id)
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '价格表设置加载失败'
  } finally {
    loading.value = false
  }
}

async function openResaleEditor(sourceID: number) {
  if (!session.token || !sourceID) return
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await fetchResaleBeanListEditor(session.token, sourceID)
    editor.value = result
    selectedSourceID.value = result.source.id
    resaleDraft.value = defaultResaleBeanListDraft(result.source, result.next_version_no)
    if (result.gradient_templates?.length) {
      resaleDraft.value.gradient_template_id = result.gradient_templates[0]?.id || 0
    }
    editingItemKey.value = ''
    editingHighlightTerms.value = ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '复制来源加载失败'
  } finally {
    loading.value = false
  }
}

function setSource(event: { detail?: { value?: number | string } }) {
  const index = Number(event.detail?.value ?? 0)
  const option = sourceOptions.value[index]
  if (option) void openResaleEditor(option.id)
}

function setTemplate(event: { detail?: { value?: number | string } }) {
  if (!resaleDraft.value) return
  const index = Number(event.detail?.value ?? 0)
  resaleDraft.value.gradient_template_id = gradientTemplates.value[index]?.id || 0
}

function miniInputValue(event: unknown): string {
  const input = event as MiniInputEvent
  return String(input.detail?.value ?? input.target?.value ?? '').trim()
}

function setConfigText(key: string, event: unknown) {
  if (!resaleDraft.value) return
  resaleDraft.value.config[key] = miniInputValue(event)
}

function configText(key: string): string {
  return String(resaleDraft.value?.config?.[key] || '')
}

function setConfigValue(key: string, value: unknown) {
  if (!resaleDraft.value) return
  resaleDraft.value.config[key] = value
}

function layoutStyle(): string {
  return String(resaleDraft.value?.config?.layoutStyle || 'card')
}

function cardsPerRow(): number {
  return Number(resaleDraft.value?.config?.cardsPerRow || 2)
}

function stylePresetActive(preset: { backgroundColor: string; fontColor: string }): boolean {
  return configText('backgroundColor') === preset.backgroundColor && configText('fontColor') === preset.fontColor
}

function setStylePreset(preset: { backgroundColor: string; fontColor: string }) {
  setConfigValue('backgroundColor', preset.backgroundColor)
  setConfigValue('fontColor', preset.fontColor)
}

function addCategoryDraft() {
  if (!resaleDraft.value) return
  const rows = resaleDraft.value.category_drafts || []
  resaleDraft.value.category_drafts = [
    ...rows,
    {
      id: `category-${Date.now()}`,
      name: '新分类',
      item_codes: [],
      collapsed: false,
      sort_order: (rows.length + 1) * 100,
    },
  ]
}

function setCategoryName(category: ResaleBeanListCategoryDraft, event: unknown) {
  category.name = miniInputValue(event)
}

function removeCategory(category: ResaleBeanListCategoryDraft) {
  uni.showModal({
    title: '删除分类',
    content: `确认删除 ${category.name || '未命名分类'}？商品会保留，可重新选择。`,
    success: (res) => {
      if (!res.confirm || !resaleDraft.value) return
      category.deleted = true
      resaleDraft.value.category_drafts = [...(resaleDraft.value.category_drafts || [])]
    },
  })
}

function toggleCategory(category: ResaleBeanListCategoryDraft) {
  category.collapsed = !category.collapsed
  if (resaleDraft.value) resaleDraft.value.category_drafts = [...(resaleDraft.value.category_drafts || [])]
}

function resaleItemSelected(bean: BeanListProductSummary): boolean {
  const key = resaleBeanListItemKey(bean)
  return Boolean(key && resaleDraft.value?.selected_item_codes?.includes(key))
}

function toggleResaleItem(bean: BeanListProductSummary) {
  if (!resaleDraft.value) return
  const key = resaleBeanListItemKey(bean)
  if (!key) return
  const selected = new Set(resaleDraft.value.selected_item_codes || [])
  if (selected.has(key)) {
    selected.delete(key)
  } else {
    selected.add(key)
  }
  resaleDraft.value.selected_item_codes = Array.from(selected)
}

function setAllResaleItems(selected: boolean) {
  if (!resaleDraft.value) return
  resaleDraft.value.selected_item_codes = selected
    ? resaleDraftItems.value.map((item) => resaleBeanListItemKey(item)).filter(Boolean)
    : []
}

function itemOverride(bean: BeanListProductSummary): ResaleBeanListItemOverride {
  if (!resaleDraft.value) return { code: resaleBeanListItemKey(bean) }
  const key = resaleBeanListItemKey(bean)
  const rows = resaleDraft.value.item_overrides || []
  let row = rows.find((item) => item.code === key)
  if (!row) {
    row = { code: key }
    resaleDraft.value.item_overrides = [...rows, row]
  }
  return row
}

function setItemBadge(bean: BeanListProductSummary, badge: string) {
  const row = itemOverride(bean)
  row.badge_label = badge || undefined
  row.clear_badge = !badge
  if (resaleDraft.value) resaleDraft.value.item_overrides = [...(resaleDraft.value.item_overrides || [])]
}

function openItemConfig(bean: BeanListProductSummary) {
  const key = resaleBeanListItemKey(bean)
  editingItemKey.value = key
  const row = itemOverride(bean)
  editingHighlightTerms.value = (row.highlight_terms?.length ? row.highlight_terms : bean.highlight_terms || []).join(' ')
}

function saveItemHighlightTerms() {
  const bean = editingItem.value
  if (!bean) return
  const row = itemOverride(bean)
  const terms = editingHighlightTerms.value
    .split(/[,\s，、]+/)
    .map((item) => item.trim())
    .filter(Boolean)
  row.highlight_terms = terms
  row.clear_highlight_terms = terms.length === 0
  if (resaleDraft.value) resaleDraft.value.item_overrides = [...(resaleDraft.value.item_overrides || [])]
  uni.showToast({ title: '商品配置已保存', icon: 'success' })
}

async function submitResale(status: 'draft' | 'published') {
  if (!session.token || !resaleDraft.value || submitting.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    const payload = buildResaleBeanListPublishPayload(resaleDraft.value)
    if (status === 'published') {
      await publishResaleBeanList(session.token, payload)
      uni.showToast({ title: '已发布', icon: 'success' })
      uni.redirectTo({ url: '/pages/customer-products/customer-products' })
    } else {
      await saveResaleBeanListDraft(session.token, payload)
      uni.showToast({ title: '草稿已保存', icon: 'success' })
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '保存失败'
  } finally {
    submitting.value = false
  }
}

onShow(() => {
  void loadWorkspace()
})
</script>

<template>
  <view class="page pull-up-brand-page pull-up-brand-page-with-tabbar" :class="themeClass">
    <EnvironmentBadge />
    <view class="header">
      <text class="eyebrow">我的价格表设置</text>
      <text class="title">价格表设置</text>
      <text class="subtitle">复制来源、选择商品、设置加价和展示样式</text>
    </view>

    <view v-if="loading && !resaleDraft" class="state">加载中...</view>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>

    <view v-if="resaleDraft" class="panel-list">
      <view class="panel">
        <text class="panel-title">复制来源</text>
        <view class="source-hints">
          <text>工厂价格表</text>
          <text>我的已发布价格表</text>
        </view>
        <picker mode="selector" :range="sourceLabels" :value="sourcePickerValue" @change="setSource">
          <view class="picker-field">{{ selectedSourceLabel }}</view>
        </picker>
        <picker mode="selector" :range="templateLabels" :value="templatePickerValue" @change="setTemplate">
          <view class="picker-field">{{ selectedTemplateLabel }}</view>
        </picker>
      </view>

      <view class="panel">
        <text class="panel-title">基础信息</text>
        <view class="form-grid">
          <input v-model="resaleDraft.version_no" class="input" placeholder="版本号，例如 V1" />
          <input class="input" :value="configText('brandName')" placeholder="品牌名" @input="setConfigText('brandName', $event)" />
          <textarea class="textarea full" :value="configText('brandIntro')" placeholder="价格表说明/品牌介绍" @input="setConfigText('brandIntro', $event)" />
          <textarea v-model="resaleDraft.changelog" class="textarea full" placeholder="版本说明" />
          <input v-model.number="resaleDraft.price_rule.add_amount" class="input" type="number" placeholder="统一加价" />
          <input v-model.number="resaleDraft.price_rule.multiplier" class="input" type="number" placeholder="倍率加价，例如 1.1" />
        </view>
      </view>

      <view class="panel">
        <text class="panel-title">样式</text>
        <view class="color-presets">
          <button v-for="preset in resaleStyleColorPresets" :key="preset.key" :class="['color-chip', { active: stylePresetActive(preset) }]" @tap="setStylePreset(preset)">
            <text class="swatch" :style="{ backgroundColor: preset.backgroundColor, color: preset.fontColor }">A</text>
            <text>{{ preset.label }}</text>
          </button>
        </view>
        <input class="input full" :value="configText('backgroundImage')" placeholder="背景图 URL，可选" @input="setConfigText('backgroundImage', $event)" />
        <input class="input full" :value="configText('logoImage')" placeholder="Logo URL，可选" @input="setConfigText('logoImage', $event)" />
        <view class="segmented">
          <button :class="['chip', { active: layoutStyle() !== 'table' }]" @tap="setConfigValue('layoutStyle', 'card')">卡片</button>
          <button :class="['chip', { active: layoutStyle() === 'table' }]" @tap="setConfigValue('layoutStyle', 'table')">表格</button>
        </view>
        <view class="segmented">
          <button v-for="count in resaleCardsPerRowOptions" :key="`cards-${count}`" :class="['chip', { active: cardsPerRow() === count }]" @tap="setConfigValue('cardsPerRow', count)">{{ count }} 列</button>
        </view>
      </view>

      <view class="panel">
        <view class="panel-head">
          <text class="panel-title">分类</text>
          <button class="small-button" @tap="addCategoryDraft">新增分类</button>
        </view>
        <view v-for="category in draftCategoryViews" :key="category.id || category.name" class="category-row">
          <view class="category-title-row">
            <input class="category-input" :value="category.name" placeholder="分类名称" @input="setCategoryName(category, $event)" />
            <button class="small-button" @tap="toggleCategory(category)">{{ category.collapsed ? '展开' : '收起' }}</button>
            <button class="small-button danger" @tap="removeCategory(category)">删除</button>
          </view>
          <text class="row-sub">{{ category.items.length }} 个商品</text>
        </view>
      </view>

      <view class="panel">
        <view class="panel-head">
          <text class="panel-title">选择商品</text>
          <button class="small-button" @tap="setAllResaleItems(!resaleSelectedAll)">{{ resaleSelectedAll ? '取消全选' : '全选' }}</button>
        </view>
        <text class="row-sub">已选 {{ resaleSelectedCount }} / {{ resaleDraftItems.length }}</text>
        <view v-for="category in draftCategoryViews" :key="`items-${category.id || category.name}`" class="item-category">
          <view class="item-category-head" @tap="toggleCategory(category)">
            <text class="category-name">{{ category.name || '未分类' }}</text>
            <text class="row-sub">{{ category.collapsed ? '展开' : '收起' }}</text>
          </view>
          <view v-if="!category.collapsed" class="item-list">
            <view v-for="bean in category.items" :key="resaleBeanListItemKey(bean)" class="item-row">
              <label class="item-check" @tap="toggleResaleItem(bean)">
                <checkbox :checked="resaleItemSelected(bean)" color="#171717" />
                <view class="item-main">
                  <text class="item-name">{{ bean.name }}</text>
                  <text class="row-sub">{{ bean.code || '无编号' }} / {{ bean.prices?.[0]?.label || '原档位' }} {{ bean.prices?.[0]?.value || '' }}</text>
                </view>
              </label>
              <button class="icon-button" @tap.stop="openItemConfig(bean)">商品配置</button>
            </view>
          </view>
        </view>
      </view>

      <view v-if="editingItem" class="panel">
        <text class="panel-title">商品配置</text>
        <text class="row-sub">{{ editingItem.name }}</text>
        <view class="segmented">
          <button class="chip" @tap="setItemBadge(editingItem, '上新')">上新</button>
          <button class="chip" @tap="setItemBadge(editingItem, '推荐')">推荐</button>
          <button class="chip" @tap="setItemBadge(editingItem, '')">清除</button>
        </view>
        <input v-model="editingHighlightTerms" class="input full" placeholder="标红词，用空格或逗号分隔" />
        <button class="small-button wide" @tap="saveItemHighlightTerms">保存商品配置</button>
      </view>

      <view class="actions">
        <button class="secondary-button" :disabled="submitting" @tap="submitResale('draft')">保存草稿</button>
        <button class="primary-button" :disabled="submitting" @tap="submitResale('published')">发布商品价格表</button>
      </view>
    </view>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter with-fixed-tabbar />
    </view>
    <MainTabBar current="mine" />
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 28rpx 28rpx 160rpx;
  background: #f5f7f6;
  box-sizing: border-box;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  padding: 30rpx 28rpx;
  border-radius: 10rpx;
  background: #173b2e;
}

.eyebrow,
.subtitle {
  color: rgba(255, 255, 255, 0.76);
  font-size: 24rpx;
}

.title {
  color: #ffffff;
  font-size: 40rpx;
  font-weight: 900;
}

.panel-list,
.panel,
.form-grid,
.color-presets,
.item-list,
.actions {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
}

.panel-list {
  margin-top: 22rpx;
}

.panel {
  padding: 22rpx;
  border: 1rpx solid #dce7e1;
  border-radius: 10rpx;
  background: #ffffff;
}

.panel-head,
.category-title-row,
.item-category-head,
.item-row,
.source-hints,
.segmented {
  display: flex;
  align-items: center;
  gap: 14rpx;
}

.panel-head,
.category-title-row,
.item-category-head,
.item-row,
.source-hints {
  justify-content: space-between;
}

.panel-title {
  color: #14201a;
  font-size: 30rpx;
  font-weight: 900;
}

.source-hints,
.row-sub {
  color: #66756c;
  font-size: 24rpx;
}

.picker-field,
.input,
.textarea,
.category-input {
  width: 100%;
  min-height: 76rpx;
  padding: 0 22rpx;
  border: 1rpx solid #dce7e1;
  border-radius: 8rpx;
  background: #fbfdfc;
  color: #14201a;
  font-size: 26rpx;
  box-sizing: border-box;
}

.textarea {
  min-height: 140rpx;
  padding-top: 18rpx;
  line-height: 1.5;
}

.full {
  width: 100%;
}

.color-chip,
.chip,
.small-button,
.icon-button,
.primary-button,
.secondary-button {
  border-radius: 8rpx;
  font-size: 24rpx;
  font-weight: 900;
}

.color-chip,
.chip,
.small-button,
.icon-button {
  min-height: 56rpx;
  border: 1rpx solid #bdd3c8;
  background: #eef6f2;
  color: #173b2e;
}

.color-chip {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10rpx;
}

.swatch {
  width: 36rpx;
  height: 36rpx;
  border-radius: 50%;
  font-size: 22rpx;
  font-weight: 900;
  line-height: 36rpx;
  text-align: center;
}

.active {
  border-color: #173b2e;
  background: #dcebe4;
}

.segmented {
  flex-wrap: wrap;
}

.chip {
  flex: 1;
  min-width: 150rpx;
}

.category-row,
.item-category {
  padding: 18rpx 0;
  border-top: 1rpx solid #edf2ef;
}

.category-input {
  flex: 1;
}

.small-button {
  min-width: 112rpx;
  height: 56rpx;
  line-height: 56rpx;
}

.small-button.danger {
  border-color: #f1b8af;
  background: #fff4f2;
  color: #b42318;
}

.small-button.wide {
  width: 100%;
}

.category-name,
.item-name {
  color: #14201a;
  font-size: 27rpx;
  font-weight: 900;
}

.item-check {
  min-width: 0;
  display: flex;
  flex: 1;
  align-items: center;
  gap: 12rpx;
}

.item-main {
  min-width: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8rpx;
}

.icon-button {
  min-width: 132rpx;
  height: 56rpx;
  line-height: 56rpx;
}

.actions {
  padding-bottom: 20rpx;
}

.primary-button,
.secondary-button {
  width: 100%;
  height: 78rpx;
  line-height: 78rpx;
}

.primary-button {
  background: #171717;
  color: #ffffff;
}

.secondary-button {
  border: 1rpx solid #bdd3c8;
  background: #eef6f2;
  color: #173b2e;
}

.state,
.error {
  display: block;
  margin-top: 22rpx;
  padding: 22rpx;
  border-radius: 10rpx;
  background: #ffffff;
  color: #66756c;
  font-size: 26rpx;
}

.error {
  color: #b42318;
}
</style>
