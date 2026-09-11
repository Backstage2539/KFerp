export async function loadProductionPlanWorkOrders(get, planID, status = '') {
  const plan = await get(`/api/production-plans/${Number(planID)}`)
  const ids = [...new Set((plan.related_work_orders || []).map(row => Number(row.id)).filter(id => id > 0))]
  const details = await Promise.all(ids.map(id => get(`/api/produce/work-orders/${id}`)))
  return {
    plan_no: plan.plan_no,
    rows: details.map(detail => detail.work_order).filter(row => row && Number(row.production_plan_id) === Number(planID) && (!status || row.status === status)),
  }
}
