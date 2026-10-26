import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const file = new URL('../components/ProductionPlanDetailWorkspace.vue', import.meta.url)

async function workspaceComponent() {
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
  const compiled = compileScript(descriptor, { id: 'production-plan-detail-groups', inlineTemplate: true })
  const bindings = {}
  const modules = new Map()
  const templateVueImports = [...compiled.content.matchAll(/import\s+\{([^}]+)\}\s+from\s+"vue"/g)]
  for (const [, imports] of templateVueImports) {
    for (const part of imports.split(',')) {
      const [imported, local] = part.trim().split(/\s+as\s+/)
      bindings[local || imported] = Vue[imported]
    }
  }
  for (const binding of Object.values(compiled.imports)) {
    if (!modules.has(binding.source)) {
      if (binding.source === 'vue') modules.set(binding.source, Vue)
      else if (binding.source === './ProductionPreparation.vue') {
        modules.set(binding.source, { default: {
          props: ['sources'],
          render() { return h('div', { class: 'test-preparation' }, (this.sources || []).map((source) => h('span', { 'data-source-id': source.id }, source.component_name))) },
        } })
      } else if (binding.source === '@tabler/icons-vue') modules.set(binding.source, { IconChevronDown: { render: () => null } })
      else modules.set(binding.source, await import(binding.source.startsWith('.')
        ? new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, file)
        : binding.source))
    }
    bindings[binding.local] = modules.get(binding.source)[binding.imported]
  }
  const code = compiled.content
    .replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\s*$/gm, '')
    .replace('export default', 'return')
  return new Function(...Object.keys(bindings), code)(...Object.values(bindings))
}

const item = (id, order) => ({
  id, output_type: 'product', output_product_id: 10, output_name: '金色山脉',
  bom_version_id: 402, bom_spec_id: 402, spec_g: 454, sales_spec_count: 10,
  inventory_unit: 'kg', inventory_qty_per_sales_unit: 0.454,
  sales_spec_snapshot_json: JSON.stringify({ spec_label: '454g', sales_unit: '袋', inventory_unit: 'kg', inventory_qty_per_sales_unit: 0.454 }),
  material_snapshot: '[{"material_id":41,"unit":"条"}]',
  process_snapshot_json: '{"operations":[{"operation":"包装"}]}',
  target_warehouse: 'finished_goods', order_nos: order,
})

test('renders grouped preparation directly under its stage and leaves multi-order task details collapsed', async () => {
  const Component = await workspaceComponent()
  const detail = {
    id: 17, plan_no: 'PP-0000000017', status: 'draft', created_at: '2026-10-25', created_by: '测试',
    picking_version: 1,
    items: [
      item(1, 'SO-A'), item(2, 'SO-B'),
      { id: 3, output_type: 'material', output_material_id: 70, output_name: '卡蒂姆5T中深烘', output_qty: 10, output_unit: 'kg', planned_output_g: 10000, process_snapshot_json: '{"operations":[{"operation":"烘焙"}]}' },
    ],
    component_sources: [
      { id: 11, production_plan_item_id: 1, picking_version: 1, component_type: 'material', component_id: 41, component_name: '咖啡豆袋', unit: '条', demand_units: 10, wip_covered_units: 0, transfer_units: 10, shortage_units: 0 },
      { id: 12, production_plan_item_id: 2, picking_version: 1, component_type: 'material', component_id: 41, component_name: '咖啡豆袋', unit: '条', demand_units: 20, wip_covered_units: 0, transfer_units: 20, shortage_units: 0 },
      { id: 13, production_plan_item_id: 1, picking_version: 1, component_type: 'material', component_id: 70, component_name: '卡蒂姆5T中深烘', unit: 'kg', demand_g: 9000, upstream_g: 9000, wip_covered_g: 0, transfer_g: 0, shortage_g: 0 },
      { id: 14, production_plan_item_id: 3, picking_version: 1, component_type: 'material', component_id: 30, component_name: '生豆-孟连水洗5T批次', unit: 'kg', demand_g: 12000, wip_covered_g: 0, transfer_g: 11000, shortage_g: 1000 },
      { id: 15, production_plan_item_id: 999, picking_version: 1, component_type: 'material', component_id: 51, component_name: '无法关联的备料', unit: 'kg', demand_g: 1000 },
    ],
    manufacturing_plan: { edges: [
      { consumer_plan_item_id: 1, supplier_plan_item_id: 3, required_g: 9000 },
      { consumer_plan_item_id: 2, supplier_plan_item_id: 3, required_g: 9000 },
    ] },
    supply_allocations: [], related_work_orders: [], operation_splits: [],
    readiness: { can_submit: false, blocking_count: 1, issues: [] },
  }
  const html = await renderToString(createSSRApp(Component, { detail, warehouses: [] }))

  assert.match(html, /同规格生产组/)
  assert.match(html, /<dt>需求<\/dt><dd>30 条<\/dd>/)
  assert.ok(html.indexOf('本组备料') < html.indexOf('展开 2 项订单任务'))
  assert.match(html, /data-source-id="11"/)
  assert.match(html, /data-source-id="12"/)
  assert.match(html, /来自阶段 1 · 卡蒂姆5T中深烘 9 kg/)
  assert.match(html, /生豆-孟连水洗5T批次/)
  assert.match(html, /class="group-preparation-row shortage"/)
  assert.match(html, /class="group-preparation-state shortage">缺料<\/span>/)
  assert.match(html, /未关联备料/)
  assert.match(html, /无法关联的备料/)
  assert.doesNotMatch(html, /id="material-sources"/)
  const taskDetailsTags = [...html.matchAll(/<details class="group-task-details"[^>]*>/g)].map(([tag]) => tag)
  assert.equal(taskDetailsTags.length, 2, 'single-task and multi-task group details are rendered')
  assert.match(taskDetailsTags[0], /\sopen(?:="")?(?:\s|>)/, 'single-task details stay directly visible')
  assert.doesNotMatch(taskDetailsTags[1], /\sopen(?:="")?(?:\s|>)/, 'multi-order task details start collapsed')
})

test('legacy plans keep their original source controls and expose unlinked source rows and old summaries', async () => {
  const Component = await workspaceComponent()
  const detail = {
    id: 18, plan_no: 'PP-LEGACY', status: 'draft',
    items: [item(8, 'SO-LEGACY')],
    component_sources: [
      { id: 51, production_plan_item_id: 8, component_type: 'material', component_id: 41, component_name: '旧计划咖啡袋', unit: '条', required_units: 10, source_warehouse: 'raw-a', source_owner_customer_id: 3, options: [{ warehouse: 'raw-a', warehouse_name: '原料仓', owner_customer_id: 3, owner_name: '客户甲', available_units: 20 }] },
      { id: 52, production_plan_item_id: 999, component_type: 'material', component_id: 42, component_name: '无任务编号来源', unit: 'kg', required_g: 1500 },
    ],
    material_summary: [{ component_type: 'material', name: '旧版总表材料', qty: 2, unit: '袋' }],
    readiness: { can_submit: false, blocking_count: 1, issues: [] },
  }
  const html = await renderToString(createSSRApp(Component, { detail, warehouses: [] }))

  assert.match(html, /旧计划咖啡袋/)
  assert.match(html, /<select/)
  assert.match(html, /原料仓（raw-a） · 客户甲/)
  assert.match(html, /未关联备料/)
  assert.match(html, /无任务编号来源/)
  assert.match(html, /旧版总表材料/)
})


test('group summaries show server pending and shortage states even when quantity snapshots are zero', async () => {
  const Component = await workspaceComponent()
  const detail = {
    id: 19, plan_no: 'PP-ZERO-SNAPSHOT', status: 'completed', picking_version: 1,
    items: [item(1, 'SO-A'), item(2, 'SO-B')],
    component_sources: [
      { id: 61, production_plan_item_id: 1, component_type: 'material', component_id: 70, component_name: '待入库熟豆', unit: 'kg', preparation_status: 'awaiting_production', demand_g: 0, upstream_g: 0, shortage_g: 0 },
      { id: 62, production_plan_item_id: 2, component_type: 'material', component_id: 70, component_name: '待入库熟豆', unit: 'kg', preparation_status: 'awaiting_production', demand_g: 0, upstream_g: 0, shortage_g: 0 },
      { id: 63, production_plan_item_id: 1, component_type: 'material', component_id: 41, component_name: '待领包装袋', unit: '条', preparation_status: 'awaiting_transfer' },
      { id: 64, production_plan_item_id: 1, component_type: 'material', component_id: 42, component_name: '缺少标签', unit: '条', preparation_status: 'shortage' },
    ],
    readiness: { can_submit: false, blocking_count: 1, issues: [] },
  }
  const html = await renderToString(createSSRApp(Component, { detail, warehouses: [] }))

  assert.match(html, /class="group-preparation-state">待生产入库<\/span>/)
  assert.match(html, /class="group-preparation-state">供给已落实<\/span>/)
  assert.match(html, /class="group-preparation-state shortage">缺料<\/span>/)
  assert.doesNotMatch(html, /现场已齐套/)
  assert.match(html, /data-source-id="61"/)
  assert.match(html, /data-source-id="62"/)
})
