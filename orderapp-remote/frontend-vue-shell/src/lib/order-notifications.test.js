import test from 'node:test'
import assert from 'node:assert/strict'
import {templateFields,canRetry,statusLabel,notificationEvents,normalizeNoticeConfig,approvedEnumValues,officialTestEvents,officialConfigEvents,canVerifyBusinessTest} from './order-notifications.js'
test('extracts exact template fields and rejects one-time subscription content',()=>{
 assert.deepEqual(templateFields('订单{{character_string1.DATA}}，状态{{thing2.DATA}} {{thing2.DATA}}'),['character_string1','thing2'])
 assert.deepEqual(templateFields('{{content.DATA}}'),[])
})
test('preserves legacy event settings while keeping an explicitly empty service account selection',()=>{
 assert.deepEqual(normalizeNoticeConfig({events:['created'],templates:{}}).official_events,['created'])
 assert.deepEqual(normalizeNoticeConfig({events:['created'],official_events:[],templates:{}}).official_events,[])
})
test('approved enum values are explicit and deduplicated',()=>{
 assert.deepEqual(approvedEnumValues('已接单\n 已发货\n已接单\n'),['已接单','已发货'])
 assert.deepEqual(approvedEnumValues(''),[])
})
test('order acceptance and shipment are available for service-account testing separately from group events',()=>{
 assert.deepEqual(officialTestEvents.slice(6,8).map(([key])=>key),['order_accepted','shipment_created'])
 assert.equal(notificationEvents.some(([key])=>key==='order_accepted'),false)
})
test('finance events are offered for service-account tests, not group events',()=>{
 assert.deepEqual(officialConfigEvents.slice(-2).map(([key])=>key),['receipt_received','bill_issued'])
 assert.equal(officialTestEvents.some(([key])=>key==='receipt_received'),true)
 assert.equal(notificationEvents.some(([key])=>key==='bill_issued'),false)
})
test('only delivered new service-account tests may be confirmed',()=>{
 const item={status:'delivered',channel:'wechat_service_account',notice:{test:true,test_detail:true,milestones:['order_accepted']}}
 assert.equal(canVerifyBusinessTest(item),true)
 assert.equal(canVerifyBusinessTest({...item,status:'accepted'}),false)
 assert.equal(canVerifyBusinessTest({...item,channel:'wechat_desktop'}),false)
 assert.equal(canVerifyBusinessTest({...item,notice:{...item.notice,test:false}}),false)
 assert.equal(canVerifyBusinessTest({...item,notice:{...item.notice,test_detail:false}}),false)
})
test('does not classify accepted or unknown as delivered or safe retry',()=>{
 assert.equal(canRetry('unknown'),false); assert.equal(canRetry('accepted'),false); assert.equal(canRetry('failed'),true)
 assert.equal(statusLabel('accepted'),'微信已受理，待回执');assert.equal(statusLabel('unknown'),'结果不明，需核对'); assert.equal(notificationEvents.length,6)
})
