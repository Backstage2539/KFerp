import { cloneValue } from './product-creator-graph.js'

export function filterWorkflowVariables(query, variables = []) {
  const needle = String(query || '').trim().toLocaleLowerCase()
  if (!needle) return variables
  const characters = [...needle]
  return variables.filter((variable) => {
    const haystack = `${variable.name || ''} ${variable.default_value || ''}`.toLocaleLowerCase()
    if (haystack.includes(needle)) return true
    let position = 0
    for (const character of characters) {
      position = haystack.indexOf(character, position)
      if (position < 0) return false
      position += character.length
    }
    return true
  })
}

export function renderNameParts(parts = [], variables = [], values = {}) {
  const definitions = new Map(variables.map((variable) => [variable.id, variable]))
  const missing = []
  let output = ''
  for (const part of parts) {
    if (part.type === 'text') {
      output += String(part.value || '')
      continue
    }
    if (part.type !== 'variable') continue
    const id = String(part.variable_id || '')
    const definition = definitions.get(id)
    const value = String(values[id] ?? definition?.default_value ?? '').trim()
    if (!value) missing.push(id)
    output += value
  }
  return { value: output.trim(), missing: [...new Set(missing)] }
}

export function upgradeWorkflowToV3(workflow = {}) {
  if (Number(workflow.version || 1) >= 3) return cloneValue(workflow)
  if (Number(workflow.version || 1) < 2) return cloneValue(workflow)

  const variables = cloneValue(workflow.variables || [])
  const idsByName = new Map(variables.map((variable) => [variable.name, variable.id]))
  const variableID = (rawName) => {
    const name = rawName.trim() === '商品' ? '商品名称' : rawName.trim()
    if (!name) return ''
    if (idsByName.has(name)) return idsByName.get(name)
    const id = `legacy-${[...name].map((character) => character.codePointAt(0).toString(16)).join('-')}`
    idsByName.set(name, id)
    variables.push({ id, name, default_value: '' })
    return id
  }

  const nodes = (workflow.nodes || []).map((node) => {
    const config = cloneValue(node.config || {})
    if (typeof config.name_pattern === 'string') {
      const parts = []
      const pattern = config.name_pattern
      const tokens = /\{\{([^{}]+)\}\}|\{(商品名称|商品)\}/g
      let cursor = 0
      let match
      while ((match = tokens.exec(pattern))) {
        if (match.index > cursor) parts.push({ type: 'text', value: pattern.slice(cursor, match.index) })
        const id = variableID(match[1] || match[2])
        if (id) parts.push({ type: 'variable', variable_id: id })
        cursor = tokens.lastIndex
      }
      if (cursor < pattern.length) parts.push({ type: 'text', value: pattern.slice(cursor) })
      config.name_parts = parts
      delete config.name_pattern
    }
    delete config.fixed_fields
    if (node.kind === 'material') {
      delete config.kind
      if (Array.isArray(config.rows)) config.rows = config.rows.map(({ kind, ...row }) => row)
    }
    if (node.kind === 'product') delete config.product_kind
    return { ...node, config }
  })

  const edgeIDs = new Set()
  const edges = (workflow.edges || []).map((edge, index) => {
    let id = String(edge.id || `legacy-edge-${index + 1}`)
    let suffix = 1
    while (edgeIDs.has(id)) id = `legacy-edge-${index + 1}-${++suffix}`
    edgeIDs.add(id)
    return { ...edge, id }
  })

  return { ...cloneValue(workflow), version: 3, variables, nodes, edges }
}
