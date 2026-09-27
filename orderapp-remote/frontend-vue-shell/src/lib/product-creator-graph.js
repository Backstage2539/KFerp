export function cloneValue(value) {
  return globalThis.structuredClone ? structuredClone(value) : JSON.parse(JSON.stringify(value))
}

export function toCanvasGraph(workflow = { nodes: [], edges: [] }, modules = []) {
  const moduleByKind = new Map(modules.map((module) => [module.kind, module]))
  return {
    nodes: (workflow.nodes || []).flatMap((node) => {
      const module = moduleByKind.get(node.kind)
      if (!module) return []
      return [{
        id: node.id,
        type: 'business',
        position: { x: Number(node.x) || 0, y: Number(node.y) || 0 },
      data: { module, label: node.name || module.name, config: cloneValue(node.config || {}), condition: node.condition ? cloneValue(node.condition) : null },
      }]
    }),
    edges: (workflow.edges || []).map((edge) => ({
      id: edge.id,
      source: edge.source,
      sourceHandle: edge.source_handle || undefined,
      target: edge.target,
      targetHandle: edge.target_handle || undefined,
      type: 'smoothstep',
      label: edge.label || '',
      data: { kind: edge.kind || 'data' },
      style: edge.kind === 'prerequisite' ? { strokeDasharray: '6 5' } : {},
      markerEnd: { type: 'arrowclosed', color: edge.kind === 'prerequisite' ? '#94a3b8' : '#75839b' },
    })),
  }
}

export function toWorkflowGraph(nodes = [], edges = []) {
  return {
    nodes: nodes.map((node) => ({
      id: node.id,
      kind: node.data.module.kind,
      name: node.data.label || node.data.module.name,
      x: Math.round(node.position.x),
      y: Math.round(node.position.y),
      config: cloneValue(node.data.config || {}),
      ...(node.data.condition ? { condition: cloneValue(node.data.condition) } : {}),
    })),
    edges: edges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      source_handle: edge.sourceHandle || '',
      target: edge.target,
      target_handle: edge.targetHandle || '',
      kind: edge.data?.kind || 'data',
      label: edge.label || '',
    })),
  }
}

export function connectionIsValid(connection, nodes, modules, mode = 'data') {
  if (!connection?.source || !connection?.target || connection.source === connection.target) return false
  const source = nodes.find((node) => node.id === connection.source)
  const target = nodes.find((node) => node.id === connection.target)
  if (!source || !target) return false
  if (mode === 'prerequisite') return true
  const sourcePort = source.data.module.outputs?.find((port) => port.id === connection.sourceHandle)
  const targetPort = target.data.module.inputs?.find((port) => port.id === connection.targetHandle)
  if (!sourcePort || !targetPort) return false
  return sourcePort.types?.some((sourceType) => targetPort.types?.includes(sourceType) || targetPort.types?.includes('*')) || false
}

export function moduleFieldLabel(module, key) {
  return module.inputs?.find((port) => port.id === key)?.label ||
    module.outputs?.find((port) => port.id === key)?.label ||
    module.fields?.find((field) => field.key === key)?.label || key
}

export function makeNodeId() {
  return globalThis.crypto?.randomUUID?.() || `node-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export function makeEdgeId() {
  return globalThis.crypto?.randomUUID?.() || `edge-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export function autoLayout(nodes, edges, { xGap = 205, yGap = 156, left = 90, top = 70 } = {}) {
  const incoming = new Map(nodes.map((node) => [node.id, 0]))
  const outgoing = new Map(nodes.map((node) => [node.id, []]))
  for (const edge of edges) {
    if (!incoming.has(edge.target) || !incoming.has(edge.source)) continue
    incoming.set(edge.target, incoming.get(edge.target) + 1)
    outgoing.get(edge.source).push(edge.target)
  }
  const ready = nodes.filter((node) => incoming.get(node.id) === 0).map((node) => node.id).sort()
  const rank = new Map(nodes.map((node) => [node.id, 0]))
  let seen = 0
  while (ready.length) {
    const current = ready.shift()
    seen += 1
    for (const next of outgoing.get(current) || []) {
      rank.set(next, Math.max(rank.get(next) || 0, (rank.get(current) || 0) + 1))
      incoming.set(next, incoming.get(next) - 1)
      if (incoming.get(next) === 0) {
        ready.push(next)
        ready.sort()
      }
    }
  }
  if (seen !== nodes.length) return nodes.map((node) => ({ ...node }))
  const byRank = new Map()
  for (const node of nodes) {
    const layer = rank.get(node.id) || 0
    if (!byRank.has(layer)) byRank.set(layer, [])
    byRank.get(layer).push(node)
  }
  const positions = new Map()
  for (const [layer, rows] of byRank) {
    rows.sort((a, b) => String(a.data.label || a.id).localeCompare(String(b.data.label || b.id), 'zh-CN'))
    rows.forEach((node, index) => positions.set(node.id, { x: left + layer * xGap, y: top + index * yGap }))
  }
  return nodes.map((node) => ({ ...node, position: positions.get(node.id) || node.position }))
}
