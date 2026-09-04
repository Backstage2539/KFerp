import {
  businessGroupItemMoveOptions,
  businessGroupVisibleName,
  buildBusinessGroupAssignmentPayload,
  isSystemDefaultBusinessGroup,
} from './product-settings.js'
import { clampPage, normalizePageSize, slicePageRows } from './pagination.js'

function toNumber(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}

function normalizedText(value) {
  return String(value ?? '').trim()
}

function assignmentUsage(row = {}) {
  return normalizedText(row.usage_key ?? row.usageKey)
}

function assignmentObjectKey(row = {}) {
  return normalizedText(row.object_key ?? row.objectKey)
}

function assignmentObjectID(row = {}) {
  return toNumber(row.object_id ?? row.objectID)
}

function assignmentObjectRef(row = {}) {
  return normalizedText(row.object_ref ?? row.objectRef)
}

export function businessGroupRowsForUsage(groups = [], usageKey = '') {
  const normalizedUsage = normalizedText(usageKey)
  return (Array.isArray(groups) ? groups : [])
    .filter((group) => group?.active !== false)
    .filter((group) => !isSystemDefaultBusinessGroup(group))
    .filter((group) => {
      if (!normalizedUsage) return true
      const usages = Array.isArray(group.usages) ? group.usages : []
      return usages.some((usage) => usage.active !== false && assignmentUsage(usage) === normalizedUsage)
    })
    .slice()
    .sort((a, b) => toNumber(a.sort_order ?? a.sortOrder) - toNumber(b.sort_order ?? b.sortOrder) || toNumber(a.id) - toNumber(b.id))
}

function uniquePositiveIDs(values = []) {
  const out = []
  const seen = new Set()
  for (const value of Array.isArray(values) ? values : []) {
    const id = toNumber(value)
    if (!(id > 0) || seen.has(id)) continue
    seen.add(id)
    out.push(id)
  }
  return out
}

export function businessGroupFeatureSelectionIDs(selection = {}) {
  return uniquePositiveIDs(selection?.group_template_ids)
}

export function businessGroupFeatureSelectionPayload(featureKey = '', groupTemplateIDs = []) {
  return {
    feature_key: normalizedText(featureKey),
    group_template_ids: uniquePositiveIDs(groupTemplateIDs),
  }
}

export function businessGroupRowsForFeatureSelection(groups = [], groupTemplateIDs = []) {
  const groupsByID = new Map((Array.isArray(groups) ? groups : [])
    .filter((group) => group?.active !== false)
    .filter((group) => !isSystemDefaultBusinessGroup(group))
    .filter((group) => toNumber(group?.id) > 0)
    .map((group) => [toNumber(group.id), group]))
  return uniquePositiveIDs(groupTemplateIDs)
    .map((id) => groupsByID.get(id))
    .filter(Boolean)
}

export function businessGroupControlOptions(groups = [], {
  selectedTemplateID = 0,
  usageKey = '',
} = {}) {
  const templateOptions = (Array.isArray(groups) ? groups : [])
    .filter((group) => group?.active !== false)
    .filter((group) => !isSystemDefaultBusinessGroup(group))
    .slice()
    .sort((a, b) => toNumber(a.sort_order) - toNumber(b.sort_order) || toNumber(a.id) - toNumber(b.id))
    .map((group) => ({
      id: toNumber(group.id),
      label: businessGroupVisibleName(group) || normalizedText(group.name) || `分组模板 #${toNumber(group.id)}`,
      group,
    }))
    .filter((option) => option.id > 0)
  const selectedTemplate = templateOptions.find((option) => option.id === toNumber(selectedTemplateID))?.group || null
  const moveOptions = businessGroupItemMoveOptions(selectedTemplate ? [selectedTemplate] : [], usageKey, {
    includeGroupName: false,
    includeGroupsWithoutUsage: true,
  }).map((option, index) => ({
    ...option,
    key: `${option.group_id}:${option.group_item_id}`,
    sort_order: index,
  }))
  return { templateOptions, selectedTemplate, moveOptions }
}

export function preferredBusinessGroupTemplateID(groups = [], {
  selectedTemplateID = 0,
  usageKey = '',
  preferredNames = [],
  preferredNameIncludes = [],
} = {}) {
  const { templateOptions } = businessGroupControlOptions(groups, { selectedTemplateID, usageKey })
  const selectedID = toNumber(selectedTemplateID)
  if (selectedID > 0 && templateOptions.some((option) => toNumber(option.id) === selectedID)) return selectedID

  const normalizedUsage = normalizedText(usageKey)
  if (normalizedUsage) {
    const usageMatch = templateOptions.find((option) => {
      const usages = Array.isArray(option.group?.usages) ? option.group.usages : []
      return usages.some((usage) => assignmentUsage(usage) === normalizedUsage && usage.active !== false)
    })
    if (usageMatch) return toNumber(usageMatch.id)
  }

  const exactNames = new Set((Array.isArray(preferredNames) ? preferredNames : []).map(normalizedText).filter(Boolean))
  if (exactNames.size) {
    const exactMatch = templateOptions.find((option) => {
      const labels = [
        option.label,
        option.group?.name,
        businessGroupVisibleName(option.group),
        option.group?.code,
      ].map(normalizedText).filter(Boolean)
      return labels.some((label) => exactNames.has(label))
    })
    if (exactMatch) return toNumber(exactMatch.id)
  }

  const includeTokens = (Array.isArray(preferredNameIncludes) ? preferredNameIncludes : []).map(normalizedText).filter(Boolean)
  if (includeTokens.length) {
    const includeMatch = templateOptions.find((option) => {
      const labels = [
        option.label,
        option.group?.name,
        businessGroupVisibleName(option.group),
        option.group?.code,
      ].map(normalizedText).filter(Boolean)
      return labels.some((label) => includeTokens.some((token) => label.includes(token)))
    })
    if (includeMatch) return toNumber(includeMatch.id)
  }

  return toNumber(templateOptions[0]?.id)
}

export function findBusinessGroupAssignmentForRow(row = {}, {
  assignments = [],
  usageKey = '',
  objectKey = '',
  objectIDForRow = (item = {}) => toNumber(item.id ?? item.product_id ?? item.productID),
  objectRefForRow = null,
  rowAssignment = null,
  preferredGroupID = 0,
} = {}) {
  if (typeof rowAssignment === 'function') return rowAssignment(row) || null
  const matches = matchingBusinessGroupAssignmentsForRow(row, {
    assignments,
    usageKey,
    objectKey,
    objectIDForRow,
    objectRefForRow,
  })
  const preferredID = toNumber(preferredGroupID)
  if (preferredID > 0) {
    const preferred = matches.find((assignment) => toNumber(assignment.group_id ?? assignment.groupID) === preferredID)
    if (preferred) return preferred
  }
  return matches[0] || null
}

function matchingBusinessGroupAssignmentsForRow(row = {}, {
  assignments = [],
  usageKey = '',
  objectKey = '',
  objectIDForRow = (item = {}) => toNumber(item.id ?? item.product_id ?? item.productID),
  objectRefForRow = null,
} = {}) {
  const normalizedUsage = normalizedText(usageKey)
  const normalizedObjectKey = normalizedText(objectKey)
  const objectID = toNumber(objectIDForRow(row))
  const objectRef = typeof objectRefForRow === 'function' ? normalizedText(objectRefForRow(row)) : ''
  return (Array.isArray(assignments) ? assignments : []).filter((assignment) => {
    if (normalizedUsage && assignmentUsage(assignment) !== normalizedUsage) return false
    if (normalizedObjectKey && assignmentObjectKey(assignment) !== normalizedObjectKey) return false
    if (objectRef) return assignmentObjectRef(assignment) === objectRef
    return objectID > 0 && assignmentObjectID(assignment) === objectID
  })
}

export function groupRowsByBusinessGroupTemplate(rows = [], {
  template = null,
  assignments = [],
  usageKey = '',
  objectKey = '',
  objectIDForRow = (row = {}) => toNumber(row.id ?? row.product_id ?? row.productID),
  objectRefForRow = null,
  rowAssignment = null,
  unclassifiedLabel = '未分类',
  includeUnclassified = true,
} = {}) {
  if (!template || isSystemDefaultBusinessGroup(template)) {
    return [{
      key: 'business-group-unclassified',
      label: unclassifiedLabel,
      path_label: unclassifiedLabel,
      depth: 0,
      group_id: 0,
      group_item_id: 0,
      rows: Array.isArray(rows) ? rows : [],
      all: false,
      unclassified: true,
      sort_order: 999999,
    }]
  }

  const templateID = toNumber(template.id)
  const moveOptions = businessGroupItemMoveOptions([template], usageKey, {
    includeGroupName: false,
    includeGroupsWithoutUsage: true,
  })
  const itemIDs = new Set(moveOptions.map((option) => toNumber(option.group_item_id)).filter(Boolean))
  const groups = moveOptions.map((option, index) => ({
    key: `business-group-${templateID}-${toNumber(option.group_item_id)}`,
    label: option.title_label || option.path_label || option.label,
    path_label: option.path_label || option.label,
    depth: toNumber(option.depth),
    parent_group_item_id: toNumber(option.parent_group_item_id),
    group_id: templateID,
    group_item_id: toNumber(option.group_item_id),
    rows: [],
    all: false,
    unclassified: false,
    sort_order: index,
  }))
  const byItemID = new Map(groups.map((group) => [toNumber(group.group_item_id), group]))
  const unclassified = {
    key: 'business-group-unclassified',
    label: unclassifiedLabel,
    path_label: unclassifiedLabel,
    depth: 0,
    parent_group_item_id: 0,
    group_id: templateID,
    group_item_id: 0,
    rows: [],
    all: false,
    unclassified: true,
    sort_order: 999999,
  }

  for (const row of Array.isArray(rows) ? rows : []) {
    const assignment = findBusinessGroupAssignmentForRow(row, {
      assignments,
      usageKey,
      objectKey,
      objectIDForRow,
      objectRefForRow,
      rowAssignment,
      preferredGroupID: templateID,
    })
    const assignmentGroupID = toNumber(assignment?.group_id ?? assignment?.groupID)
    const assignmentItemID = toNumber(assignment?.group_item_id ?? assignment?.groupItemID)
    const target = assignmentGroupID === templateID && itemIDs.has(assignmentItemID)
      ? byItemID.get(assignmentItemID)
      : unclassified
    target.rows.push(row)
  }

  return includeUnclassified ? [...groups, unclassified] : groups
}

export function groupRowsByBusinessGroupTemplates(rows = [], {
  templates = [],
  assignments = [],
  usageKey = '',
  objectKey = '',
  objectIDForRow = (row = {}) => toNumber(row.id ?? row.product_id ?? row.productID),
  objectRefForRow = null,
  rowAssignment = null,
  allLabel = '全部商品',
  unclassifiedLabel = '未分类',
  includeUnclassified = true,
} = {}) {
  const sourceRows = Array.isArray(rows) ? rows : []
  const activeTemplates = (Array.isArray(templates) ? templates : [])
    .filter((template) => template?.active !== false)
    .filter((template) => !isSystemDefaultBusinessGroup(template))
    .filter((template) => toNumber(template?.id) > 0)

  if (!activeTemplates.length) {
    return [{
      key: 'all-products',
      label: allLabel,
      path_label: allLabel,
      depth: 0,
      parent_group_item_id: 0,
      group_id: 0,
      group_item_id: 0,
      template_label: '',
      rows: sourceRows,
      all: true,
      unclassified: false,
      sort_order: 0,
    }]
  }

  const groups = []
  const groupsByAssignmentKey = new Map()
  const templateIDs = new Set()
  const templateHeaders = []
  for (const template of activeTemplates) {
    const templateID = toNumber(template.id)
    const templateLabel = businessGroupVisibleName(template) || normalizedText(template.name) || `分组模板 #${templateID}`
    templateIDs.add(templateID)
    const templateHeader = {
      key: `business-template-${templateID}`,
      label: templateLabel,
      path_label: templateLabel,
      depth: 0,
      parent_group_item_id: 0,
      group_id: templateID,
      group_item_id: 0,
      template_label: '',
      rows: [],
      all: false,
      unclassified: false,
      is_template_group: true,
      template_total: 0,
      sort_order: groups.length,
    }
    groups.push(templateHeader)
    templateHeaders.push(templateHeader)
    const moveOptions = businessGroupItemMoveOptions([template], usageKey, {
      includeGroupName: false,
      includeGroupsWithoutUsage: true,
    })
    for (const option of moveOptions) {
      const groupItemID = toNumber(option.group_item_id)
      const group = {
        key: `business-group-${templateID}-${groupItemID}`,
        label: option.title_label || option.path_label || option.label,
        path_label: option.path_label || option.label,
        depth: toNumber(option.depth),
        parent_group_item_id: toNumber(option.parent_group_item_id),
        group_id: templateID,
        group_item_id: groupItemID,
        template_label: templateLabel,
        rows: [],
        all: false,
        unclassified: false,
        sort_order: groups.length,
      }
      groups.push(group)
      groupsByAssignmentKey.set(`${templateID}:${groupItemID}`, group)
    }
  }

  const unclassified = {
    key: 'business-group-unclassified',
    label: unclassifiedLabel,
    path_label: unclassifiedLabel,
    depth: 0,
    parent_group_item_id: 0,
    group_id: 0,
    group_item_id: 0,
    template_label: '',
    rows: [],
    all: false,
    unclassified: true,
    sort_order: groups.length,
  }

  for (const row of sourceRows) {
    const matches = typeof rowAssignment === 'function'
      ? [rowAssignment(row)].filter(Boolean)
      : matchingBusinessGroupAssignmentsForRow(row, {
          assignments,
          usageKey,
          objectKey,
          objectIDForRow,
          objectRefForRow,
        })
    const assignment = matches.find((candidate) => templateIDs.has(toNumber(candidate?.group_id ?? candidate?.groupID))) || null
    const assignmentKey = `${toNumber(assignment?.group_id ?? assignment?.groupID)}:${toNumber(assignment?.group_item_id ?? assignment?.groupItemID)}`
    const target = groupsByAssignmentKey.get(assignmentKey) || unclassified
    target.rows.push(row)
  }

  for (const templateHeader of templateHeaders) {
    const templateID = toNumber(templateHeader.group_id)
    templateHeader.template_total = groups
      .filter((group) => !group.is_template_group && toNumber(group.group_id) === templateID)
      .reduce((sum, group) => sum + (Array.isArray(group.rows) ? group.rows.length : 0), 0)
  }

  return includeUnclassified ? [...groups, unclassified] : groups
}

export function businessGroupMoveAssignmentPayload({
  usageKey = '',
  objectKey = '',
  objectID = 0,
  objectRef = '',
  option = null,
  groupID = 0,
  groupItemID = 0,
  sortOrder = 100,
} = {}) {
  return buildBusinessGroupAssignmentPayload({
    usage_key: usageKey,
    object_key: objectKey,
    object_id: objectID,
    object_ref: objectRef,
    group_id: option ? option.group_id : groupID,
    group_item_id: option ? option.group_item_id : groupItemID,
    sort_order: sortOrder,
  })
}

function businessGroupDisplayRowCount(group = {}) {
  if (Object.prototype.hasOwnProperty.call(group, 'total')) {
    const total = Number(group.total)
    if (Number.isFinite(total) && total >= 0) return total
  }
  return Array.isArray(group.rows) ? group.rows.length : 0
}

export function businessGroupCategoryTreeNodes(groups = [], {
  allLabel = '全部分类',
} = {}) {
  const source = Array.isArray(groups) ? groups : []
  const flatListCount = source
    .filter((group) => Boolean(group?.all))
    .reduce((sum, group) => sum + businessGroupDisplayRowCount(group), 0)
  const nodes = [{
    key: 'business-group-all',
    label: normalizedText(allLabel) || '全部分类',
    path_label: normalizedText(allLabel) || '全部分类',
    kind: 'all',
    group_id: 0,
    group_item_id: 0,
    parent_group_item_id: 0,
    parent_key: '',
    tree_depth: 0,
    direct_count: flatListCount,
    count: 0,
    targetable: false,
    expandable: false,
    child_keys: [],
  }]
  const templateKeys = new Map()
  const categoryKeys = new Map()

  for (const group of source) {
    if (!group?.is_template_group) continue
    const groupID = toNumber(group.group_id)
    if (!(groupID > 0)) continue
    const key = normalizedText(group.key) || `business-template-${groupID}`
    templateKeys.set(groupID, key)
    nodes.push({
      ...group,
      key,
      kind: 'template',
      group_id: groupID,
      group_item_id: 0,
      parent_group_item_id: 0,
      parent_key: 'business-group-all',
      tree_depth: 1,
      direct_count: 0,
      count: 0,
      targetable: false,
      expandable: false,
      child_keys: [],
    })
  }

  for (const group of source) {
    if (group?.is_template_group || group?.all || group?.unclassified) continue
    const groupID = toNumber(group?.group_id)
    const groupItemID = toNumber(group?.group_item_id)
    if (!(groupItemID > 0)) continue
    const key = normalizedText(group.key) || `business-group-${groupID}-${groupItemID}`
    categoryKeys.set(`${groupID}:${groupItemID}`, key)
  }

  for (const group of source) {
    if (group?.is_template_group || group?.all) continue
    const unclassified = Boolean(group?.unclassified)
    const groupID = toNumber(group?.group_id)
    const groupItemID = toNumber(group?.group_item_id)
    if (!unclassified && !(groupItemID > 0)) continue
    const parentItemID = toNumber(group?.parent_group_item_id)
    const key = normalizedText(group.key) || (unclassified
      ? 'business-group-unclassified'
      : `business-group-${groupID}-${groupItemID}`)
    const parentKey = unclassified
      ? 'business-group-all'
      : (categoryKeys.get(`${groupID}:${parentItemID}`) || templateKeys.get(groupID) || 'business-group-all')
    nodes.push({
      ...group,
      key,
      kind: unclassified ? 'unclassified' : 'category',
      group_id: groupID,
      group_item_id: groupItemID,
      parent_group_item_id: parentItemID,
      parent_key: parentKey,
      tree_depth: unclassified ? 1 : Math.max(1, toNumber(group.depth) + (templateKeys.has(groupID) ? 2 : 1)),
      direct_count: businessGroupDisplayRowCount(group),
      count: 0,
      targetable: true,
      expandable: false,
      child_keys: [],
    })
  }

  const byKey = new Map(nodes.map((node) => [node.key, node]))
  for (const node of nodes) {
    if (!node.parent_key) continue
    const parent = byKey.get(node.parent_key)
    if (parent) parent.child_keys.push(node.key)
  }
  for (const node of nodes) node.expandable = node.child_keys.length > 0

  const countFor = (node, visiting = new Set()) => {
    if (!node || visiting.has(node.key)) return 0
    const nextVisiting = new Set(visiting)
    nextVisiting.add(node.key)
    const total = toNumber(node.direct_count) + node.child_keys.reduce((sum, key) => sum + countFor(byKey.get(key), nextVisiting), 0)
    node.count = total
    return total
  }
  for (const node of nodes) countFor(node)

  const ordered = []
  const emitted = new Set()
  const appendPreorder = (node) => {
    if (!node || emitted.has(node.key)) return
    emitted.add(node.key)
    ordered.push(node)
    for (const childKey of node.child_keys) appendPreorder(byKey.get(childKey))
  }
  appendPreorder(nodes[0])
  for (const node of nodes) appendPreorder(node)
  return ordered
}

export function businessGroupCategoryBreadcrumb(nodes = [], selectedKey = 'business-group-all') {
  const source = Array.isArray(nodes) ? nodes : []
  const byKey = new Map(source.map((node) => [node.key, node]))
  let current = byKey.get(normalizedText(selectedKey)) || byKey.get('business-group-all') || source[0] || null
  const labels = []
  const visited = new Set()
  while (current && !visited.has(current.key)) {
    visited.add(current.key)
    labels.unshift(normalizedText(current.label) || normalizedText(current.path_label) || current.key)
    current = current.parent_key ? byKey.get(current.parent_key) : null
  }
  return labels.filter(Boolean).join(' / ')
}

export function businessGroupGroupsForCategorySelection(groups = [], selectedKey = 'business-group-all') {
  const source = Array.isArray(groups) ? groups : []
  const normalizedKey = normalizedText(selectedKey)
  if (!normalizedKey || normalizedKey === 'business-group-all') return source
  const selected = source.find((group) => normalizedText(group?.key) === normalizedKey)
  if (!selected) return source
  if (selected.unclassified) return source.filter((group) => Boolean(group?.unclassified))
  const selectedGroupID = toNumber(selected.group_id)
  if (selected.is_template_group) {
    return source.filter((group) => toNumber(group?.group_id) === selectedGroupID && !group?.unclassified)
  }
  const selectedItemID = toNumber(selected.group_item_id)
  if (!(selectedItemID > 0)) return source
  const descendantIDs = new Set([selectedItemID])
  let changed = true
  while (changed) {
    changed = false
    for (const group of source) {
      if (toNumber(group?.group_id) !== selectedGroupID) continue
      const itemID = toNumber(group?.group_item_id)
      const parentID = toNumber(group?.parent_group_item_id)
      if (!(itemID > 0) || !descendantIDs.has(parentID) || descendantIDs.has(itemID)) continue
      descendantIDs.add(itemID)
      changed = true
    }
  }
  return source.filter((group) => (
    toNumber(group?.group_id) === selectedGroupID
    && descendantIDs.has(toNumber(group?.group_item_id))
    && !group?.is_template_group
  ))
}

export function beginBusinessGroupMoveState(state = {}, nodes = []) {
  const expandedKeys = Array.from(new Set((Array.isArray(state.expandedKeys) ? state.expandedKeys : []).map(normalizedText).filter(Boolean)))
  const snapshot = {
    expandedKeys,
    selectedKey: normalizedText(state.selectedKey) || 'business-group-all',
    scrollTop: Math.max(0, Number(state.scrollTop || 0)),
  }
  return {
    active: true,
    expandedKeys: (Array.isArray(nodes) ? nodes : []).filter((node) => node?.expandable).map((node) => node.key),
    selectedKey: snapshot.selectedKey,
    scrollTop: 0,
    snapshot,
  }
}

export function restoreBusinessGroupMoveState(state = {}) {
  const snapshot = state?.snapshot || {}
  return {
    active: false,
    expandedKeys: Array.from(new Set((Array.isArray(snapshot.expandedKeys) ? snapshot.expandedKeys : []).map(normalizedText).filter(Boolean))),
    selectedKey: normalizedText(snapshot.selectedKey) || 'business-group-all',
    scrollTop: Math.max(0, Number(snapshot.scrollTop || 0)),
    snapshot: null,
  }
}

export function businessGroupInlineListState(groups = [], paginationByGroup = {}, options = {}) {
  const sourceGroups = Array.isArray(groups) ? groups : []
  const sourcePagination = paginationByGroup && typeof paginationByGroup === 'object'
    ? paginationByGroup
    : {}
  const defaultPageSize = normalizePageSize(options.defaultPageSize)
  const paginationThreshold = Math.max(0, toNumber(options.paginationThreshold ?? 10))
  const pagination = {}
  const visibleRows = []
  let total = 0

  const paginatedGroups = sourceGroups.map((group, index) => {
    const key = normalizedText(group?.key) || `business-group-inline-${index}`
    if (group?.is_template_group) {
      return {
        ...group,
        key,
        rows: [],
        total: Math.max(0, toNumber(group?.template_total)),
        page: 1,
        pageSize: defaultPageSize,
        needsPagination: false,
      }
    }
    const sourceRows = Array.isArray(group?.rows) ? group.rows : []
    const requested = sourcePagination[key] || {}
    const pageSize = normalizePageSize(requested.pageSize || defaultPageSize)
    const needsPagination = sourceRows.length > paginationThreshold
    const page = needsPagination ? clampPage(requested.page, sourceRows.length, pageSize) : 1
    const rows = needsPagination ? slicePageRows(sourceRows, { page, pageSize }) : sourceRows
    pagination[key] = { page, pageSize }
    visibleRows.push(...rows)
    total += sourceRows.length
    return {
      ...group,
      key,
      total: sourceRows.length,
      page,
      pageSize,
      needsPagination,
      rows,
    }
  })

  return { groups: paginatedGroups, pagination, visibleRows, total }
}

export function businessGroupSearchCollapsedKeys(groups = []) {
  const source = Array.isArray(groups) ? groups : []
  const templateKeys = new Map()
  const categoryKeys = new Map()
  const byKey = new Map()
  const childKeys = new Map()

  for (const group of source) {
    const key = normalizedText(group?.key)
    if (!key) continue
    byKey.set(key, group)
    if (group?.is_template_group && toNumber(group?.group_id) > 0) {
      templateKeys.set(toNumber(group.group_id), key)
    }
    if (!group?.is_template_group && !group?.all && !group?.unclassified && toNumber(group?.group_item_id) > 0) {
      categoryKeys.set(`${toNumber(group?.group_id)}:${toNumber(group?.group_item_id)}`, key)
    }
  }

  for (const [key, group] of byKey) {
    if (group?.is_template_group || group?.all || group?.unclassified) continue
    const groupID = toNumber(group?.group_id)
    const parentItemID = toNumber(group?.parent_group_item_id)
    const parentKey = parentItemID > 0
      ? categoryKeys.get(`${groupID}:${parentItemID}`)
      : templateKeys.get(groupID)
    if (!parentKey) continue
    const children = childKeys.get(parentKey) || []
    children.push(key)
    childKeys.set(parentKey, children)
  }

  const subtreeCount = (key, visiting = new Set()) => {
    if (!key || visiting.has(key)) return 0
    const group = byKey.get(key)
    if (!group) return 0
    const nextVisiting = new Set(visiting)
    nextVisiting.add(key)
    const directCount = group?.is_template_group ? 0 : businessGroupDisplayRowCount(group)
    return directCount + (childKeys.get(key) || []).reduce((sum, childKey) => (
      sum + subtreeCount(childKey, nextVisiting)
    ), 0)
  }

  return source
    .map((group) => normalizedText(group?.key))
    .filter((key) => key && subtreeCount(key) === 0)
}

export function businessGroupMoveCollapsedKeys(groups = []) {
  return Array.from(new Set((Array.isArray(groups) ? groups : [])
    .map((group) => normalizedText(group?.key))
    .filter(Boolean)))
}

export function businessGroupVisibleGroups(groups = [], collapsedGroupKeys = [], { showAllHeadings = false } = {}) {
  const source = Array.isArray(groups) ? groups : []
  if (showAllHeadings) return source
  return source.filter((group) => !businessGroupHiddenByCollapsedAncestor(source, group, collapsedGroupKeys))
}

export function businessGroupHiddenByCollapsedAncestor(groups = [], group = {}, collapsedGroupKeys = []) {
  const source = Array.isArray(groups) ? groups : []
  const collapsed = new Set((Array.isArray(collapsedGroupKeys) ? collapsedGroupKeys : [])
    .map(normalizedText)
    .filter(Boolean))
  const groupID = toNumber(group?.group_id)
  if (!collapsed.size || !(groupID > 0)) return false
  if (!group?.is_template_group) {
    const templateHeader = source.find((candidate) => (
      candidate?.is_template_group && toNumber(candidate?.group_id) === groupID
    ))
    if (templateHeader && collapsed.has(normalizedText(templateHeader.key))) return true
  }
  let parentItemID = toNumber(group?.parent_group_item_id)
  const byItemID = new Map(source
    .filter((candidate) => toNumber(candidate?.group_id) === groupID && toNumber(candidate?.group_item_id) > 0)
    .map((candidate) => [toNumber(candidate.group_item_id), candidate]))
  const visited = new Set()
  while (parentItemID > 0 && !visited.has(parentItemID)) {
    visited.add(parentItemID)
    const parent = byItemID.get(parentItemID)
    if (!parent) break
    if (collapsed.has(normalizedText(parent.key))) return true
    parentItemID = toNumber(parent.parent_group_item_id)
  }
  return false
}

export function businessGroupVisibleRows(groups = [], collapsedGroupKeys = []) {
  const source = Array.isArray(groups) ? groups : []
  const collapsed = new Set((Array.isArray(collapsedGroupKeys) ? collapsedGroupKeys : [])
    .map(normalizedText)
    .filter(Boolean))
  return source.flatMap((group) => (
    group?.is_template_group
      || collapsed.has(normalizedText(group?.key))
      || businessGroupHiddenByCollapsedAncestor(source, group, collapsedGroupKeys)
      || !Array.isArray(group?.rows)
      ? []
      : group.rows
  ))
}

export function businessGroupHeaderIndentStyle(group = {}) {
  return { '--classification-group-indent': `${16 + toNumber(group.depth) * 24}px` }
}

export function businessGroupItemIndentStyle(group = {}) {
  return { '--classification-item-indent': `${18 + toNumber(group.depth) * 24}px` }
}
