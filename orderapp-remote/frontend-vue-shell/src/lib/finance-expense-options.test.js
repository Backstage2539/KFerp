import test from 'node:test'
import assert from 'node:assert/strict'

import {
  expenseCategoryOptions,
  expensePaymentOptions,
  filterExpenseOptions,
} from './finance-expense-options.js'

test('finance expense category and payment options cover common Chinese roastery expense entries', () => {
  assert.ok(expenseCategoryOptions.length >= 30)
  assert.ok(expensePaymentOptions.length >= 12)
  for (const value of ['人工', '房租', '物流快递', '水电燃气', '包材耗材', '外协加工费', '银行手续费']) {
    assert.ok(expenseCategoryOptions.includes(value), `missing category ${value}`)
  }
  for (const value of ['微信支付', '支付宝', '银行转账', '对公银行', '员工垫付', '应付未付']) {
    assert.ok(expensePaymentOptions.includes(value), `missing payment ${value}`)
  }
})

test('filterExpenseOptions fuzzy matches option text without removing custom input support', () => {
  assert.deepEqual(filterExpenseOptions(['微信支付', '支付宝', '银行转账'], '银行'), ['银行转账'])
  assert.deepEqual(filterExpenseOptions(['物流快递', '房租', '包材耗材'], '耗材'), ['包材耗材'])
  assert.deepEqual(filterExpenseOptions(['微信支付'], ''), ['微信支付'])
})
