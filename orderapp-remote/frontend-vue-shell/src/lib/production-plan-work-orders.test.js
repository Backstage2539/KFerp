import test from 'node:test'
import assert from 'node:assert/strict'
import { loadProductionPlanWorkOrders } from './production-plan-work-orders.js'

test('plan work-order entry reads all linked documents without relying on the global first page', async () => {
  const calls=[]
  const get=async path=>{
    calls.push(path)
    if(path==='/api/production-plans/113') return {plan_no:'PP-113',related_work_orders:[{id:201},{id:502},{id:201}]}
    if(path.endsWith('/201')) return {work_order:{id:201,production_plan_id:113,status:'released'}}
    if(path.endsWith('/502')) return {work_order:{id:502,production_plan_id:113,status:'completed'}}
    throw Error('unexpected endpoint')
  }
  const result=await loadProductionPlanWorkOrders(get,113,'released')
  assert.equal(result.plan_no,'PP-113')
  assert.deepEqual(result.rows.map(r=>r.id),[201])
  assert.equal(calls.filter(p=>p.endsWith('/201')).length,1)
  assert.equal(calls.includes('/api/produce/work-orders'),false)
})
