// Mirrors the server's V8 dependency walk; client state is never authoritative.
export function isReusedOutput(workflow, node, inputs = {}) {
  const c = node.config || {}
  return Number(workflow.version) >= 8 && ['material', 'product'].includes(node.kind) && c.data_role === 'output' &&
    (inputs[node.id]?.action || c.object_action || c.action || c.defaults?.action) === 'reuse'
}
export function buildExecutionPlan(workflow = {}, inputs = {}) {
  const nodes = workflow.nodes || [], edges = workflow.edges || []
  const byID = new Map(nodes.map(n => [n.id, n])), incoming = new Map(), outgoing = new Set()
  const plan = Object.fromEntries(nodes.map(n => [n.id, { status: 'ready', reused_by_node_ids: [] }]))
  if (Number(workflow.version) < 8) return plan
  for (const e of edges) { incoming.set(e.target, [...(incoming.get(e.target) || []), e.source]); outgoing.add(e.source) }
  const active = new Set()
  function visit(id) {
    if (active.has(id) || !byID.has(id)) return
    active.add(id)
    if (!isReusedOutput(workflow, byID.get(id), inputs)) for (const p of incoming.get(id) || []) visit(p)
  }
  for (const n of nodes) if (!outgoing.has(n.id)) visit(n.id)
  for (const n of nodes) plan[n.id].status = !active.has(n.id) ? 'skipped' : isReusedOutput(workflow, n, inputs) ? 'reused' : 'ready'
  for (const n of nodes.filter(n => plan[n.id].status === 'reused')) {
    const seen = new Set()
    function trace(id) {
      if (seen.has(id)) return
      seen.add(id)
      if (plan[id]?.status === 'skipped') plan[id].reused_by_node_ids.push(n.id)
      for (const p of incoming.get(id) || []) trace(p)
    }
    for (const p of incoming.get(n.id) || []) trace(p)
  }
  for (const state of Object.values(plan)) state.reused_by_node_ids.sort()
  return plan
}
