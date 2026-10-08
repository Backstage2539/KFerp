import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')

test('price template drawer selects scoped products and specifications and previews draft pricing', () => {
  const start = source.indexOf('aria-label="价格模板试算"')
  const end = source.indexOf('</section>', start)
  assert.ok(start > -1 && end > start, 'trial section must be inside the template editor drawer')
  const trial = source.slice(start, end)
  assert.match(trial, /<span>商品<\/span>[\s\S]*setPriceTemplateTrialProduct/)
  assert.match(trial, /<span>规格<\/span>[\s\S]*priceTemplateTrialSpecCandidates/)
  assert.match(trial, /正在计算/)
  assert.match(trial, /重试/)
  assert.match(trial, /生产成本/)
  assert.match(trial, /税费/)
  assert.match(trial, /取整变化/)
  assert.match(trial, /最低毛利预警/)
})

test('trial uses draft endpoint independently and cancels when context or editor closes', () => {
  assert.ok(/apiSend\('\/api\/costing\/pricing-rule-trial'/.test(source), 'trial uses the single read-only endpoint')
  assert.ok(/priceTemplateTrialPayload\(candidate, priceListPricingRuleEditorForm\.value, activeBeanListCustomerID\.value\)/.test(source), 'trial uses the current draft and customer')
  assert.ok(/priceTemplateTrialRunner\.cancel\(\)/.test(source), 'context changes invalidate pending results')
  assert.ok(/priceTemplateTrialCandidates = computed\(\(\) => buildPriceTemplateTrialCandidates\(priceListFlatRows\.value\)\)/.test(source), 'candidates come from selected flat price rows')
  assert.ok(/function closePriceListPricingRuleEditor\(\)[\s\S]*?priceTemplateTrialRunner\.cancel\(\)/.test(source), 'closing the editor cancels its trial')
})
