import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const designer = readFileSync(new URL('../views/ProductCreatorView.vue', import.meta.url), 'utf8')
const runForm = readFileSync(new URL('../views/ProductCreatorRunView.vue', import.meta.url), 'utf8')

test('V5 designer keeps BOM defaults when a route is connected and previews each product spec route', () => {
  assert.match(designer, /const selectedBOMConnectedRoute = computed\(\(\) =>/)
  assert.match(designer, /实际工艺 \{\{ routeName\(selectedBOMConnectedRoute\?\.id \|\| variant\.process_route_id\) \}\}/)
  assert.match(designer, /删除工艺连线后会恢复规格模板中的路线/)
  assert.doesNotMatch(designer, /target\.data\.config\.route_id\s*=\s*0/)
  assert.match(designer, /连线优先；未连接时按需选择/)
})

test('V5 naming preview is live and sample values stay out of saved workflow data', () => {
  assert.match(designer, /const selectedNamePreview = computed\(\(\) => renderNamePreview\(selectedNameParts\.value, workflowVariables\.value, namePreviewSamples\.value\)\)/)
  assert.match(designer, /生成名称预览/)
  assert.match(designer, /仅用于预览/)
  assert.match(designer, /function setNamePreviewSample\(variableID, value\)/)
  assert.match(designer, /workflow: toWorkflowGraph\(nodes\.value, edges\.value, templateWorkflowVersion\.value, workflowVariables\.value\)/)
  assert.doesNotMatch(designer.match(/function saveTemplate\(\)[\s\S]*?\n\}/)?.[0] || '', /namePreviewSamples/)
})

test('V5 run forms allow independent route overrides for material and specification-template BOMs', () => {
  assert.match(runForm, /workflowVersion >= 5[\s\S]*?route_override_id/)
  assert.match(runForm, /defaultBOMRouteSource\(node\).*defaultBOMRouteLabel\(node\)/)
  assert.match(runForm, /本次改选 · \{\{ route\.label \}\}/)
  assert.match(runForm, /effectiveTemplateVariantRouteID\(node, variant\)/)
  assert.match(runForm, /pc-run-bom-route-results/)
  assert.match(runForm, /effective_specification_routes/)
  assert.match(runForm, /connected_node: '使用连线工艺'/)
  assert.doesNotMatch(runForm, /跟随连线工艺/)
})
