import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'

function read(path) {
  return fs.readFileSync(new URL(path, import.meta.url), 'utf8')
}

test('PR-667 ERP delivery keeps product, demand, work-order and receipt trace screens connected', () => {
  const app = read('../App.vue')
  const product = read('../views/ProductSettingsView.vue')
  const demand = read('../views/ProducePlanView.vue')
  const workOrder = read('../components/ProductionExecutionHubDrawer.vue')
  const receipt = read('../views/StockEntriesView.vue')
  const customer = read('../views/CustomerProcessingPortalView.vue')

  for (const label of ['客户归属', '是否代加工商品', '默认已发布 BOM · 配方与工艺', '工艺路线', '目标客户成品仓', '客户别名与代发价格表']) {
    assert.match(product, new RegExp(label))
  }
  assert.match(product, /customer-order-price-table-bindings\?customer_id=/)
  assert.match(product, /一件代发已绑定/)
  for (const label of ['需求来源', '客户工单', '目标仓库', '现货不冲减客户申请数量']) {
    assert.match(demand, new RegExp(label))
  }
  for (const label of ['客户生产申请', '生产计划', '目标仓库 / 货权', '物料预订', '关联']) {
    assert.match(workOrder, new RegExp(label))
  }
  for (const label of ['完工入库结果', '订单占用', '现货可用', '成品批次', '查看客户申请']) {
    assert.match(receipt, new RegExp(label))
  }
  assert.match(workOrder, /navigate\('customerProcessing',[\s\S]*processing_request_id/)
  assert.match(receipt, /navigateFromCompletion\('customerProcessing',[\s\S]*processing_request_no/)
  assert.match(receipt, /@click="openExisting\(row\)"/)
  assert.doesNotMatch(receipt, /v-if="!row\.legacy" class="link" type="button" @click="openExisting\(row\)"/)
  assert.match(receipt, /const stockEntryID = Number\(params\.stock_entry_id \|\| 0\)/)
  assert.match(receipt, /await openExisting\(row\)/)
  assert.match(customer, /openRequestedProcessingRequest/)
  assert.match(app, /'processing_request_id', 'processing_request_no'/)
  assert.match(app, /'stock_entry_id'/)
})
