import test from 'node:test'
import assert from 'node:assert/strict'
import { orderRowClass } from './order-row-state.js'

test('未付款始终红色，已发货且已付款才整行绿色', () => {
  assert.equal(orderRowClass({ pay_status: '未付款', ship_status: '已发货' })['state-unpaid'], true)
  assert.equal(orderRowClass({ pay_status: '未付款', ship_status: '已发货' })['state-shipped-paid'], false)
  assert.equal(orderRowClass({ pay_status: '已付款', ship_status: '已发货' })['state-shipped-paid'], true)
  assert.equal(orderRowClass({ pay_status: '已付款', ship_status: '待发货' })['state-shipped-paid'], false)
  assert.equal(orderRowClass({ pay_status: '部分付款', ship_status: '已发货' })['state-shipped-paid'], false)
  assert.equal(orderRowClass({ pay_status: '已付款', ship_status: '已发货', is_void: true })['state-shipped-paid'], false)
})

test('新订单高亮不能把未付款或其他订单涂成已完成绿色', () => {
  assert.equal(orderRowClass({ pay_status: '未付款', ship_status: '待发货' }, true)['state-unpaid'], true)
  assert.equal(orderRowClass({ pay_status: '已付款', ship_status: '待发货' }, true)['state-shipped-paid'], false)
  assert.equal(orderRowClass({ pay_status: '已付款', ship_status: '已发货' }, true)['state-shipped-paid'], true)
})
