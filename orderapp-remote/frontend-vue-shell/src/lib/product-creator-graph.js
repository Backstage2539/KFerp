export function cloneValue(value) {
  if (globalThis.structuredClone) {
    try {
      return structuredClone(value)
    } catch {
      // Vue Flow exposes reactive proxies, which structuredClone cannot copy.
    }
  }
  const serialized = JSON.stringify(value)
  return serialized === undefined ? serialized : JSON.parse(serialized)
}

export function appendGraphSnapshot(history, currentIndex, snapshot, limit = 60) {
  const current = history[currentIndex]
  if (current && JSON.stringify(current) === JSON.stringify(snapshot)) return { history, index: currentIndex }
  const prior = history.slice(0, currentIndex + 1)
  prior.push(snapshot)
  const nextHistory = prior.slice(-limit)
  return { history: nextHistory, index: nextHistory.length - 1 }
}

export function toCanvasGraph(workflow = { nodes: [], edges: [] }, modules = []) {
  const version = Number(workflow.version || 1)
  const nodes = (workflow.nodes || []).flatMap((node) => {
      const module = moduleForNode(node, modules, version)
      if (!module) return []
      return [{
        id: node.id,
        type: 'business',
        position: { x: Number(node.x) || 0, y: Number(node.y) || 0 },
        data: { module, label: node.name || module.name, config: cloneValue(node.config || {}), condition: node.condition ? cloneValue(node.condition) : null },
      }]
  })
  const edges = (workflow.edges || []).map((edge) => ({
      id: edge.id,
      source: edge.source,
      sourceHandle: edge.source_handle || undefined,
      target: edge.target,
      targetHandle: version >= 3 && edge.target_handle === 'components' ? recipePortForEdge(edge.id) : edge.target_handle || undefined,
      type: 'smoothstep',
      label: edge.label || '',
      data: { kind: edge.kind || 'data' },
      style: edge.kind === 'prerequisite' ? { strokeDasharray: '6 5' } : {},
      markerEnd: { type: 'arrowclosed', color: edge.kind === 'prerequisite' ? '#94a3b8' : '#75839b' },
  }))
  if (version >= 3) {
    for (const node of nodes.filter((item) => item.data.module.kind === 'bom')) {
      const connected = edges.filter((edge) => edge.target === node.id && isRecipePort(edge.targetHandle))
      node.data.recipeInputs = connected.map((edge) => {
        const source = nodes.find((item) => item.id === edge.source)
        const sourceName = source?.data.label || source?.data.module.name || '配方来源'
        const label = edge.sourceHandle === 'specs' ? `${sourceName} · 商品规格` : sourceName
        const mainInputCandidate = version >= 4 && node.data.config?.output_type === 'product'
        return { id: edge.targetHandle, label: `${label}${mainInputCandidate ? ' · 主体候选' : ''}`, edgeId: edge.id }
      })
      node.data.recipeInputs.push({ id: 'components:add', label: '＋配方输入', add: true })
    }
  }
  return { nodes, edges }
}

export function moduleForNode(node, modules = [], version = 1) {
  const choices = modules.filter((module) => module.kind === node.kind)
  const base = choices.find((module) => Number(module.workflow_version || 1) === Number(version)) || choices[0]
  if (!base) return null
  const module = { ...base }
  if (version >= 2 && ['material', 'product'].includes(node.kind)) {
    module.inputs = node.config?.data_role === 'output'
      ? [{ id: 'from_bom', label: 'BOM产出', types: ['bom.output'], required: true }]
      : []
  }
  if (version >= 4 && node.kind === 'bom' && node.config?.output_type === 'product') {
    module.inputs = (module.inputs || []).filter((port) => port.id !== 'route').map((port) => port.id === 'components' ? { ...port, label: '规格主体候选' } : port)
    module.fields = (module.fields || [])
      .filter((field) => !['output_qty', 'output_unit', 'route_id', 'material_loss_rate', 'variants'].includes(field.key))
      .map((field) => field.key === 'spec_template_version_id' ? { ...field, required: true } : field)
  } else if (version >= 4 && node.kind === 'bom') {
    module.fields = (module.fields || []).filter((field) => field.key !== 'spec_template_version_id')
  }
  return module
}

export function connectionRoleUpdates(connection, nodes, mode = 'data', edges = []) {
  if (mode !== 'data') return {}
  const source = nodes.find((node) => node.id === connection?.source)
  const target = nodes.find((node) => node.id === connection?.target)
  if (!source || !target) return {}
  const sourceKind = source.data?.module?.kind
  const targetKind = target.data?.module?.kind
  if (sourceKind === 'bom' && ['material', 'product'].includes(targetKind) && connection.targetHandle === 'from_bom') {
    return { [target.id]: 'output' }
  }
  if (['material', 'product'].includes(sourceKind) && targetKind === 'bom' && canonicalInputPortID(connection.targetHandle) === 'components') {
    const hasProducingBOM = edges.some((edge) => edge.target === source.id && edge.targetHandle === 'from_bom' && edge.data?.kind !== 'prerequisite')
    return hasProducingBOM ? {} : { [source.id]: 'input' }
  }
  return {}
}

export function toWorkflowGraph(nodes = [], edges = [], version = 1, variables = []) {
  return {
    ...(version >= 2 ? { version } : {}),
    ...(version >= 3 ? { variables: cloneValue(variables) } : {}),
    nodes: nodes.map((node) => ({
      id: node.id,
      kind: node.data.module.kind,
      name: node.data.label || node.data.module.name,
      x: Math.round(node.position.x),
      y: Math.round(node.position.y),
      config: cloneValue(node.data.config || {}),
      ...(version < 2 && node.data.condition ? { condition: cloneValue(node.data.condition) } : {}),
    })),
    edges: edges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      source_handle: edge.sourceHandle || '',
      target: edge.target,
      target_handle: canonicalInputPortID(edge.targetHandle) || '',
      kind: edge.data?.kind || 'data',
      label: edge.label || '',
    })),
  }
}

export function connectionIsValid(connection, nodes, modules, mode = 'data', edges = []) {
  if (!connection?.source || !connection?.target || connection.source === connection.target) return false
  const source = nodes.find((node) => node.id === connection.source)
  const target = nodes.find((node) => node.id === connection.target)
  if (!source || !target) return false
  if (mode === 'prerequisite') return true
  const sourcePort = source.data.module.outputs?.find((port) => port.id === connection.sourceHandle)
  const targetPort = target.data.module.inputs?.find((port) => port.id === canonicalInputPortID(connection.targetHandle))
  if (!sourcePort || !targetPort) return false
  if (isRecipePort(connection.targetHandle) || connection.targetHandle === 'components:add') {
    const duplicate = edges.some((edge) => edge.id !== connection.id && edge.target === connection.target && edge.source === connection.source && edge.sourceHandle === connection.sourceHandle && canonicalInputPortID(edge.targetHandle) === 'components')
    if (duplicate) return false
  }
  return sourcePort.types?.some((sourceType) => targetPort.types?.includes(sourceType) || targetPort.types?.includes('*')) || false
}

export function recipePortForEdge(edgeID) {
  return `components:source:${edgeID}`
}

export function isRecipePort(portID) {
  return String(portID || '').startsWith('components:source:')
}

export function canonicalInputPortID(portID) {
  return isRecipePort(portID) || portID === 'components:add' ? 'components' : portID
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

export function replaceGraphEdge(edges = [], edge) {
  return [...edges.filter((current) => current.id !== edge.id), edge]
}

export function autoLayout(nodes, edges, { xGap = 320, yGap = 156, left = 90, top = 70 } = {}) {
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
