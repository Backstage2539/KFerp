import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const designer = readFileSync(new URL('../views/ProductCreatorView.vue', import.meta.url), 'utf8')
const runtime = readFileSync(new URL('../views/ProductCreatorRunView.vue', import.meta.url), 'utf8')

test('V7 designer removes material presets while historical V6 runtime retains them', () => {
  assert.doesNotMatch(designer, /默认物料行|addDefaultMaterialRow|selectedDefaultMaterialRows/)
  assert.match(designer, /changeMaterialSupplyMode\(selectedNode.id, \$event.target.value\)/)
  assert.match(runtime, /workflowVersion.value === 6 \? node.config\?\.default_rows/)
  assert.match(runtime, /action: workflowVersion.value === 6 \? 'create' : 'reuse'/)
})

test('V6 run rows apply material name variables and estimate purchased cost without mutating reused objects', () => {
  assert.match(runtime, /generatedMaterialRowName\(node, row\)/)
  assert.match(runtime, /恢复模板命名/)
  assert.match(runtime, /仅修改本次运行/)
  assert.match(runtime, /row\.supply_mode === 'purchase'[\s\S]*暂估采购单价/)
  assert.match(runtime, /row\.action === 'reuse'[\s\S]*已选择：\{\{ selectedMaterialLabel\(row\) \}\}/)
  assert.match(runtime, /row\.classification_group_id/)
})

test('V6 classification drawer scopes active choices to the object type and route/source wording is explicit', () => {
  const picker = readFileSync(new URL('../components/BusinessGroupClassificationPicker.vue', import.meta.url), 'utf8')
  assert.match(picker, /搜索分类名称或完整路径/)
  assert.match(runtime, /BusinessGroupClassificationPicker :groups="businessGroups"[\s\S]*businessGroupSelections\[classificationDrawer.usage\]/)
  assert.match(picker, /usageKey: props.usageKey/)
  assert.match(runtime, /classification_group_id: Number\(option\.group_id\)/)
  assert.match(runtime, /使用连线工艺/)
  assert.match(runtime, /使用模板预设工艺/)
  assert.match(runtime, /使用规格模板工艺/)
  assert.match(runtime, /来源配方对象 \*/)
})
