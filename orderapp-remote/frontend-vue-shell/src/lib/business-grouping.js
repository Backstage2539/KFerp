import {
  businessGroupItemMoveOptions,
  businessGroupVisibleName,
  buildBusinessGroupAssignmentPayload,
  isSystemDefaultBusinessGroup,
} from './product-settings.js'

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
  for (const template of activeTemplates) {
    const templateID = toNumber(template.id)
    const templateLabel = businessGroupVisibleName(template) || normalizedText(template.name) || `分组模板 #${templateID}`
    templateIDs.add(templateID)
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

export function businessGroupHeaderIndentStyle(group = {}) {
  return { '--classification-group-indent': `${16 + toNumber(group.depth) * 24}px` }
}

export function businessGroupItemIndentStyle(group = {}) {
  return { '--classification-item-indent': `${18 + toNumber(group.depth) * 24}px` }
}
