import test from 'node:test'
import assert from 'node:assert/strict'
import {
  menuFromEditor,
  menuToEditor,
  fixedPricePath,
  menuAction,
  setMenuAction,
} from './wechat-official.js'
test('fixed path depends only on entry key, not version', () => {
  assert.equal(
    fixedPricePath({ key: 'abc', publication_id: 1 }),
    fixedPricePath({ key: 'abc', publication_id: 2 }),
  )
})
test('menus roundtrip groups, customer order actions and old view links', () => {
  const menu = {
    button: [
      {
        name: '我的订单',
        sub_button: [
          { name: '最近一次', type: 'click', key: 'ORDERS_RECENT_1' },
        ],
      },
      { name: '网站', type: 'view', url: 'https://example.com' },
    ],
  }
  assert.deepEqual(menuFromEditor(menuToEditor(menu)), menu)
})
test('unknown imported buttons remain visible until deliberately edited', () => {
  const menu = { button: [{ name: '资料', type: 'media_id', media_id: 'xyz' }] }
  assert.deepEqual(menuFromEditor(menuToEditor(menu)), menu)
})

test('imported historical price-entry paths roundtrip without being redirected', () => {
  const menu = {
    button: [{
      name: '旧价格表',
      type: 'miniprogram',
      appid: 'wx-mini',
      pagepath: 'pages/price-list/price-list?entry=0123456789abcdef0123456789abcdef',
    }],
  }
  assert.deepEqual(menuFromEditor(menuToEditor(menu)), menu)
})

test('import does not mislabel custom WeChat actions as order replies', () => {
  assert.equal(menuAction({ type: 'click', key: 'LEGACY_CLICK' }), 'custom')
  assert.equal(
    menuAction({ type: 'miniprogram', pagepath: 'pages/legacy/legacy' }),
    'custom',
  )
  const button = { name: '全部订单' }
  setMenuAction(button, 'orders', 'wx-mini')
  assert.equal(
    button.pagepath,
    'pages/service/service?key=orders&source=official',
  )
})
