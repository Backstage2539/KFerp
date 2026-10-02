import { cloneValue, materialSupplyMode } from './product-creator-graph.js'

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

export function renderNameParts(parts = [], variables = [], values = {}, options = {}) {
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
    if (!value) {
      missing.push(id)
      output += typeof options.missingPlaceholder === 'function' ? options.missingPlaceholder(definition, id) : ''
    } else {
      output += value
    }
  }
  return { value: output.trim(), missing: [...new Set(missing)] }
}

export function renderNamePreview(parts = [], variables = [], exampleValues = {}) {
  const definitions = new Map(variables.map((variable) => [variable.id, variable]))
  const previewValues = {}
  for (const [id, variable] of definitions) {
    previewValues[id] = exampleValues[id] ?? variable.default_value ?? ''
  }
  return renderNameParts(parts, variables, previewValues, {
    missingPlaceholder: (variable) => `【${variable?.name || '变量'}待填写】`,
  })
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

// V4 moves product variants, packaging, process, and loss from the product
// output BOM into a published BOM specification-template version. Preserve
// any prior hand-maintained product-BOM setup in a visible migration snapshot
// so the designer can require an explicit review before publishing.
export function upgradeWorkflowToV4(workflow = {}) {
  const sourceVersion = Number(workflow.version || 1)
  if (sourceVersion >= 4) return cloneValue(workflow)
  const source = upgradeWorkflowToV3(workflow)
  const nodes = (source.nodes || []).map((node) => {
    if (node.kind !== 'bom' || String(node.config?.output_type || '') !== 'product') return cloneValue(node)
    const config = cloneValue(node.config || {})
    const snapshot = {
      variants: cloneValue(config.variants || []),
      components: cloneValue(config.components || []),
      output_qty: config.output_qty,
      output_unit: config.output_unit,
      route_id: config.route_id,
      material_loss_rate: config.material_loss_rate,
    }
    const hasLegacySetup = snapshot.variants.length > 0 || snapshot.components.length > 0 ||
      Number(snapshot.route_id || 0) > 0 || Number(snapshot.material_loss_rate || 0) > 0 ||
      Number(snapshot.output_qty || 0) > 0 || Boolean(snapshot.output_unit)
    delete config.variants
    delete config.components
    delete config.output_qty
    delete config.output_unit
    delete config.route_id
    delete config.material_loss_rate
    config.spec_template_version_id = 0
    if (hasLegacySetup) {
      config.legacy_spec_configuration = snapshot
      config.legacy_spec_configuration_pending = true
    }
    return { ...node, config }
  })
  return { ...cloneValue(source), version: 4, nodes }
}

export function upgradeWorkflowToV5(workflow = {}) {
  const upgraded = upgradeWorkflowToV4(workflow)
  if (Number(upgraded.version || 1) >= 5) return upgraded
  return { ...upgraded, version: 5 }
}

export function upgradeWorkflowToV6(workflow = {}) {
  const upgraded = upgradeWorkflowToV5(workflow)
  if (Number(upgraded.version || 1) >= 6) return upgraded
  const nodes = (upgraded.nodes || []).map((node) => {
    if (node.kind !== 'material' || node.config?.data_role === 'output') return cloneValue(node)
    const config = cloneValue(node.config || {})
    if (!Array.isArray(config.default_rows)) {
      config.default_rows = Array.isArray(config.rows) ? config.rows : []
    }
    delete config.rows
    return { ...node, config }
  })
  // Historical purchase nodes stay in the V6 draft for explicit removal. The
  // designer renders them as legacy-only nodes and server validation blocks
  // publication until the user resolves that old behavior.
  return { ...cloneValue(upgraded), version: 6, nodes }
}

// Only an editable draft is upgraded; published versions and runs keep their snapshots.
export function upgradeWorkflowToV7(workflow = {}) {
  const draft = upgradeWorkflowToV6(workflow)
  return { ...draft, version: 7, nodes: (draft.nodes || []).map((node) => {
    if (node.kind !== 'material') return node
    const config = { ...node.config, supply_mode: materialSupplyMode(node.config) }
    config.data_role = config.supply_mode === 'manufacture' ? 'output' : 'input'
    delete config.default_rows
    delete config.rows
    if (config.data_role === 'input') {
      delete config.name_parts
      delete config.name_pattern
      delete config.name
      delete config.defaults
    }
    return { ...node, config }
  }) }
}

export function upgradeWorkflowToV8(workflow = {}) {
  if (Number(workflow.version) >= 8) return cloneValue(workflow)
  const draft = Number(workflow.version) >= 7 ? cloneValue(workflow) : upgradeWorkflowToV7(workflow)
  return { ...draft, version: 8 }
}
