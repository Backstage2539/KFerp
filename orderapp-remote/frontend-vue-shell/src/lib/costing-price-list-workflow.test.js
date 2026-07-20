import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import fs from 'node:fs'
import * as priceListWorkflow from './costing-price-list-workflow.js'
import {
  dedupePriceListFlatRows,
  priceListPricingRuleTrialRequestsForRows,
} from './costing-price-list-workflow.js'

describe('costing price-list workflow helpers', () => {
  it('selects uncached pricing-rule trial requests and skips terminal cache rows', () => {
    const rows = [
      { row_key: 'a', product_id: 7, pricing_rule_id: 11 },
      { row_key: 'b', product_id: 8, pricing_rule_id: 12 },
      { row_key: 'c', product_id: 9, pricing_rule_id: 0 },
    ]
    const requests = priceListPricingRuleTrialRequestsForRows(rows, {
      customerID: 3,
      cache: { '8:12:3': { status: 'success' } },
      payloadForRow: (row, context) => (
        row.pricing_rule_id > 0
          ? { product_id: row.product_id, pricing_rule_id: row.pricing_rule_id, customer_id: context.customerID }
          : null
      ),
      cacheKeyForPayload: (payload) => `${payload.product_id}:${payload.pricing_rule_id}:${payload.customer_id}`,
    })

    assert.deepEqual(requests, [{
      row: rows[0],
      payload: { product_id: 7, pricing_rule_id: 11, customer_id: 3 },
      key: '7:11:3',
    }])
  })

  it('executes pricing-rule trials in 100-row batches and maps response indexes back to cache keys', async () => {
    const requests = Array.from({ length: 201 }, (_, index) => ({
      key: `key-${index}`,
      payload: { sequence: index + 1 },
    }))
    const batchSizes = []
    const completed = await priceListWorkflow.executePriceListPricingRuleTrialBatches(requests, {
      sendBatch: async (payloads) => {
        batchSizes.push(payloads.length)
        return {
          rows: payloads.map((payload, index) => ({
            index,
            result: { final_unit_price: payload.sequence },
          })).reverse(),
        }
      },
    })

    assert.deepEqual(batchSizes, [100, 100, 1])
    assert.deepEqual(completed['key-0'], { status: 'success', result: { final_unit_price: 1 } })
    assert.deepEqual(completed['key-100'], { status: 'success', result: { final_unit_price: 101 } })
    assert.deepEqual(completed['key-200'], { status: 'success', result: { final_unit_price: 201 } })
  })

  it('isolates partial batch errors and rejects a zero-price trial instead of publishing a stale automatic price', async () => {
    const requests = [
      { key: 'valid', payload: { product_id: 1 } },
      { key: 'failed', payload: { product_id: 2 } },
      { key: 'zero', payload: { product_id: 3 } },
    ]
    const completed = await priceListWorkflow.executePriceListPricingRuleTrialBatches(requests, {
      sendBatch: async () => ({
        rows: [
          { index: 2, result: { final_unit_price: 0, warnings: ['该商品暂无可试算的标准制造成本'] } },
          { index: 0, result: { final_unit_price: 88 } },
          { index: 1, error: 'BOM detail unavailable' },
        ],
      }),
    })

    assert.equal(completed.valid.status, 'success')
    assert.deepEqual(completed.failed, { status: 'error', error: 'BOM detail unavailable' })
    assert.deepEqual(completed.zero, { status: 'error', error: '该商品暂无可试算的标准制造成本' })

    const staleRow = {
      pricing_mode: 'pricing_rule',
      pricing_rule_id: 7,
      pricing_rule_version: 'V1',
      final_unit_price: 88,
      price_unit: 'kg',
      inventory_unit: 'kg',
      group_snapshot: { group_item_name: '意式拼配豆' },
      cost_source_snapshot: { pricing_rule_version: 'V1' },
    }
    assert.equal(priceListWorkflow.priceListFlatRowsReady([staleRow], { trialStatusForRow: () => completed.zero.status }), false)
  })

  it('marks an aborted batch as timed out and makes only failed cache entries retryable', async () => {
    const requests = [{ key: 'failed', payload: { product_id: 1 } }]
    let clearedTimerID = 0
    const timedOut = await priceListWorkflow.executePriceListPricingRuleTrialBatches(requests, {
      scheduleTimeout: (callback) => {
        callback()
        return 9
      },
      cancelTimeout: (timerID) => {
        clearedTimerID = timerID
      },
      sendBatch: async (_payloads, { signal }) => {
        assert.equal(signal.aborted, true)
        const err = new Error('aborted')
        err.name = 'AbortError'
        throw err
      },
    })
    assert.equal(clearedTimerID, 9)
    assert.deepEqual(timedOut.failed, { status: 'error', error: '价格计算超时，请重新试算' })

    const retryCache = priceListWorkflow.priceListPricingRuleTrialCacheForRetry({
      failed: timedOut.failed,
      success: { status: 'success', result: { final_unit_price: 88 } },
      loading: { status: 'loading' },
    }, ['failed', 'success', 'loading'])
    assert.equal(retryCache.failed, undefined)
    assert.equal(retryCache.success.status, 'success')
    assert.equal(retryCache.loading.status, 'loading')

    const retried = await priceListWorkflow.executePriceListPricingRuleTrialBatches(requests, {
      sendBatch: async (_payloads, { signal }) => {
        assert.equal(signal.aborted, false)
        return { rows: [{ index: 0, result: { final_unit_price: 91 } }] }
      },
    })
    assert.deepEqual(retried.failed, { status: 'success', result: { final_unit_price: 91 } })
  })

  it('keeps distinct template tiers when the product, pricing rule and unit are the same', () => {
    const rows = [
      {
        row_key: '556:tier-template:11:25',
        product_id: 556,
        product_name: '熟豆-白巧坚果拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 25,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '24kg',
        price_unit: 'kg',
      },
      {
        row_key: '556:tier-template:11:26',
        product_id: 556,
        product_name: '熟豆-白巧坚果拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 26,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '1kg',
        price_unit: 'kg',
      },
      {
        row_key: '557:tier-template:11:26',
        product_id: 557,
        product_name: '曜石2.0',
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 26,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '1kg',
        price_unit: 'kg',
      },
    ]

    const got = dedupePriceListFlatRows(rows)

    assert.deepEqual(got.map((row) => row.row_key), [
      '556:tier-template:11:25',
      '556:tier-template:11:26',
      '557:tier-template:11:26',
    ])
  })

  it('collapses only the same generated template-tier row and keeps the manual adjustment', () => {
    const rows = [
      {
        row_key: '556:tier-template:11:25',
        product_id: 556,
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 25,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '24kg',
        min_qty: 24,
        max_qty: 47,
        price_unit: 'kg',
        final_unit_price: 88,
      },
      {
        row_key: '556:tier-template:11:25',
        product_id: 556,
        pricing_mode: 'tier_template',
        tier_template_id: 11,
        template_tier_id: 25,
        pricing_rule_id: 11,
        tier_pricing_rule_id: 11,
        tier_label: '24kg',
        min_qty: 24,
        max_qty: 47,
        price_unit: 'kg',
        final_unit_price: 86,
        manual_adjusted: true,
      },
    ]

    const got = dedupePriceListFlatRows(rows)

    assert.equal(got.length, 1)
    assert.equal(got[0].final_unit_price, 86)
    assert.equal(got[0].manual_adjusted, true)
  })

  it('uses the template-tier identity when legacy rows have no generated row key', () => {
    const base = {
      product_id: 556,
      pricing_mode: 'tier_template',
      tier_template_id: 11,
      pricing_rule_id: 11,
      tier_pricing_rule_id: 11,
      price_unit: 'kg',
    }
    const rows = [
      { ...base, template_tier_id: 25, tier_label: '24kg', min_qty: 24, max_qty: 47 },
      { ...base, template_tier_id: 26, tier_label: '1kg', min_qty: 1, max_qty: 13 },
      { ...base, template_tier_id: 25, tier_label: '24kg', min_qty: 24, max_qty: 47 },
    ]

    assert.deepEqual(dedupePriceListFlatRows(rows).map((row) => row.template_tier_id), [25, 26])
  })

  it('does not let a repeated legacy row key merge different template tiers', () => {
    const base = {
      row_key: 'legacy-reused-row-key',
      product_id: 556,
      pricing_mode: 'tier_template',
      tier_template_id: 11,
      pricing_rule_id: 11,
      tier_pricing_rule_id: 11,
      price_unit: 'kg',
    }
    const rows = [
      { ...base, template_tier_id: 25, tier_label: '24kg', min_qty: 24, max_qty: 47 },
      { ...base, template_tier_id: 26, tier_label: '1kg', min_qty: 1, max_qty: 13 },
    ]

    assert.deepEqual(dedupePriceListFlatRows(rows).map((row) => row.template_tier_id), [25, 26])
  })

  it('keeps a camel-case manual adjustment when duplicate legacy rows collapse', () => {
    const base = {
      product_id: 556,
      pricing_mode: 'tier_template',
      tier_template_id: 11,
      template_tier_id: 25,
      pricing_rule_id: 11,
      tier_pricing_rule_id: 11,
      price_unit: 'kg',
      original_final_unit_price: 88,
    }
    const rows = [
      { ...base, final_unit_price: 88 },
      { ...base, final_unit_price: 86, manualAdjusted: true },
    ]

    const got = dedupePriceListFlatRows(rows)

    assert.equal(got.length, 1)
    assert.equal(got[0].final_unit_price, 86)
  })

  it('falls back to the tier label and quantity range when legacy rows have no tier id', () => {
    const base = {
      product_id: 556,
      pricing_mode: 'tier_template',
      tier_template_id: 11,
      template_tier_id: 0,
      pricing_rule_id: 11,
      tier_pricing_rule_id: 11,
      price_unit: 'kg',
    }
    const rows = [
      { ...base, tier_label: '24kg', min_qty: 24, max_qty: 47 },
      { ...base, tier_label: '1kg', min_qty: 1, max_qty: 13 },
      { ...base, tier_label: '24kg', min_qty: 24, max_qty: 47 },
    ]

    assert.deepEqual(dedupePriceListFlatRows(rows).map((row) => row.tier_label), ['24kg', '1kg'])
  })

  it('keeps tier-template flat rows when tiers use different pricing rules', () => {
    const rows = [
      {
        row_key: '88:tier-template:3:31',
        product_id: 88,
        product_name: '初晓拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 31,
        pricing_rule_id: 41,
        tier_pricing_rule_id: 41,
        price_unit: 'kg',
      },
      {
        row_key: '88:tier-template:3:32',
        product_id: 88,
        product_name: '初晓拼配',
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 32,
        pricing_rule_id: 42,
        tier_pricing_rule_id: 42,
        price_unit: 'kg',
      },
    ]

    assert.deepEqual(dedupePriceListFlatRows(rows).map((row) => row.row_key), [
      '88:tier-template:3:31',
      '88:tier-template:3:32',
    ])
  })

  it('treats child SKU id as the price-list row identity', () => {
    const rows = [
      {
        row_key: 'parent88:sku101:tier31',
        product_id: 88,
        sku_id: 101,
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 31,
        tier_pricing_rule_id: 41,
        price_unit: '袋',
      },
      {
        row_key: 'parent88:sku102:tier31',
        product_id: 88,
        sku_id: 102,
        pricing_mode: 'tier_template',
        tier_template_id: 3,
        template_tier_id: 31,
        tier_pricing_rule_id: 41,
        price_unit: '袋',
      },
    ]

    assert.deepEqual(dedupePriceListFlatRows(rows).map((row) => row.row_key), [
      'parent88:sku101:tier31',
      'parent88:sku102:tier31',
    ])
  })

  it('shows child SKU spec in flat price row titles', () => {
    assert.equal(typeof priceListWorkflow.priceListFlatRowDisplayTitle, 'function')
    assert.equal(typeof priceListWorkflow.priceListFlatRowPriceUnitLabel, 'function')

    const bagRow = {
      product_name: '榛巧拼配',
      sku_name: '227g袋装',
      spec_label: '227g',
      price_unit: '袋',
    }
    assert.equal(priceListWorkflow.priceListFlatRowDisplayTitle(bagRow), '榛巧拼配（227g袋装）')
    assert.equal(priceListWorkflow.priceListFlatRowPriceUnitLabel(bagRow), '227g')

    assert.equal(priceListWorkflow.priceListFlatRowDisplayTitle({
      product_name: '榛巧拼配',
      spec_label: '227g',
      net_content_qty: 227,
      net_content_unit: 'g',
      price_unit: '袋',
    }), '榛巧拼配（227g）')
  })

  it('shows compact sales spec labels when a default spec uses a named package unit', () => {
    const row = {
      product_name: '榛巧拼配',
      sku_name: '227g袋装',
      price_unit: '227g袋装',
      inventory_unit: 'kg',
      inventory_conversion_json: { '227g袋装': { kg: 0.227 } },
    }

    assert.equal(priceListWorkflow.priceListFlatRowDisplayTitle(row), '榛巧拼配（227g袋装）')
    assert.equal(priceListWorkflow.priceListFlatRowPriceUnitLabel(row), '227g')
    assert.deepEqual(priceListWorkflow.priceListFlatRowErrors({
      ...row,
      pricing_mode: 'fixed_price',
      fixed_unit_price: 59.92,
      final_unit_price: 59.92,
      group_snapshot: { group_item_name: '意式拼配豆' },
      cost_source_snapshot: { pricing_rule_version: 'V1' },
    }), [])
  })

  it('returns item-specific publish errors for flat price rows', () => {
    assert.equal(typeof priceListWorkflow.priceListFlatRowErrors, 'function')
    assert.equal(typeof priceListWorkflow.priceListFlatRowsReady, 'function')

    const badRow = {
      product_name: '榛巧拼配',
      sku_name: '227g袋装',
      pricing_mode: 'tier_template',
      tier_template_id: 3,
      template_tier_id: 0,
      pricing_rule_id: 0,
      price_unit: '袋',
      inventory_unit: 'g',
      inventory_conversion_json: {},
      group_snapshot: {},
      cost_source_snapshot: {},
      final_unit_price: 0,
    }
    const errors = priceListWorkflow.priceListFlatRowErrors(badRow)

    assert.deepEqual(errors, [
      '榛巧拼配（227g袋装）：缺少阶梯档位',
      '榛巧拼配（227g袋装）：缺少计算模板',
      '榛巧拼配（227g袋装）：最终价必须大于 0',
      '榛巧拼配（227g袋装）：缺少 袋 到 g 的换算',
      '榛巧拼配（227g袋装）：缺少价格表分组快照',
      '榛巧拼配（227g袋装）：缺少成本来源快照',
    ])
    assert.equal(priceListWorkflow.priceListFlatRowsReady([badRow]), false)
  })

  it('does not show a final-price error while the live pricing trial is loading', () => {
    const loadingRow = {
      product_name: '榛巧拼配',
      pricing_mode: 'tier_template',
      tier_template_id: 3,
      template_tier_id: 31,
      pricing_rule_id: 7,
      pricing_rule_version: 'V1',
      price_unit: 'kg',
      inventory_unit: 'kg',
      group_snapshot: { group_item_name: '意式拼配豆' },
      cost_source_snapshot: { pricing_rule_version: 'V1' },
      final_unit_price: 0,
    }

    assert.deepEqual(priceListWorkflow.priceListFlatRowErrors(loadingRow, { trialStatus: 'loading' }), [])
    assert.deepEqual(priceListWorkflow.priceListFlatRowErrors(loadingRow), ['榛巧拼配：最终价必须大于 0'])
    assert.equal(priceListWorkflow.priceListFlatRowsReady([loadingRow]), false, 'loading rows still cannot be published')
  })

  it('blocks stale automatic prices until the live trial succeeds and exposes trial failures', () => {
    const staleRow = {
      product_name: '榛巧拼配',
      pricing_mode: 'tier_template',
      tier_template_id: 3,
      template_tier_id: 31,
      pricing_rule_id: 7,
      pricing_rule_version: 'V1',
      price_unit: 'kg',
      inventory_unit: 'kg',
      group_snapshot: { group_item_name: '意式拼配豆' },
      cost_source_snapshot: { pricing_rule_version: 'V1' },
      final_unit_price: 88,
    }
    const readyWithStatus = (status) => priceListWorkflow.priceListFlatRowsReady([staleRow], { trialStatusForRow: () => status })

    assert.equal(readyWithStatus(''), false)
    assert.equal(readyWithStatus('loading'), false)
    assert.equal(readyWithStatus('error'), false)
    assert.equal(readyWithStatus('success'), true)
    assert.deepEqual(priceListWorkflow.priceListFlatRowErrors(staleRow, { trialStatus: 'error', trialError: 'BOM detail unavailable' }), [
      '榛巧拼配：价格计算失败：BOM detail unavailable',
    ])
    assert.equal(priceListWorkflow.priceListFlatRowsReady([{ ...staleRow, manual_adjusted: true }], { trialStatusForRow: () => 'error' }), true)
  })

  it('blocks a tier template whose quantity unit differs from the product sales spec even when units are convertible', () => {
    const incompatibleRow = {
      product_name: '初晓',
      pricing_mode: 'tier_template',
      tier_template_id: 3,
      tier_template_name: '咖啡熟豆',
      template_tier_id: 31,
      tier_quantity_unit: 'kg',
      tier_unit_compatible: false,
      tier_unit_compatibility_error: '阶梯模板不可用：商品规格“磅”与阶梯规格“kg”不匹配',
      pricing_rule_id: 7,
      pricing_rule_version: 'V1',
      price_unit: 'lb',
      inventory_unit: 'kg',
      inventory_conversion_json: { lb: { kg: 0.454 } },
      group_snapshot: { group_item_name: '咖啡豆' },
      cost_source_snapshot: { pricing_rule_version: 'V1' },
      final_unit_price: 88,
    }

    assert.deepEqual(priceListWorkflow.priceListFlatRowErrors(incompatibleRow), [
      '阶梯模板不可用：商品规格“磅”与阶梯规格“kg”不匹配',
    ])
    assert.equal(priceListWorkflow.priceListFlatRowsReady([incompatibleRow]), false)
    assert.equal(priceListWorkflow.priceListFlatRowsReady([{ ...incompatibleRow, manual_adjusted: true }]), false)
  })

  it('product price list flat rows read product master sales unit conversion', () => {
    const source = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')

    assert.match(source, /default_sales_unit/)
    assert.match(source, /unit_conversion_json/)
    assert.match(source, /itemSkuID/)
    assert.match(source, /sku_snapshot/)
    assert.match(source, /parent_product_id/)
    assert.match(source, /priceListFlatRowUnitSummary/)
    assert.match(source, /商品档案单位/)
  })

  it('product price list flat rows render SKU spec and row-level errors in the editor', () => {
    const source = fs.readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
    const flatRowEditor = source.match(/<div v-if="priceListFlatRows\.length" class="pdf-picker flat-price-row-editor"[\s\S]*?<div class="pdf-preview-title">/)?.[0] || ''
    const previewTitle = source.match(/<div class="pdf-preview-title"[\s\S]*?<div class="pdf-preview-phone/)?.[0] || ''

    assert.match(flatRowEditor, /priceListFlatRowDisplayTitle\(row\)/)
    assert.match(flatRowEditor, /priceListFlatRowVisibleErrors\(row\)/)
    assert.match(flatRowEditor, /priceListFlatRowPricingTrialStatus\(row\)/)
    assert.match(flatRowEditor, /flat-price-row-error-list/)
    assert.match(flatRowEditor, /hasPriceListFlatRowError\(row\)/)
    assert.doesNotMatch(flatRowEditor, /发布前需要为每行补齐计价模式/)
    assert.doesNotMatch(previewTitle, /price-list-publish-guard/)
  })
})
