import {describe,it,expect} from 'vitest'
import {orderProgressLabel,directShipProgressLabel} from './orderProgress'
describe('shared order progress',()=>{it('renders server status for every client without losing inbound progress',()=>{expect(orderProgressLabel({fulfillment_progress:{label:'生产完成 · 已入库'}})).toBe('生产完成 · 已入库');expect(orderProgressLabel({fulfillment_progress:{label:'库存充足 · 可以发货'}})).toBe('库存充足 · 可以发货')});it('hides stale shipping for retained legacy responses',()=>{expect(orderProgressLabel({delivery_mode:'customer_stock',delivery_status:'received',process_status:'生产完成',ship_status:'待发货'})).toBe('生产完成 · 已入库')})})

it('direct ship requests use shared package progress while preserving cancellation',()=>{expect(directShipProgressLabel({status:'reserved',packages:[{fulfillment_progress:{label:'库存充足 · 可以发货'}}]})).toBe('库存充足 · 可以发货');expect(directShipProgressLabel({status:'cancelled',packages:[{fulfillment_progress:{label:'库存充足 · 可以发货'}}]})).toBe('已取消')})
