<template>
  <div class="page capability-templates">
    <section class="panel top-panel">
      <div class="panel-head">
        <div>
          <h2>客户能力模板设置</h2>
          <p>ERP 权限、小程序入口、客户能力和计价规则</p>
        </div>
        <button class="secondary" type="button" @click="loadTemplates" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section
      v-for="editor in visibleTemplateEditors"
      :key="editor.form.key"
      class="template-panel"
      :class="{ inactive: !editor.form.active, child: !!editor.form.parent_template_key }"
      :style="{ marginLeft: `${templateDepth(editor) * 22}px` }">
      <div class="template-head">
        <div class="template-title">
          <button
            v-if="hasChildren(editor)"
            class="icon-button"
            type="button"
            @click="toggleTemplateCollapsed(editor)"
            :aria-label="collapsedTemplateKeys.has(editor.form.key) ? '展开子模板' : '折叠子模板'">
            {{ collapsedTemplateKeys.has(editor.form.key) ? '▸' : '▾' }}
          </button>
          <span v-else class="tree-spacer"></span>
          <div>
            <h3>{{ editor.form.label }}</h3>
            <span>{{ editor.form.key }}</span>
          </div>
        </div>
        <div class="template-actions">
          <label class="status-toggle">
            <input v-model="editor.form.active" type="checkbox" />
            <span>{{ editor.form.active ? '模板启用' : '模板失效' }}</span>
          </label>
          <button class="secondary" type="button" @click="copyTemplate(editor)" :disabled="loading || editor.saving">复制模板</button>
          <button class="primary" type="button" @click="saveTemplate(editor)" :disabled="loading || editor.saving">
            {{ editor.saving ? '保存中' : '保存模板' }}
          </button>
        </div>
      </div>

      <div class="template-layout">
        <div class="rule-group identity-group">
          <div class="group-title">模板定位</div>
          <label>
            <span>模板名称</span>
            <input v-model.trim="editor.form.label" />
          </label>
          <label>
            <span>父模板</span>
            <input :value="editor.form.parent_template_key || '母模板'" disabled />
          </label>
          <label class="wide">
            <span>说明</span>
            <textarea v-model.trim="editor.form.description" rows="3"></textarea>
          </label>
          <label>
            <span>小程序首页</span>
            <select v-model="editor.form.miniapp_entry_mode">
              <option value="services">订单处理</option>
              <option value="mall">商城</option>
            </select>
          </label>
          <div class="theme-picker">
            <span>小程序主题</span>
            <div class="theme-options">
              <button
                v-for="theme in customerPortalThemeOptions"
                :key="`${editor.form.key}-${theme.key}`"
                type="button"
                class="theme-option"
                :class="{ selected: editor.form.theme_key === theme.key }"
                :title="theme.description"
                @click="editor.form.theme_key = theme.key">
                <i :class="['theme-swatch', theme.swatchClass]"></i>
                <span>{{ theme.label }}</span>
              </button>
            </div>
          </div>
        </div>

        <div class="rule-group capability-group">
          <div class="group-title">客户能力开关</div>
          <div class="capability-grid">
            <label v-for="item in capabilityDefinitions" :key="`${editor.form.key}-${item.code}`" class="capability-row">
              <input v-model="capabilityOf(editor, item.code).enabled" type="checkbox" />
              <span>{{ item.label }}</span>
            </label>
          </div>
        </div>

        <div class="rule-group rules-group">
          <div class="group-title">代发和公共 SKU 规则</div>
          <label class="check-row">
            <input v-model="capabilityOf(editor, 'product_order').config.public_sku_aliases" type="checkbox" />
            <span>现货下单支持公共 SKU 别名</span>
          </label>
          <label class="check-row">
            <input v-model="capabilityOf(editor, 'direct_ship').config.public_sku_aliases" type="checkbox" />
            <span>一件代发支持公共 SKU 别名</span>
          </label>
          <label class="check-row">
            <input v-model="capabilityOf(editor, 'direct_ship').config.customer_sender" type="checkbox" />
            <span>代发默认使用客户寄件人</span>
          </label>
          <label class="check-row">
            <input v-model="capabilityOf(editor, 'direct_ship').config.external_recipients" type="checkbox" />
            <span>代发收件人不进入系统客户列表</span>
          </label>
          <div class="small-batch">
            <label class="check-row">
              <input v-model="smallBatchRule(editor).enabled" type="checkbox" />
              <span>启用小批量价格梯度</span>
            </label>
            <div class="number-grid">
              <label>
                <span>低于磅数</span>
                <input v-model.number="smallBatchRule(editor).threshold_lb" type="number" min="0" step="0.1" />
              </label>
              <label>
                <span>梯度下限</span>
                <input v-model.number="smallBatchRule(editor).tier_min_lb" type="number" min="0" step="0.1" />
              </label>
              <label>
                <span>梯度上限</span>
                <input v-model.number="smallBatchRule(editor).tier_max_lb" type="number" min="0" step="0.1" />
              </label>
            </div>
          </div>
        </div>

        <div class="rule-group mapping-group">
          <div class="group-title">ERP 权限映射</div>
          <div class="chip-block">
            <span>角色</span>
            <div class="chips">
              <i v-for="value in editor.form.erp_role_codes" :key="`${editor.form.key}-role-${value}`">{{ value }}</i>
            </div>
          </div>
          <div class="chip-block">
            <span>权限</span>
            <div class="chips">
              <i v-for="value in editor.form.erp_permissions" :key="`${editor.form.key}-perm-${value}`">{{ value }}</i>
            </div>
          </div>
          <div class="chip-block">
            <span>页面</span>
            <div class="chips">
              <i v-for="value in editor.form.erp_view_keys" :key="`${editor.form.key}-view-${value}`">{{ viewLabel(value) }}</i>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { customerPortalThemeOptions, normalizeCustomerPortalThemeKey } from '../lib/customer-portal-theme'

const capabilityDefinitions = [
  { code: 'bean_list', label: '我的豆单' },
  { code: 'mall', label: '商城下单' },
  { code: 'product_order', label: '现货下单' },
  { code: 'direct_ship', label: '一件代发' },
  { code: 'processing', label: '代加工' },
  { code: 'inventory_custody', label: '我的库存' },
  { code: 'settlement', label: '结算中心' },
]

const defaultCapabilityLabels = Object.fromEntries(capabilityDefinitions.map((item) => [item.code, item.label]))
const editors = ref([])
const collapsedTemplateKeys = ref(new Set())
const loading = ref(false)
const error = ref('')
const ok = ref('')

const editorsByKey = computed(() => new Map(editors.value.map((editor) => [editor.form.key, editor])))
const visibleTemplateEditors = computed(() => {
  const out = []
  for (const editor of editors.value) {
    if (isHiddenByCollapsedParent(editor)) continue
    out.push(editor)
  }
  return out
})

async function loadTemplates() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiGet('/api/customer-portal/admin/capability-templates')
    editors.value = (data.templates || []).map(createEditor)
  } catch (err) {
    error.value = err.message || '加载模板失败'
  } finally {
    loading.value = false
  }
}

function createEditor(template) {
  const form = normalizeTemplate(template)
  return {
    form,
    saving: false,
  }
}

function normalizeTemplate(template) {
  const capabilities = capabilityDefinitions.map((definition) => {
    const existing = (template?.capabilities || []).find((item) => item.code === definition.code) || {}
    return {
      code: definition.code,
      label: existing.label || definition.label,
      description: existing.description || '',
      enabled: !!existing.enabled,
      config: normalizeCapabilityConfig(definition.code, existing.config || {}),
    }
  })
  return {
    key: template?.key || '',
    parent_template_key: template?.parent_template_key || '',
    label: template?.label || '',
    description: template?.description || '',
    theme_key: normalizeCustomerPortalThemeKey(template?.theme_key),
    miniapp_entry_mode: template?.miniapp_entry_mode === 'mall' ? 'mall' : 'services',
    erp_role_codes: normalizedStringList(template?.erp_role_codes),
    erp_permissions: normalizedStringList(template?.erp_permissions),
    erp_view_keys: normalizedStringList(template?.erp_view_keys),
    active: template?.active !== false,
    sort_order: Number(template?.sort_order || 0),
    capabilities,
  }
}

function normalizeCapabilityConfig(code, config) {
  const source = { ...(config || {}) }
  if (code === 'product_order') {
    return { public_sku_aliases: !!source.public_sku_aliases }
  }
  if (code === 'direct_ship') {
    return {
      public_sku_aliases: !!source.public_sku_aliases,
      customer_sender: !!source.customer_sender,
      external_recipients: !!source.external_recipients,
      small_batch_price_rule: normalizeSmallBatchRule(source.small_batch_price_rule),
    }
  }
  return source
}

function normalizeSmallBatchRule(value) {
  const source = value || {}
  return {
    enabled: !!source.enabled,
    threshold_lb: positiveNumber(source.threshold_lb, 14),
    tier_min_lb: positiveNumber(source.tier_min_lb, 15),
    tier_max_lb: positiveNumber(source.tier_max_lb, 28),
  }
}

function normalizedStringList(values) {
  return [...new Set((values || []).map((value) => String(value || '').trim()).filter(Boolean))]
}

function capabilityOf(editor, code) {
  let capability = editor.form.capabilities.find((item) => item.code === code)
  if (!capability) {
    capability = {
      code,
      label: defaultCapabilityLabels[code] || code,
      description: '',
      enabled: false,
      config: normalizeCapabilityConfig(code, {}),
    }
    editor.form.capabilities.push(capability)
  }
  if (!capability.config) {
    capability.config = normalizeCapabilityConfig(code, {})
  }
  if (code === 'product_order' && typeof capability.config.public_sku_aliases !== 'boolean') {
    capability.config.public_sku_aliases = !!capability.config.public_sku_aliases
  }
  if (code === 'direct_ship') {
    if (typeof capability.config.public_sku_aliases !== 'boolean') {
      capability.config.public_sku_aliases = !!capability.config.public_sku_aliases
    }
    if (typeof capability.config.customer_sender !== 'boolean') {
      capability.config.customer_sender = !!capability.config.customer_sender
    }
    if (typeof capability.config.external_recipients !== 'boolean') {
      capability.config.external_recipients = !!capability.config.external_recipients
    }
    if (!capability.config.small_batch_price_rule) {
      capability.config.small_batch_price_rule = normalizeSmallBatchRule({})
    }
  }
  return capability
}

function smallBatchRule(editor) {
  const directShip = capabilityOf(editor, 'direct_ship')
  if (!directShip.config.small_batch_price_rule) {
    directShip.config.small_batch_price_rule = normalizeSmallBatchRule({})
  }
  return directShip.config.small_batch_price_rule
}

function positiveNumber(value, fallback) {
  const got = Number(value)
  return Number.isFinite(got) && got > 0 ? got : fallback
}

function payloadFor(editor) {
  return {
    key: editor.form.key,
    parent_template_key: editor.form.parent_template_key || '',
    label: editor.form.label,
    description: editor.form.description,
    theme_key: normalizeCustomerPortalThemeKey(editor.form.theme_key),
    miniapp_entry_mode: editor.form.miniapp_entry_mode === 'mall' ? 'mall' : 'services',
    erp_role_codes: normalizedStringList(editor.form.erp_role_codes),
    erp_permissions: normalizedStringList(editor.form.erp_permissions),
    erp_view_keys: normalizedStringList(editor.form.erp_view_keys),
    active: editor.form.active !== false,
    sort_order: Number(editor.form.sort_order || 0),
    capabilities: capabilityDefinitions.map((definition) => {
      const capability = capabilityOf(editor, definition.code)
      return {
        code: capability.code,
        enabled: !!capability.enabled,
        config: normalizeCapabilityConfig(capability.code, capability.config || {}),
      }
    }),
  }
}

function hasChildren(editor) {
  return editors.value.some((item) => item.form.parent_template_key === editor.form.key)
}

function toggleTemplateCollapsed(editor) {
  const next = new Set(collapsedTemplateKeys.value)
  if (next.has(editor.form.key)) next.delete(editor.form.key)
  else next.add(editor.form.key)
  collapsedTemplateKeys.value = next
}

function isHiddenByCollapsedParent(editor) {
  let parentKey = editor.form.parent_template_key
  const seen = new Set()
  while (parentKey && !seen.has(parentKey)) {
    if (collapsedTemplateKeys.value.has(parentKey)) return true
    seen.add(parentKey)
    parentKey = editorsByKey.value.get(parentKey)?.form?.parent_template_key || ''
  }
  return false
}

function templateDepth(editor) {
  let depth = 0
  let parentKey = editor.form.parent_template_key
  const seen = new Set()
  while (parentKey && !seen.has(parentKey)) {
    depth += 1
    seen.add(parentKey)
    parentKey = editorsByKey.value.get(parentKey)?.form?.parent_template_key || ''
  }
  return depth
}

async function copyTemplate(editor) {
  if (!editor?.form?.key) return
  const newKey = window.prompt('请输入新模板 key，只能使用小写字母、数字和下划线', `${editor.form.key}_copy`)
  if (!newKey) return
  const label = window.prompt('请输入新模板名称', `${editor.form.label} 副本`)
  if (!label) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend(`/api/customer-portal/admin/capability-templates/${editor.form.key}/copy`, {
      method: 'POST',
      body: { new_key: newKey, label },
    })
    await loadTemplates()
    ok.value = `已复制模板 ${data.label || label}`
  } catch (err) {
    error.value = err.message || '复制模板失败'
  } finally {
    loading.value = false
  }
}

async function saveTemplate(editor) {
  if (!editor?.form?.key) return
  editor.saving = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend(`/api/customer-portal/admin/capability-templates/${editor.form.key}`, {
      method: 'PUT',
      body: payloadFor(editor),
    })
    const index = editors.value.findIndex((item) => item.form.key === editor.form.key)
    if (index >= 0) {
      editors.value[index] = createEditor(data)
    }
    ok.value = `已保存 ${data.label || editor.form.label}`
  } catch (err) {
    error.value = err.message || '保存模板失败'
  } finally {
    editor.saving = false
  }
}

function viewLabel(key) {
  if (key === 'customerProcessingPortal') return '客户履约工作台（客户侧）'
  return key
}

onMounted(loadTemplates)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel, .template-panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.template-panel.child { border-left: 4px solid #d7e2ea; }
.template-panel.inactive { background: #f8fafc; opacity: .78; }
.panel-head, .template-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.template-title, .template-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
h2, h3, p { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 18px; }
p, .template-head span { color: #666; font-size: 13px; margin-top: 5px; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.icon-button { width: 30px; min-height: 30px; padding: 0; border-color: #d5dce3; background: #f8fafc; }
.tree-spacer { display: inline-block; width: 30px; height: 30px; }
.status-toggle { display: inline-flex; align-items: center; gap: 6px; min-height: 38px; border: 1px solid #e4e7ec; border-radius: 6px; padding: 0 10px; }
.status-toggle input { width: auto; height: auto; }
.status-toggle span { margin: 0; color: #333; }
.template-layout { display: grid; grid-template-columns: minmax(240px, .9fr) minmax(260px, 1fr) minmax(300px, 1.1fr) minmax(260px, 1fr); gap: 18px; margin-top: 14px; align-items: start; }
.rule-group { min-width: 0; }
.group-title { color: #333; font-weight: 700; font-size: 14px; padding-bottom: 8px; border-bottom: 1px solid #eef1f4; margin-bottom: 10px; }
label span, .theme-picker > span, .chip-block > span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 8px 9px; font: inherit; background: #fff; }
input, select { height: 38px; }
textarea { resize: vertical; min-height: 78px; }
.identity-group { display: grid; gap: 10px; }
.theme-options { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
.theme-option { min-height: 44px; display: grid; grid-template-columns: 14px minmax(0, 1fr); column-gap: 6px; align-items: center; width: 100%; height: auto; padding: 5px 6px; border: 1px solid #e4e7ec; border-radius: 6px; background: #fff; color: #171717; text-align: left; }
.theme-option.selected { border-color: #1f1f1f; box-shadow: 0 0 0 2px rgba(31,31,31,.08); }
.theme-option span { min-width: 0; color: #333; font-size: 12px; line-height: 1.2; overflow-wrap: anywhere; margin: 0; }
.theme-swatch { width: 14px; height: 14px; border-radius: 999px; }
.theme-swatch-coffee { background: linear-gradient(135deg, #2b2118, #9b7141); }
.theme-swatch-clean { background: linear-gradient(135deg, #e7f0eb, #28624a); }
.theme-swatch-premium { background: linear-gradient(135deg, #111, #b88a46); }
.capability-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.capability-row, .check-row { min-height: 38px; display: flex; align-items: center; gap: 8px; border: 1px solid #e4e7ec; border-radius: 6px; padding: 7px 9px; }
.capability-row input, .check-row input { width: auto; height: auto; }
.capability-row span, .check-row span { margin: 0; color: #333; font-size: 13px; line-height: 1.25; }
.rules-group { display: grid; gap: 8px; }
.small-batch { display: grid; gap: 8px; padding-top: 4px; }
.number-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
.chip-block { margin-bottom: 12px; }
.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.chips i { font-style: normal; border: 1px solid #dde3ea; border-radius: 999px; background: #f8fafc; padding: 4px 8px; color: #333; font-size: 12px; line-height: 1.3; max-width: 100%; overflow-wrap: anywhere; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 1200px) {
  .template-layout { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 720px) {
  .template-layout, .capability-grid, .number-grid, .theme-options { grid-template-columns: 1fr; }
}
</style>
