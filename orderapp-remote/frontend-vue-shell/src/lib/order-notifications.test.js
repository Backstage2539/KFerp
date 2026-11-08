import test from 'node:test'
import assert from 'node:assert/strict'
import {templateFields,canRetry,statusLabel,notificationEvents} from './order-notifications.js'
test('extracts exact template fields and rejects one-time subscription content',()=>{
 assert.deepEqual(templateFields('订单{{character_string1.DATA}}，状态{{thing2.DATA}} {{thing2.DATA}}'),['character_string1','thing2'])
 assert.deepEqual(templateFields('{{content.DATA}}'),[])
})
test('does not classify accepted or unknown as delivered or safe retry',()=>{
 assert.equal(canRetry('unknown'),false); assert.equal(canRetry('accepted'),false); assert.equal(canRetry('failed'),true)
 assert.equal(statusLabel('accepted'),'微信已受理，待回执');assert.equal(statusLabel('unknown'),'结果不明，需核对'); assert.equal(notificationEvents.length,6)
})
