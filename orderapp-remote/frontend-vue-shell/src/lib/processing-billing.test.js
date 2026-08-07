import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import assert from 'node:assert/strict'

const view = fs.readFileSync(path.resolve('src/views/OutsourceSettingsView.vue'), 'utf8')
const app = fs.readFileSync(path.resolve('src/App.vue'), 'utf8')
const menu = fs.readFileSync(path.resolve('src/lib/menu-ia.js'), 'utf8')

test('outsource settings edits immutable processing billing rules for all supported actual bases', () => {
  for (const marker of [
    'actual_input_kg',
    'actual_output_kg',
    'actual_minutes',
    'actual_units',
    'fixed_per_work_order',
    'factory_material_actual_cost',
    '保存并发布新版本',
    '当前发布版本',
  ]) {
    assert.match(view, new RegExp(marker))
  }
})

test('outsource settings previews and confirms completed customer work order bills', () => {
  for (const marker of [
    '/api/finance/customer-processing-billing/candidates',
    '/api/finance/customer-processing-billing/preview',
    '/api/finance/customer-processing-billing/confirm',
    '选择客户',
    '选择已完工工单',
    '预览账单',
    '确认生成并推送账单',
  ]) {
    assert.match(view, new RegExp(marker))
  }
  assert.match(view, /async function loadTemplates\(\)[\s\S]*apiGet\('\/api\/outsource\/templates'\)/)
  assert.match(view, /async function loadBillingOptions\(\)[\s\S]*apiGet\('\/api\/finance\/customer-processing-billing\/options'\)/)
  assert.doesNotMatch(view, /\/api\/customer-fulfillment\/customers/)
})

test('template maintenance and finance billing permissions fail independently', () => {
  for (const marker of [
    'templateAccessDenied',
    'billingAccessDenied',
    '当前账号没有代加工模板维护权限',
    '当前账号没有代加工账单查看权限',
    "err.status === 403",
    'Promise.all([loadTemplates(), loadBillingOptions()])',
  ]) {
    assert.match(view, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
})

test('finance users can reach processing billing without the settings-only view gate', () => {
  assert.match(app, /processingBilling:\s*OutsourceSettingsView/)
  assert.match(menu, /key:\s*'processingBilling',\s*label:\s*'代加工账单'/)
})

test('outsource settings manages confirmed processing bill payment reversal and immutable adjustments', () => {
  for (const marker of [
    '/api/finance/customer-processing-billing/runs?',
    '/pay',
    '/reverse',
    '/adjustments',
    '账单生命周期',
    '登记付款',
    '冲销账单',
    '新增调整单',
    '原账单计算快照不会被修改',
    'roasting',
    'labor',
    'material',
  ]) {
    assert.match(view, new RegExp(marker))
  }
})
