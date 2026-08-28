<template>
  <div>
    <div v-if="legacyBlocked" class="error pricing-rule-migration-alert" role="alert">
      <strong>旧价格方式无法安全换算；请新建加价率模板。</strong>
      <span>该模板仅供核对，不能直接保存。原方式：{{ legacyMethodLabel }}；原参数：{{ legacyValueLabel }}。</span>
    </div>
    <form class="template-editor pricing-rule-form" @submit.prevent="$emit('save')">
      <div class="template-editor-grid">
        <label><span>模板名称</span><input v-model.trim="form.name" placeholder="如 成本加成含税" /></label>
        <label><span>模板编号</span><input v-model.trim="form.code" placeholder="如 PR-COST-PLUS" /></label>
        <label>
          <span>基础成本</span>
          <select v-model="form.cost_source_mode"><option value="bom_current_cost">生产 BOM 成本（物料+工序）</option></select>
        </label>
        <label><span>公式版本</span><input v-model.trim="form.formula_version" placeholder="v1" /></label>
      </div>
      <div class="pricing-rule-form-section">
        <div class="pricing-rule-section-head">
          <strong>其他成本</strong>
          <button class="secondary compact-action" type="button" @click="addOtherCost">新增其他成本</button>
        </div>
        <small class="muted">生产 BOM 成本已包含物料采购成本和已选择工序成本；货币使用全局币种配置，当前不在价格模板中单独设置。</small>
        <div class="pricing-rule-other-cost-list">
          <div v-for="(row, index) in form.other_cost_rows" :key="index" class="pricing-rule-other-cost-row">
            <label><span>成本名</span><input v-model.trim="row.key" placeholder="如 包装贴标" /></label>
            <label><span>成本价格</span><input v-model.number="row.value" type="number" min="0" step="0.0001" placeholder="0" /></label>
            <button class="secondary compact-action" type="button" @click="removeOtherCost(index)">删除</button>
          </div>
        </div>
      </div>
      <div class="template-editor-grid">
        <label>
          <span>加价率（80%=0.8）</span>
          <input v-model.number="form.margin_rate" type="number" min="0" step="0.0001" placeholder="如 0.8" />
          <small>计算公式：税前价 = 成本基数 × (1 + 加价率)；最终售价再计算税额和取整</small>
        </label>
        <label>
          <span>最低毛利率（仅预警）</span>
          <input v-model.number="form.minimum_margin_rate" type="number" min="0" step="0.0001" placeholder="0.18" />
          <small>只比较试算结果，不参与售价计算</small>
        </label>
        <label>
          <span>税费方式</span>
          <select v-model="form.tax_mode"><option value="tax_included">含税</option><option value="tax_excluded">未税</option><option value="none">不计税</option></select>
        </label>
        <label><span>税率</span><input v-model.number="form.tax_rate" type="number" min="0" step="0.0001" placeholder="0.06" /></label>
        <label>
          <span>取整规则</span>
          <select v-model="form.rounding_mode"><option value="none">不取整</option><option value="jiao">保留到角</option><option value="yuan">保留到元</option></select>
        </label>
      </div>
      <label class="wide-field"><span>备注</span><textarea v-model.trim="form.remark" rows="2" placeholder="说明 BOM/耗材/工艺成本如何参与试算"></textarea></label>
      <label class="wide-field"><span>试算说明</span><textarea v-model.trim="form.trial_note" rows="2" placeholder="例如：选择商品、销售单位后按生产 BOM 成本试算"></textarea></label>
      <div v-if="error" class="error" role="alert">{{ error }}</div>
      <div v-if="message" class="ok" role="status">{{ message }}</div>
      <div class="form-actions">
        <button class="primary" type="submit" :disabled="saving || legacyBlocked">{{ saving ? '保存中…' : '保存价格计算模板' }}</button>
        <button v-if="allowDeactivate && form.id && form.active !== false" class="secondary danger-outline" type="button" :disabled="saving" @click="$emit('deactivate')">失效</button>
      </div>
    </form>
  </div>
</template>

<script setup>
const props = defineProps({
  form: { type: Object, required: true },
  saving: { type: Boolean, default: false },
  error: { type: String, default: '' },
  message: { type: String, default: '' },
  legacyBlocked: { type: Boolean, default: false },
  legacyMethodLabel: { type: String, default: '-' },
  legacyValueLabel: { type: String, default: '-' },
  allowDeactivate: { type: Boolean, default: false },
})

defineEmits(['save', 'deactivate'])

function addOtherCost() {
  if (!Array.isArray(props.form.other_cost_rows)) props.form.other_cost_rows = []
  props.form.other_cost_rows.push({ key: '', value: 0 })
}

function removeOtherCost(index) {
  if (!Array.isArray(props.form.other_cost_rows)) return
  props.form.other_cost_rows.splice(index, 1)
  if (!props.form.other_cost_rows.length) addOtherCost()
}
</script>

<style scoped>
.template-editor, .pricing-rule-form { display: grid; gap: 12px; }
.template-editor-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
label { display: grid; gap: 5px; min-width: 0; }
label > span { font-weight: 600; }
input, select, textarea { box-sizing: border-box; width: 100%; border: 1px solid #d8d0c5; border-radius: 7px; padding: 9px 10px; font: inherit; background: #fff; }
.pricing-rule-form-section { display: grid; gap: 9px; border: 1px solid #eee8df; border-radius: 8px; padding: 12px; background: #fbfaf8; }
.pricing-rule-section-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.pricing-rule-other-cost-list { display: grid; gap: 8px; }
.pricing-rule-other-cost-row { display: grid; grid-template-columns: minmax(0, 1fr) minmax(140px, .45fr) auto; gap: 8px; align-items: end; }
.wide-field { grid-column: 1 / -1; }
.form-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.primary, .secondary { border: 1px solid #282522; border-radius: 7px; padding: 8px 12px; cursor: pointer; font: inherit; }
.primary { background: #282522; color: #fff; }
.secondary { background: #fff; color: #282522; }
.compact-action { padding: 6px 9px; }
.danger-outline { border-color: #b04848; color: #a43636; }
.muted, small { color: #716b63; }
.error, .ok { border-radius: 7px; padding: 9px 10px; }
.error { color: #a12e2e; background: #fff0f0; }
.ok { color: #276b3a; background: #eef9f1; }
.pricing-rule-migration-alert { display: grid; gap: 4px; margin-bottom: 10px; }
button:disabled { cursor: not-allowed; opacity: .55; }
@media (max-width: 720px) {
  .template-editor-grid, .pricing-rule-other-cost-row { grid-template-columns: 1fr; }
}
</style>
