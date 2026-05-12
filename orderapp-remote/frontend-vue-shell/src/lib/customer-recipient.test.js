import test from 'node:test'
import assert from 'node:assert/strict'

import { parseRecipientText } from './customer-recipient.js'

test('parseRecipientText detects name phone and address from pasted recipient block', () => {
  assert.deepEqual(parseRecipientText(`
收件人：张三
电话：13800138000
地址：云南省普洱市思茅区咖啡路 88 号
  `), {
    recipient_name: '张三',
    phone: '13800138000',
    address: '云南省普洱市思茅区咖啡路 88 号',
  })
})

test('parseRecipientText handles compact common address text', () => {
	assert.deepEqual(parseRecipientText('李四 13900139000 浙江省杭州市西湖区文三路 10 号'), {
		recipient_name: '李四',
		phone: '13900139000',
		address: '浙江省杭州市西湖区文三路 10 号',
	})
})

test('parseRecipientText handles address phone name order', () => {
	assert.deepEqual(parseRecipientText('云南省昆明市西山区西坝新村30号C区 15302787466 刘祎泊'), {
		recipient_name: '刘祎泊',
		phone: '15302787466',
		address: '云南省昆明市西山区西坝新村30号C区',
	})
})
