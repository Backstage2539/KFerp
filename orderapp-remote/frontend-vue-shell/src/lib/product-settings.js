import { clampPage, normalizePageSize, slicePageRows } from './pagination.js'

export const PRODUCT_KIND_ALL = 'all'
export const SKU_CUSTOM_TYPE_ALL = 'all'

export const skuTypeOptions = [
  { value: SKU_CUSTOM_TYPE_ALL, label: '全部类型' },
  { value: 'standard', label: '标准' },
  { value: 'public_sku_alias', label: '公共 SKU 改名' },
  { value: 'custom_roast', label: '定制烘焙' },
  { value: 'custom_blend', label: '定制拼配' },
]

export const priceListRulePricingModeOptions = [
  { value: 'inherit_gradient_template', label: '按阶梯价模板' },
  { value: 'fixed_unit_price', label: '固定单价' },
  { value: 'cost_plus', label: '成本加成' },
]

export const priceListRuleRoundingOptions = [
  { value: 'none', label: '不取整' },
  { value: 'jiao', label: '保留到角' },
  { value: 'yuan', label: '保留到元' },
]

export function rowIsDeleted(row = {}) {
  if (row?.deleted === true) return true
  if (row?.deleted_at || row?.deletedAt) return true
  const state = String(row?.template_state || row?.templateState || '').trim().toLowerCase()
  return state === 'deleted'
}

export function visibleNonDeletedRows(rows = []) {
  return (Array.isArray(rows) ? rows : []).filter((row) => !rowIsDeleted(row))
}

export const integerUnitModeOptions = [
  { value: 'inherit', label: '继承子类型' },
  { value: 'integer', label: '只允许整数' },
  { value: 'decimal', label: '允许小数' },
]

export function normalizedProductKind(row = {}) {
  const kind = String(row?.product_kind || '').trim()
  if (kind === 'green_bean') return 'green_bean'
  if (kind === 'drip_bag') return 'drip_bag'
  if (kind === 'instant_coffee' || kind === 'instant') return 'instant_coffee'
  return 'roasted'
}

export function inferProductKindFromProductTypeCategory(category = {}) {
  const text = `${category?.name || ''} ${category?.source_name || ''}`.trim().toLowerCase()
  if (!text) return 'roasted'
  if (text.includes('速溶') || text.includes('冻干') || text.includes('instant')) return 'instant_coffee'
  if (text.includes('挂耳') || text.includes('drip')) return 'drip_bag'
  if (text.includes('生豆') || text.includes('green')) return 'green_bean'
  return 'roasted'
}

export function productKindRequiresRoast(kindOrRow = {}) {
  const kind = typeof kindOrRow === 'object' ? normalizedProductKind(kindOrRow) : normalizedProductKind({ product_kind: kindOrRow })
  return kind !== 'green_bean'
}

export function productKindSupportsBomParams(kindOrRow = {}) {
  const kind = typeof kindOrRow === 'object' ? normalizedProductKind(kindOrRow) : normalizedProductKind({ product_kind: kindOrRow })
  return kind !== 'green_bean'
}

export function skuTypeValue(row = {}) {
  return String(row?.custom_type || '').trim() || 'standard'
}

export function skuTypeLabel(value) {
  const normalized = String(value || '').trim() || 'standard'
  return skuTypeOptions.find((item) => item.value === normalized)?.label || '标准'
}

export function filterSkuRows(rows = [], filters = {}) {
  const productKind = String(filters.productKind || PRODUCT_KIND_ALL).trim()
  const customType = String(filters.customType || SKU_CUSTOM_TYPE_ALL).trim()
  const query = String(filters.query || '').trim().toLowerCase()
  const primaryCategory = String(filters.primaryCategory || '').trim()
  const secondaryCategory = String(filters.secondaryCategory || '').trim()
  const active = String(filters.active || 'all').trim()
  return (rows || []).filter((row) => {
    if (productKind && productKind !== PRODUCT_KIND_ALL && normalizedProductKind(row) !== productKind) return false
    if (customType && customType !== SKU_CUSTOM_TYPE_ALL && skuTypeValue(row) !== customType) return false
    if (query) {
      const haystack = `${row.name || ''} ${row.number || ''} ${skuTypeLabel(row.custom_type)} ${row.remark || ''}`.toLowerCase()
      if (!haystack.includes(query)) return false
    }
    if (primaryCategory && String(row.primary_name || '') !== primaryCategory) return false
    if (secondaryCategory && String(row.secondary_name || '') !== secondaryCategory) return false
    if (active === 'active' && row.active === false) return false
    if (active === 'inactive' && row.active !== false) return false
    return true
  })
}

export function normalizeVisibleSkuFilters(filters = {}, rows = null) {
  const normalized = {
    productKind: PRODUCT_KIND_ALL,
    customType: SKU_CUSTOM_TYPE_ALL,
    active: String(filters.active || 'active').trim(),
    query: String(filters.query || '').trim(),
    primaryCategory: String(filters.primaryCategory || '').trim(),
    secondaryCategory: String(filters.secondaryCategory || '').trim(),
  }
  if (Array.isArray(rows)) {
    const primaryOptions = primaryCategoryOptions(rows)
    if (normalized.primaryCategory && !primaryOptions.includes(normalized.primaryCategory)) {
      normalized.primaryCategory = ''
    }
    const secondaryOptions = secondaryCategoryOptions(rows, normalized.primaryCategory)
    if (normalized.secondaryCategory && !secondaryOptions.includes(normalized.secondaryCategory)) {
      normalized.secondaryCategory = ''
    }
  }
  return normalized
}

export function paginatedSkuRows(rows = [], filters = {}, pagination = {}) {
  return slicePageRows(filterSkuRows(rows, filters), pagination)
}

export function skuTableState(rows = [], filters = {}, pagination = {}) {
  const sourceRows = Array.isArray(rows) ? rows : []
  const normalizedFilters = normalizeVisibleSkuFilters(filters, sourceRows)
  const filteredRows = filterSkuRows(sourceRows, normalizedFilters)
  const pageSize = normalizePageSize(pagination.pageSize)
  const page = clampPage(pagination.page, filteredRows.length, pageSize)
  const start = (page - 1) * pageSize
  return {
    filters: normalizedFilters,
    primaryOptions: primaryCategoryOptions(sourceRows),
    secondaryOptions: secondaryCategoryOptions(sourceRows, normalizedFilters.primaryCategory),
    total: filteredRows.length,
    page,
    pageSize,
    rows: filteredRows.slice(start, start + pageSize),
  }
}

export function skuListRowsFromProducts(products = [], categoryTree = [], filterFn = () => true) {
  const categoryMetaByProductID = categoryProductMetaByID(categoryTree)
  const categoryMetaByCategoryID = categoryPathMetaByID(categoryTree)
  return (products || [])
    .filter((product) => {
      try {
        return filterFn(product)
      } catch (_) {
        return false
      }
    })
    .map((product) => ({
      ...product,
      ...(categoryMetaByProductID.get(Number(product?.id || 0))
        || categoryMetaByCategoryID.get(Number(product?.product_category_id || 0))
        || {}),
    }))
}

export function customerSkuCustomerOptions(customers = []) {
  const rows = Array.isArray(customers)
    ? customers
    : Array.isArray(customers?.customers)
      ? customers.customers
      : Array.isArray(customers?.rows)
        ? customers.rows
        : []
  return rows
    .filter((customer) => Number(customer?.id || 0) > 0 && customer?.active !== false)
    .slice()
    .sort((a, b) => String(a?.name || '').localeCompare(String(b?.name || '')))
}

export function buildCustomerProductAliasPayload(form = {}) {
  return {
    id: Number(form.id || 0),
    customer_id: Number(form.customer_id || form.customerID || 0),
    product_id: Number(form.product_id || form.productID || 0),
    display_name: String(form.display_name ?? form.displayName ?? '').trim(),
    brand_name: String(form.brand_name ?? form.brandName ?? '').trim(),
    display_category_id: Number(form.display_category_id || form.displayCategoryID || 0),
    product_config_template_id: Number(form.product_config_template_id || form.productConfigTemplateID || 0),
    sort_order: Number(form.sort_order || form.sortOrder || 0),
    include_in_price_list: Boolean(form.include_in_price_list ?? form.includeInPriceList ?? true),
    active: Boolean(form.active ?? true),
    remark: String(form.remark ?? '').trim(),
  }
}

export function customerAliasEffectiveDisplayName(alias = {}) {
  const renamed = String(alias.brand_name ?? alias.brandName ?? '').trim()
  if (renamed) return renamed
  return String(alias.display_name ?? alias.displayName ?? '').trim()
}

export function productCodeLabel(row = {}) {
  const explicit = String(row.product_code ?? row.productCode ?? row.code ?? '').trim()
  if (explicit) return explicit
  const id = Number(row.id ?? row.product_id ?? row.productID ?? 0)
  return id > 0 ? `SKU-${String(id).padStart(6, '0')}` : ''
}

export function activeProductionBomOptions(rows = []) {
  return (Array.isArray(rows) ? rows : [])
    .filter((row) => Number(row?.id || 0) > 0)
    .filter((row) => String(row?.status || 'active').trim().toLowerCase() === 'active')
    .map((row) => ({
      ...row,
      id: Number(row.id || 0),
      code: String(row.code || '').trim(),
      name: String(row.name || '').trim(),
      latest_version_no: String(row.latest_version_no || row.production_bom_version_no || '').trim(),
      latest_version_status: String(row.latest_version_status || '').trim(),
      group_name: String(row.group_name || '').trim(),
    }))
    .sort((a, b) => String(a.code || '').localeCompare(String(b.code || '')) || String(a.name || '').localeCompare(String(b.name || '')))
}

export function productionBomOptionLabel(row = {}) {
  const code = String(row.code || '').trim()
  const name = String(row.name || '').trim()
  const version = String(row.latest_version_no || row.production_bom_version_no || '').trim()
  return [code, name].filter(Boolean).join(' ') + (version ? ` / ${version}` : '')
}

export function buildCustomerProductAliasBatchPayload(form = {}) {
  const ids = []
  const seen = new Set()
  for (const raw of form.product_ids || form.productIDs || []) {
    const id = Number(raw || 0)
    if (!id || seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  return {
    customer_id: Number(form.customer_id || form.customerID || 0),
    product_ids: ids,
    include_in_price_list: Boolean(form.include_in_price_list ?? form.includeInPriceList ?? true),
    brand_name: String(form.brand_name ?? form.brandName ?? '').trim(),
    display_category_id: Number(form.display_category_id || form.displayCategoryID || 0),
  }
}

export function buildClassificationTemplateUsagePayload(form = {}) {
  const payload = {
    classification_template_id: Number(form.classification_template_id || form.classificationTemplateID || form.template_id || form.templateID || 0),
    sort_order: Number(form.sort_order || form.sortOrder || 100),
  }
  const customerID = Number(form.customer_id || form.customerID || 0)
  if (customerID > 0) payload.customer_id = customerID
  return payload
}

export function classificationTemplateTabs(templates = [], usages = [], options = {}) {
  const activeTemplateByID = new Map((templates || [])
    .filter((template) => template?.active !== false)
    .map((template) => [Number(template.id || 0), template]))
  const seen = new Set()
  const tabs = [{
    key: 'all',
    id: 0,
    template_id: 0,
    label: String(options.allLabel || '全部商品'),
    all: true,
  }]
  if (options.unclassifiedLabel) {
    tabs.push({
      key: 'unclassified',
      id: -1,
      template_id: 0,
      label: String(options.unclassifiedLabel),
      unclassified: true,
      all: false,
      sort_order: -1,
    })
  }
  for (const usage of usages || []) {
    if (usage?.active === false) continue
    const templateID = Number(usage.classification_template_id || usage.template_id || 0)
    const template = activeTemplateByID.get(templateID)
    if (!template || seen.has(templateID)) continue
    seen.add(templateID)
    tabs.push({
      key: `template-${templateID}`,
      id: templateID,
      template_id: templateID,
      label: template.name || `分类模板 #${templateID}`,
      sort_order: Number(usage.sort_order || template.sort_order || 100),
      template,
      all: false,
    })
  }
  const fixedTabs = tabs.filter((tab) => tab.all || tab.unclassified)
  return fixedTabs.concat(tabs.filter((tab) => !tab.all && !tab.unclassified)
    .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || String(a.label || '').localeCompare(String(b.label || ''))))
}

export function groupRowsByClassificationCategory(rows = [], template = {}, options = {}) {
  const idKey = options.idKey || 'id'
  const assignmentKey = options.assignmentKey || 'product_id'
  const assignmentsKey = options.assignmentsKey || 'product_assignments'
  const assignmentsByObjectID = new Map((template?.[assignmentsKey] || [])
    .filter((assignment) => Number(assignment.template_id || template.id || 0) === Number(template.id || 0))
    .map((assignment) => [Number(assignment[assignmentKey] || 0), assignment]))
  const categories = (template?.categories || [])
    .filter((category) => category?.active !== false)
    .slice()
    .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0))
  const groups = categories.map((category) => ({
    key: `category-${Number(category.id || 0)}`,
    id: Number(category.id || 0),
    label: category.name || '未命名分类',
    rows: [],
    category,
  }))
  const groupByCategoryID = new Map(groups.map((group) => [group.id, group]))
  const uncategorized = { key: 'uncategorized', id: 0, label: '未分类', rows: [], category: null }
  const onlyAssigned = Boolean(options.onlyAssigned)
  for (const row of rows || []) {
    const objectID = Number(row?.[idKey] || 0)
    const assignment = assignmentsByObjectID.get(objectID)
    if (onlyAssigned && !assignment) continue
    const categoryID = Number(assignment?.category_id || 0)
    const target = groupByCategoryID.get(categoryID) || uncategorized
    target.rows.push({
      ...row,
      classification_category_id: target.id,
      classification_sort_order: Number(assignment?.sort_order || row?.sort_order || 100),
    })
  }
  for (const group of [...groups, uncategorized]) {
    group.rows.sort((a, b) => Number(a.classification_sort_order || 0) - Number(b.classification_sort_order || 0) || Number(a.id || 0) - Number(b.id || 0))
  }
  return [...groups, uncategorized]
}

export function classificationAssignmentForRow(row = {}, templates = [], options = {}) {
  const assignmentType = String(options.assignmentType || 'product')
  const rowID = Number(row?.[options.idKey || 'id'] || row?.product_id || row?.alias_id || 0)
  if (!rowID) return null
  const assignmentsKey = options.assignmentsKey || (assignmentType === 'alias' ? 'customer_alias_assignments' : 'product_assignments')
  const assignmentIDKey = options.assignmentKey || (assignmentType === 'alias' ? 'alias_id' : 'product_id')
  for (const template of templates || []) {
    const templateID = Number(template?.id || template?.template_id || 0)
    if (!templateID || template?.active === false) continue
    const assignment = (template?.[assignmentsKey] || []).find((item) => Number(item?.[assignmentIDKey] || 0) === rowID)
    if (!assignment) continue
    const categoryID = Number(assignment.category_id || 0)
    const category = (template.categories || []).find((item) => Number(item.id || 0) === categoryID)
    return { assignment, template, category }
  }
  return null
}

export function classificationAssignmentLabel(row = {}, templates = [], options = {}) {
  const found = classificationAssignmentForRow(row, templates, options)
  if (!found) return '未分类'
  const templateName = found.template?.name || `分类模板 #${Number(found.template?.id || 0)}`
  const categoryName = found.category?.name || '未分类'
  return `${templateName} / ${categoryName}`
}

export function classificationAssignmentConflict(row = {}, targetTemplateID = 0, templates = [], options = {}) {
  return null
}

export function classificationTemplateUnitPriceWarnings(input = {}) {
  const productConfigTemplate = input.productConfigTemplate || input.product_config_template || {}
  const classificationTemplate = input.classificationTemplate || input.classification_template || {}
  const classificationCategory = input.classificationCategory || input.classification_category || {}
  const effectiveProductConfigID = Number(classificationCategory.product_config_template_id || classificationTemplate.product_config_template_id || 0)
  const productConfigID = Number(productConfigTemplate.id || productConfigTemplate.product_config_template_id || 0)
  const warnings = []
  if (productConfigID > 0 && effectiveProductConfigID > 0 && productConfigID !== effectiveProductConfigID) {
    warnings.push('商品已选择商品配置模板，将覆盖所属分类引用的商品配置模板')
  }
  return warnings
}

export function customerProductAliasRowsForCustomer(rows = [], customerID = 0, options = {}) {
  const selectedCustomerID = Number(customerID || 0)
  const active = String(options.active || (options.includeInactive ? 'all' : 'active')).trim()
  const query = String(options.query || '').trim().toLowerCase()
  return (rows || [])
    .filter((row) => {
      if (selectedCustomerID > 0 && Number(row?.customer_id || 0) !== selectedCustomerID) return false
      if (active === 'active' && row?.active === false) return false
      if (active === 'inactive' && row?.active !== false) return false
      if (query) {
        const haystack = [
          row?.display_name,
          row?.customer_item_code,
          row?.brand_name,
          row?.product_code,
          row?.product_name,
        ].join(' ').toLowerCase()
        if (!haystack.includes(query)) return false
      }
      return true
    })
    .slice()
    .sort((a, b) => Number(a?.sort_order || 0) - Number(b?.sort_order || 0) || Number(a?.id || 0) - Number(b?.id || 0))
}

export function industryFieldOptionsTextFromJSON(raw = '[]') {
  if (Array.isArray(raw)) return raw.map((value) => String(value || '').trim()).filter(Boolean).join(', ')
  try {
    const parsed = JSON.parse(String(raw || '[]'))
    if (Array.isArray(parsed)) return parsed.map((value) => String(value || '').trim()).filter(Boolean).join(', ')
  } catch (_) {
    return String(raw || '').trim()
  }
  return ''
}

export function industryFieldOptionsJSONFromText(raw = '') {
  const seen = new Set()
  const values = String(raw || '')
    .split(/[,，]/)
    .map((value) => value.trim())
    .filter((value) => {
      if (!value || seen.has(value)) return false
      seen.add(value)
      return true
    })
  return JSON.stringify(values)
}

export function industryFieldSummary(fields = []) {
  const parts = (fields || [])
    .filter((field) => String(field?.field_key || field?.label || '').trim())
    .map((field) => {
      const label = String(field?.label || field?.field_key || '').trim()
      const value = String(field?.value_text ?? field?.value ?? '').trim()
      return value ? `${label}：${value}` : ''
    })
    .filter(Boolean)
  return parts.length ? parts.join('；') : '-'
}

export function buildCustomerProductAliasIndustryFieldPayload(form = {}) {
  const fields = Array.isArray(form?.fields) ? form.fields : []
  const seen = new Set()
  return {
    fields: fields
      .map((field) => ({
        field_key: String(field?.field_key || '').trim(),
        value_text: String(field?.value_text ?? field?.value ?? '').trim(),
      }))
      .filter((field) => {
        const key = field.field_key.toLowerCase()
        if (!key || seen.has(key)) return false
        seen.add(key)
        return true
      }),
  }
}

export function productCreationActionOptions(context = {}) {
  return [{
    key: 'product_record',
    label: '创建新商品档案',
    description: '配方、包装、生产方式、库存对象或成本口径变化时使用，后续维护独立生产 BOM。',
  }]
}

export function customerProductAliasMigrationCandidateSummary(row = {}) {
  const product = [row.product_code, row.product_name].map((value) => String(value || '').trim()).filter(Boolean).join(' ')
  const base = [row.base_product_code, row.base_product_name].map((value) => String(value || '').trim()).filter(Boolean).join(' ')
  const reason = String(row.suggested_reason || '').trim()
  if (row.suggested_action === 'convert_to_customer_product_alias') {
    return `建议转为客户商品：${product || '当前客户商品'} → 绑定 ${base || '来源商品档案'}${reason ? `；${reason}` : ''}`
  }
  return `建议保留商品档案：${product || '当前客户商品'}${reason ? `；${reason}` : ''}`
}

export function buildCustomerPublicUsagePayload(customerID, options = {}) {
  return {
    customer_id: Number(customerID || 0),
    use_public_sku: Boolean(options.use_public_sku ?? options.usePublicSku),
    use_public_categories: Boolean(options.use_public_categories ?? options.usePublicCategories),
    use_public_gradient_templates: Boolean(options.use_public_gradient_templates ?? options.usePublicGradientTemplates),
  }
}

export function productBelongsToSkuContext(product = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const productCustomerID = Number(product.customer_id || 0)
  if (!customerID) return productCustomerID === 0
  if (productCustomerID === customerID) return true
  if (productCustomerID !== 0 || !Boolean(context.usePublicSku || context.use_public_sku)) return false
  return !hasCustomerDerivedProduct(product, context.customerProducts)
}

export function categoryBelongsToSkuContext(category = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const categoryCustomerID = Number(category.customer_id || 0)
  if (!customerID) return categoryCustomerID === 0
  if (categoryCustomerID === customerID) return true
  if (categoryCustomerID !== 0 || !Boolean(context.usePublicCategories || context.use_public_categories)) return false
  return !hasCustomerDerivedCategory(category, context.customerCategories)
}

export function nextSkuContextCustomerID(currentCustomerID = 0, { workspaceMode = '', customerContextID = 0, customerContextId = 0 } = {}) {
  if (String(workspaceMode || '').trim() !== 'customer') return 0
  const lockedCustomerID = Number(customerContextID || customerContextId || 0)
  if (lockedCustomerID > 0) return lockedCustomerID
  return Number(currentCustomerID || 0)
}

export function gradientTemplateBelongsToSkuContext(template = {}, context = {}) {
	const customerID = Number(context.customerID || context.customer_id || 0)
	const templateCustomerID = Number(template.customer_id || 0)
	if (!customerID) return templateCustomerID === 0
  if (templateCustomerID === customerID) return true
  if (templateCustomerID !== 0 || !Boolean(context.usePublicGradientTemplates || context.use_public_gradient_templates)) return false
	return !hasCustomerDerivedTemplate(template, context.customerTemplates)
}

export function productConfigTemplateBelongsToSkuContext(template = {}, context = {}) {
	const customerID = Number(context.customerID || context.customer_id || 0)
	const templateCustomerID = Number(template.customer_id || 0)
	if (!customerID) return templateCustomerID === 0
	if (templateCustomerID === customerID) return true
	if (templateCustomerID !== 0) return false
	if (context.usePublicProductConfigTemplates === false || context.use_public_product_config_templates === false) return false
	return !hasCustomerDerivedTemplate(template, context.customerTemplates)
}

export function categoryDisplayState(category = {}, context = {}) {
  if (Number(category.customer_id || 0) === 0 && Number(context.customerID || context.customer_id || 0) > 0) {
    return { label: '公共模板', tone: 'template' }
  }
  if (Number(category.source_category_id || 0) > 0 || category.template_state === 'derived_from_public') {
    return { label: '来自公共模板', tone: 'derived' }
  }
  return { label: '客户自有', tone: 'owned' }
}

export function productDisplayState(product = {}, context = {}) {
  if (Number(product.customer_id || 0) === 0 && Number(context.customerID || context.customer_id || 0) > 0) {
    return { label: '公共模板', tone: 'template' }
  }
  if (Number(product.base_product_id || 0) > 0 && product.custom_type === 'public_sku_alias') {
    return { label: '来自公共 SKU', tone: 'derived' }
  }
  return { label: '客户自有', tone: 'owned' }
}

export function buildAssignCategoryPayload({ product = {}, category = {}, customerID = 0, position = 0 } = {}) {
  const scopedCustomerID = Number(customerID || 0)
  const productCustomerID = Number(product.customer_id || 0)
  const categoryCustomerID = Number(category.customer_id || 0)
  const payload = {
    category_id: Number(category.id || 0),
    position: Number(position || 0),
  }
  if (scopedCustomerID > 0) {
    payload.customer_id = scopedCustomerID
    payload.derive_public_category = Number(category.id || 0) > 0 && categoryCustomerID === 0
    payload.derive_public_product = productCustomerID === 0
  }
  return payload
}

export function buildProductCategoryConfigPayload(category = {}) {
	return {
		id: Number(category.id || 0),
		customer_id: Number(category.customer_id || 0),
		name: String(category.name || '').trim(),
		parent_id: Number(category.parent_id || 0),
		position: Number(category.position || 0),
		product_config_template_id: Number(category.product_config_template_id || 0),
		gradient_template_id: Number(category.gradient_template_id || 0),
		operation_template_id: Number(category.operation_template_id || 0),
    price_list_rule_json: hasStructuredPriceRuleFields(category)
      ? priceListRuleJSONFromForm(category)
      : normalizeJSONString(category.price_list_rule_json),
    inventory_unit: normalizeUnitText(category.inventory_unit, 'kg'),
    quote_unit: normalizeUnitText(category.quote_unit, normalizeUnitText(category.inventory_unit, 'kg')),
    order_unit: normalizeUnitText(category.order_unit, normalizeUnitText(category.quote_unit, normalizeUnitText(category.inventory_unit, 'kg'))),
    unit_conversion_json: Array.isArray(category.unit_conversion_rows)
      ? unitConversionJSONFromRows(category.unit_conversion_rows)
      : normalizeJSONString(category.unit_conversion_json),
    integer_unit: Boolean(category.integer_unit),
	}
}

export function buildProductConfigTemplatePayload(form = {}) {
	return {
		id: Number(form.id || 0),
		customer_id: Number(form.customer_id || 0),
		name: String(form.name || '').trim(),
		gradient_template_id: productConfigTemplateNeedsGradientTemplate(form) ? Number(form.gradient_template_id || 0) : 0,
		operation_template_id: Number(form.operation_template_id || 0),
		unit_template_id: Number(form.unit_template_id || 0),
		price_list_rule_json: hasStructuredPriceRuleFields(form)
			? priceListRuleJSONFromForm(form)
			: normalizeJSONString(form.price_list_rule_json),
		special_attrs_schema_json: Array.isArray(form.special_attrs_schema_rows)
			? specialAttrSchemaJSONFromRows(form.special_attrs_schema_rows)
			: normalizeJSONArrayString(form.special_attrs_schema_json),
		active: form.active === false ? false : true,
	}
}

export function productConfigTemplateNeedsGradientTemplate(form = {}) {
  const ruleForm = hasStructuredPriceRuleFields(form) ? form : priceListRuleFormFromJSON(form.price_list_rule_json || '{}')
  return optionValue(ruleForm.price_rule_pricing_mode, priceListRulePricingModeOptions, 'inherit_gradient_template') === 'inherit_gradient_template'
}

export function buildProductUnitDefinitionPayload(form = {}) {
  return {
    code: String(form.code || '').trim(),
    name: String(form.name || '').trim(),
    unit_type: String(form.unit_type || '').trim() || 'other',
    allow_decimal: Boolean(form.allow_decimal),
    active: form.active === false ? false : true,
  }
}

export function buildProductUnitTemplatePayload(form = {}) {
  return {
    id: Number(form.id || 0),
    name: String(form.name || '').trim(),
    inventory_unit: normalizeUnitText(form.inventory_unit, 'kg'),
    quote_unit: normalizeUnitText(form.quote_unit, normalizeUnitText(form.inventory_unit, 'kg')),
    order_unit: normalizeUnitText(form.order_unit, normalizeUnitText(form.quote_unit, normalizeUnitText(form.inventory_unit, 'kg'))),
    unit_conversion_json: Array.isArray(form.unit_conversion_rows)
      ? unitConversionJSONFromRows(form.unit_conversion_rows)
      : normalizeJSONString(form.unit_conversion_json),
    integer_unit: Boolean(form.integer_unit),
    active: form.active === false ? false : true,
  }
}

export function buildSkuConfigOverridePayload(row = {}) {
  return {
    gradient_template_id_override: Number(row.gradient_template_id_override || 0),
    operation_template_id_override: Number(row.operation_template_id_override || 0),
    unit_rule_override_json: hasStructuredUnitRuleFields(row)
      ? unitRuleJSONFromForm(row)
      : normalizeJSONString(row.unit_rule_override_json),
  }
}

export function buildCustomerProductRuleTemplatePayload(form = {}) {
  return {
    id: Number(form.id || 0),
    customer_id: Number(form.customer_id || 0),
    name: String(form.name || '').trim(),
    active: form.active === false ? false : true,
    items: (form.items || []).map(buildCustomerProductRuleTemplateItemPayload),
  }
}

export function buildCustomerProductRuleTemplateItemPayload(row = {}) {
  return {
    product_subtype_category_id: Number(row.product_subtype_category_id || 0),
    gradient_template_id: Number(row.gradient_template_id || 0),
    operation_template_id: Number(row.operation_template_id || 0),
    price_list_rule_json: hasStructuredPriceRuleFields(row)
      ? priceListRuleJSONFromForm(row)
      : normalizeJSONString(row.price_list_rule_json),
    unit_rule_json: hasStructuredUnitRuleFields(row)
      ? unitRuleJSONFromForm(row)
      : normalizeJSONString(row.unit_rule_json),
    active: row.active === false ? false : true,
  }
}

export function buildCustomerProductRuleOverridePayload(row = {}) {
  return {
    id: Number(row.id || 0),
    customer_id: Number(row.customer_id || 0),
    product_subtype_category_id: Number(row.product_subtype_category_id || 0),
    gradient_template_id: Number(row.gradient_template_id || 0),
    operation_template_id: Number(row.operation_template_id || 0),
    price_list_rule_json: hasStructuredPriceRuleFields(row)
      ? priceListRuleJSONFromForm(row)
      : normalizeJSONString(row.price_list_rule_json),
    unit_rule_json: hasStructuredUnitRuleFields(row)
      ? unitRuleJSONFromForm(row)
      : normalizeJSONString(row.unit_rule_json),
    active: row.active === false ? false : true,
  }
}

export function buildCustomerProductRuleBindingPayload(customerID, templateID) {
  return {
    customer_id: Number(customerID || 0),
    template_id: Number(templateID || 0),
  }
}

export function buildSkuContextCategoryTree(categories = [], context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  const publicRoots = (categories || []).filter((category) => Number(category.customer_id || 0) === 0)
  if (!customerID) {
    return numberCategoryTree(publicRoots.map((primary) => projectCategoryNode(primary, context, null)))
  }

  const customerRoots = (categories || []).filter((category) => Number(category.customer_id || 0) === customerID)
  const customerRootBySource = new Map(customerRoots
    .filter((category) => Number(category.source_category_id || 0) > 0)
    .map((category) => [Number(category.source_category_id || 0), category]))
  const usedCustomerRootIDs = new Set()
  const out = []

  for (const publicRoot of publicRoots) {
    const derivedRoot = customerRootBySource.get(Number(publicRoot.id || 0))
    if (derivedRoot) {
      usedCustomerRootIDs.add(Number(derivedRoot.id || 0))
      out.push(projectMergedCategoryNode(derivedRoot, publicRoot, context, null))
      continue
    }
    if (categoryBelongsToSkuContext(publicRoot, context)) {
      out.push(projectCategoryNode(publicRoot, context, null))
    }
  }

  for (const customerRoot of customerRoots) {
    if (usedCustomerRootIDs.has(Number(customerRoot.id || 0))) continue
    if (categoryBelongsToSkuContext(customerRoot, context)) {
      out.push(projectCategoryNode(customerRoot, context, null))
    }
  }

  return numberCategoryTree(out)
}

export function isPublicReferenceRow(row = {}, context = {}) {
  const customerID = Number(context.customerID || context.customer_id || 0)
  return customerID > 0 && Number(row.customer_id || 0) === 0
}

function projectMergedCategoryNode(customerCategory = {}, publicCategory = {}, context = {}, parentName = null) {
  const primaryName = parentName === null ? customerCategory.name || publicCategory.name || '' : parentName
  const secondaryName = parentName === null ? '' : customerCategory.name || publicCategory.name || ''
  const mergedProducts = [
    ...contextProductsForCategory(customerCategory, context),
    ...contextProductsForCategory(publicCategory, context),
  ]
  const customerChildren = customerCategory.children || []
  const publicChildren = publicCategory.children || []
  const customerChildBySource = new Map(customerChildren
    .filter((category) => Number(category.source_category_id || 0) > 0)
    .map((category) => [Number(category.source_category_id || 0), category]))
  const usedCustomerChildIDs = new Set()
  const children = []

  for (const publicChild of publicChildren) {
    const derivedChild = customerChildBySource.get(Number(publicChild.id || 0))
    if (derivedChild) {
      usedCustomerChildIDs.add(Number(derivedChild.id || 0))
      children.push(projectMergedCategoryNode(derivedChild, publicChild, context, primaryName))
      continue
    }
    if (categoryBelongsToSkuContext(publicChild, context)) {
      children.push(projectCategoryNode(publicChild, context, customerCategory.name || publicCategory.name || ''))
    }
  }

  for (const customerChild of customerChildren) {
    if (usedCustomerChildIDs.has(Number(customerChild.id || 0))) continue
    if (categoryBelongsToSkuContext(customerChild, context)) {
      children.push(projectCategoryNode(customerChild, context, customerCategory.name || publicCategory.name || ''))
    }
  }

  return {
    ...customerCategory,
    products: numberProducts(dedupeRowsByID(mergedProducts), primaryName, secondaryName),
    children: numberCategoryChildren(children),
  }
}

function projectCategoryNode(category = {}, context = {}, parentName = null) {
  const primaryName = parentName === null ? category.name || '' : parentName
  const secondaryName = parentName === null ? '' : category.name || ''
  return {
    ...category,
    products: numberProducts(contextProductsForCategory(category, context), primaryName, secondaryName),
    children: numberCategoryChildren((category.children || [])
      .filter((child) => categoryBelongsToSkuContext(child, context))
      .map((child) => projectCategoryNode(child, context, category.name || ''))),
  }
}

function contextProductsForCategory(category = {}, context = {}) {
  return (category.products || [])
    .filter((product) => productBelongsToCategoryTree(product, context))
}

function productBelongsToCategoryTree(product = {}, context = {}) {
  const allowPublicInCategoryTree = Boolean(context.usePublicSku || context.use_public_sku || context.usePublicSkuInCategoryTree)
  return productBelongsToSkuContext(product, {
    ...context,
    usePublicSku: allowPublicInCategoryTree,
    use_public_sku: allowPublicInCategoryTree,
  })
}

function numberCategoryTree(nodes = []) {
  return numberCategoryChildren(nodes)
}

function numberCategoryChildren(nodes = []) {
  return (nodes || []).map((node, index) => ({
    ...node,
    number: index + 1,
    children: numberCategoryChildren(node.children || []),
  }))
}

function numberProducts(products = [], primaryName = '', secondaryName = '') {
  return (products || []).map((product, index) => ({
    ...product,
    number: index + 1,
    primary_name: primaryName,
    secondary_name: secondaryName,
  }))
}

function categoryProductMetaByID(categoryTree = []) {
  const out = new Map()
  for (const primary of categoryTree || []) {
    const primaryName = primary?.name || ''
    for (const product of primary?.products || []) {
      const id = Number(product?.id || 0)
      if (!id) continue
      out.set(id, {
        number: product.number || '',
        primary_name: primaryName,
        secondary_name: '',
      })
    }
    for (const secondary of primary?.children || []) {
      const secondaryName = secondary?.name || ''
      for (const product of secondary?.products || []) {
        const id = Number(product?.id || 0)
        if (!id) continue
        out.set(id, {
          number: product.number || '',
          primary_name: primaryName,
          secondary_name: secondaryName,
        })
      }
    }
  }
  return out
}

function categoryPathMetaByID(categoryTree = []) {
  const out = new Map()
  function visit(category = {}, primaryName = '', secondaryName = '') {
    const id = Number(category?.id || 0)
    if (id) {
      out.set(id, {
        primary_name: primaryName || category?.name || '',
        secondary_name: secondaryName,
      })
    }
    const nextPrimaryName = primaryName || category?.name || ''
    for (const child of category?.children || []) {
      visit(child, nextPrimaryName, child?.name || '')
    }
  }
  for (const primary of categoryTree || []) {
    visit(primary, primary?.name || '', '')
  }
  return out
}

function dedupeRowsByID(rows = []) {
  const seen = new Set()
  const out = []
  for (const row of rows || []) {
    const id = Number(row?.id || 0)
    if (id && seen.has(id)) continue
    if (id) seen.add(id)
    out.push(row)
  }
  return out
}

function isUnmodifiedPublicSkuCopy(product = {}, publicProducts = []) {
  const baseID = Number(product.base_product_id || 0)
  if (!baseID || product.custom_type !== 'public_sku_alias') return false
  const base = (publicProducts || []).find((row) => Number(row.id || 0) === baseID)
  return Boolean(base && String(base.name || '').trim().toLowerCase() === String(product.name || '').trim().toLowerCase())
}

function hasCustomerDerivedProduct(product = {}, customerProducts = []) {
  const productID = Number(product.id || 0)
  if (!productID) return false
  return (customerProducts || []).some((row) => Number(row.base_product_id || 0) === productID && String(row.custom_type || '').trim() === 'public_sku_alias')
}

function hasCustomerDerivedCategory(category = {}, customerCategories = []) {
  const categoryID = Number(category.id || 0)
  if (!categoryID) return false
  return (customerCategories || []).some((row) => Number(row.source_category_id || 0) === categoryID)
}

function hasCustomerDerivedTemplate(template = {}, customerTemplates = []) {
  const templateID = Number(template.id || 0)
  if (!templateID) return false
  return (customerTemplates || []).some((row) => Number(row.source_template_id || 0) === templateID)
}

function isDuplicatedPublicCategory(category = {}, publicCategories = [], publicProducts = []) {
  if (!(category.products || []).length && !(category.children || []).length) return false
  const matchesPublicCategory = (publicCategories || []).some((row) => (
    Number(row.customer_id || 0) === 0
    && Number(row.level || 0) === Number(category.level || 0)
    && String(row.name || '').trim().toLowerCase() === String(category.name || '').trim().toLowerCase()
  ))
  if (!matchesPublicCategory) return false
  if ((category.products || []).some((product) => !isUnmodifiedPublicSkuCopy(product, publicProducts))) {
    return false
  }
  if ((category.children || []).some((child) => !isDuplicatedPublicCategory(child, publicCategories, publicProducts))) {
    return false
  }
  return true
}

export function primaryCategoryOptions(rows = []) {
  return uniqueSorted((rows || []).map((row) => row.primary_name))
}

export function secondaryCategoryOptions(rows = [], primaryCategory = '') {
  const primary = String(primaryCategory || '').trim()
  return uniqueSorted((rows || [])
    .filter((row) => !primary || String(row.primary_name || '') === primary)
    .map((row) => row.secondary_name))
}

export function productSubtypeCategoryOptionsForType(categoryTree = [], productTypeCategoryID = 0) {
  const typeID = Number(productTypeCategoryID || 0)
  if (!typeID) return []
  const productType = (categoryTree || []).find((category) => Number(category?.id || 0) === typeID)
  if (!productType) return []
  return (productType.children || [])
    .filter((category) => Number(category?.id || 0) > 0)
    .map((category) => ({
      id: Number(category.id || 0),
      parent_id: Number(category.parent_id || typeID),
      name: category.name || '',
      customer_id: Number(category.customer_id || 0),
		source_category_id: Number(category.source_category_id || 0),
		product_config_template_id: Number(category.product_config_template_id || 0),
		template_state: category.template_state || '',
	}))
}

export function buildProductCreatePayload(form = {}) {
  const kind = normalizedProductKind(form)
  const payload = {
    name: String(form.name || '').trim(),
    product_kind: kind,
    remark: String(form.remark || '').trim(),
  }
  const configTemplateID = Number(form.product_config_template_id || 0)
  if (configTemplateID > 0) payload.product_config_template_id = configTemplateID
  if (kind === 'green_bean') return payload
  const yieldRate = normalizedYieldRateFromPercent(form)
  if (yieldRate !== null) payload.yield_rate = yieldRate
  return payload
}

export function buildCustomProductCreatePayload(customerID, form = {}) {
  const kind = normalizedProductKind(form)
  const payload = {
    customer_id: Number(customerID || form.customer_id || 0),
    base_product_id: Number(form.base_product_id || 0),
    name: String(form.name || '').trim(),
    remark: String(form.remark || '').trim(),
    product_kind: kind,
    custom_type: String(form.custom_type || '').trim(),
    copy_bom: Boolean(form.copy_bom),
    copy_price_tiers: Boolean(form.copy_price_tiers),
  }
  if (kind === 'green_bean') {
    payload.base_product_id = 0
    payload.copy_bom = false
    payload.copy_price_tiers = false
    return payload
  }
  if (payload.custom_type === 'custom_roast') {
    payload.base_product_id = 0
    payload.copy_bom = false
    payload.copy_price_tiers = false
  }
  const yieldRate = normalizedYieldRateFromPercent(form)
  if (yieldRate !== null) payload.yield_rate = yieldRate
  if (kind === 'instant_coffee') {
    payload.copy_bom = false
    return payload
  }
  return payload
}

export function buildSkuCreatePayload(customerID, form = {}) {
  const payload = {
    customer_id: Number(customerID || form.customer_id || 0),
    name: String(form.name || '').trim(),
    remark: String(form.remark || '').trim(),
    active: form.active === false ? false : true,
  }
  const configTemplateID = Number(form.product_config_template_id || 0)
  if (configTemplateID > 0) payload.product_config_template_id = configTemplateID
  return payload
}

export function resolveCreatedProductForConfig(result = {}, products = []) {
  const createdProduct = result?.product || result?.sku || result || {}
  const createdID = Number(createdProduct.id || createdProduct.product_id || 0)
  if (createdID > 0) {
    const product = products.find(row => Number(row?.id || row?.product_id || 0) === createdID)
    if (product) return product
  }

  const createdCode = String(createdProduct.code || createdProduct.product_code || createdProduct.number || '').trim()
  if (createdCode) {
    const product = products.find(row => String(row?.code || row?.product_code || row?.number || '').trim() === createdCode)
    if (product) return product
  }

  const createdName = String(createdProduct.name || createdProduct.product_name || '').trim()
  if (createdName) {
    const product = products.find(row => String(row?.name || row?.product_name || '').trim() === createdName)
    if (product) return product
  }

  return createdID > 0 || createdName ? createdProduct : null
}

export function buildProductProductionConfigField(row = {}, index = 0) {
  const source = row && typeof row === 'object' ? row : {}
  const rawType = String(source.field_type || '').trim()
  const type = ['text', 'textarea', 'number', 'ratio', 'select', 'checkbox', 'date', 'bool'].includes(rawType) ? (rawType === 'bool' ? 'checkbox' : rawType) : 'text'
  return {
    local_id: `${Number(source.id || 0) || 'new'}-${Date.now()}-${index}`,
    id: Number(source.id || 0),
    field_key: String(source.field_key || '').trim(),
    template_field_key: String(source.template_field_key || source.field_key || '').trim(),
    label: String(source.label || '').trim(),
    field_type: type,
    unit: String(source.unit || '').trim(),
    value_text: String(source.value_text || '').trim(),
    value_number: source.value_number === null || typeof source.value_number === 'undefined' || source.value_number === '' ? null : Number(source.value_number),
    value_bool: Boolean(source.value_bool),
    required: Boolean(source.required),
    options_json: String(source.options_json || '[]').trim() || '[]',
    show_in_price_list: source.show_in_price_list !== false,
    sort_order: Number(source.sort_order || index + 1),
  }
}

export function buildProductProductionConfigForm(config = {}, product = {}) {
  const sourceConfig = config && typeof config === 'object' ? config : {}
  const sourceProduct = product && typeof product === 'object' ? product : {}
  const lossRate = Number(sourceConfig.expected_loss_rate ?? sourceProduct.expected_loss_rate ?? 0)
  const fields = Array.isArray(sourceConfig.fields) ? sourceConfig.fields : []
  return {
    product_id: Number(sourceConfig.product_id || sourceProduct.id || 0),
    name: String(sourceProduct.name || '').trim(),
    remark: String(sourceProduct.remark || '').trim(),
    product_kind: sourceProduct.product_kind || 'roasted',
    product_config_template_id: Number(sourceProduct.product_config_template_id || 0),
    production_bom_id: Number(sourceConfig.production_bom_id || sourceProduct.production_bom_id || 0),
    production_bom_version_id: Number(sourceConfig.production_bom_version_id || sourceProduct.production_bom_version_id || 0),
    process_route_id: Number(sourceConfig.process_route_id || 0),
    industry_field_template_id: Number(sourceConfig.industry_field_template_id || 0),
    expected_loss_percent: Number.isFinite(lossRate) && lossRate > 0 ? Number((lossRate * 100).toFixed(2)) : 0,
    note: String(sourceConfig.note || sourceProduct.production_config_note || '').trim(),
    fields: fields
      .slice()
      .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0))
      .map((field, index) => buildProductProductionConfigField(field, index)),
  }
}

export function buildProductBasicsPayload(row = {}, marginRateOverride = null) {
  const kind = normalizedProductKind(row)
  const payload = {
    product_kind: kind,
    remark: String(row.remark || '').trim(),
  }
  const name = String(row.name || '').trim()
  if (name) payload.name = name
  if (Object.prototype.hasOwnProperty.call(row, 'product_config_template_id')) {
    payload.product_config_template_id = Number(row.product_config_template_id || 0)
  }
  if (kind !== 'green_bean') {
    const yieldRate = normalizedYieldRateFromPercent(row)
    if (yieldRate !== null) payload.yield_rate = yieldRate
  }
  payload.margin_rate_override = marginRateOverride
  return payload
}

function normalizedYieldRateFromPercent(form = {}) {
  if (!Object.prototype.hasOwnProperty.call(form, 'yield_percent')) return null
  const rate = Number(form.yield_percent || 0) / 100
  if (!Number.isFinite(rate) || rate <= 0) return null
  return Number(rate.toFixed(4))
}

function rowCustomerID(row = {}) {
  return Number(row.customer_id ?? row.customerID ?? 0)
}

function rowOrderUsageCount(row = {}) {
  const raw = row.order_usage_count ?? row.orderUsageCount ?? row.order_count ?? row.orderCount ?? 0
  const value = Number(raw || 0)
  return Number.isFinite(value) ? value : 0
}

export function sortRowsForCustomerSkuPriority(rows = [], customerID = 0) {
  const selectedCustomerID = Number(customerID || 0)
  return [...rows].sort((a, b) => {
    if (selectedCustomerID > 0) {
      const aOwned = rowCustomerID(a) === selectedCustomerID ? 0 : 1
      const bOwned = rowCustomerID(b) === selectedCustomerID ? 0 : 1
      if (aOwned !== bOwned) return aOwned - bOwned
    }
    const usageDiff = rowOrderUsageCount(b) - rowOrderUsageCount(a)
    if (usageDiff !== 0) return usageDiff
    const positionDiff = Number(a.product_category_position || 0) - Number(b.product_category_position || 0)
    if (positionDiff !== 0) return positionDiff
    const numberDiff = Number(a.number || 0) - Number(b.number || 0)
    if (numberDiff !== 0) return numberDiff
    return String(a.name || '').localeCompare(String(b.name || ''))
  })
}

export function buildProductBomURL(currentHref = '', row = {}) {
  const url = new URL(currentHref || window.location.href)
  const bomID = Number(row.production_bom_id || row.bom_id || 0)
  url.searchParams.set('view', 'bom')
  if (bomID > 0) {
    url.searchParams.set('production_bom_id', String(bomID))
  } else {
    url.searchParams.delete('production_bom_id')
  }
  url.searchParams.delete('product_id')
  url.searchParams.delete('bom_filter_product_id')
  return url
}

function uniqueSorted(values = []) {
  return Array.from(new Set(values.map((value) => String(value || '').trim()).filter(Boolean)))
    .sort((a, b) => a.localeCompare(b))
}

export function priceListRuleFormFromJSON(value = {}) {
	const rule = parseJSONObject(value)
	const extra = { ...rule }
	for (const key of ['enabled', 'include_in_price_list', 'pricing_mode', 'display_mode', 'display_unit', 'rounding', 'tax_included', 'unit_price', 'price_per_unit', 'fixed_unit_price', 'fixed_price', 'cost_plus_rate', 'markup_rate', 'margin_rate']) {
    delete extra[key]
	}
  const fixedUnitPrice = normalizeOptionalNumber(rule.fixed_unit_price ?? rule.unit_price ?? rule.price_per_unit ?? rule.fixed_price)
  const costPlusRate = normalizeOptionalNumber(rule.cost_plus_rate ?? rule.markup_rate ?? rule.margin_rate)
	return {
		price_rule_pricing_mode: optionValue(rule.pricing_mode, priceListRulePricingModeOptions, 'inherit_gradient_template'),
		price_rule_fixed_unit_price: fixedUnitPrice === null ? '' : fixedUnitPrice,
		price_rule_cost_plus_percent: costPlusRate === null ? '' : Number((costPlusRate * 100).toFixed(4)),
    price_rule_rounding: optionValue(rule.rounding, priceListRuleRoundingOptions, 'none'),
    price_rule_tax_included: Boolean(rule.tax_included),
    price_rule_extra: extra,
  }
}

export function priceListRuleJSONFromForm(form = {}) {
	const out = sanitizeExtraObject(form.price_rule_extra)
	out.pricing_mode = optionValue(form.price_rule_pricing_mode, priceListRulePricingModeOptions, 'inherit_gradient_template')
  const fixedUnitPrice = normalizeOptionalNumber(form.price_rule_fixed_unit_price)
  if (out.pricing_mode === 'fixed_unit_price' && fixedUnitPrice !== null) {
    out.fixed_unit_price = trimDecimal(fixedUnitPrice)
  }
  const costPlusPercent = normalizeOptionalNumber(form.price_rule_cost_plus_percent)
  if (out.pricing_mode === 'cost_plus' && costPlusPercent !== null) {
    out.cost_plus_rate = trimDecimal(costPlusPercent / 100)
  }
  out.rounding = optionValue(form.price_rule_rounding, priceListRuleRoundingOptions, 'none')
  out.tax_included = Boolean(form.price_rule_tax_included)
  return JSON.stringify(out)
}

export function unitConversionRowsFromJSON(value = {}) {
  const conversion = parseJSONObject(value)
  const rows = []
  for (const [fromUnit, targets] of Object.entries(conversion)) {
    const normalizedFromUnit = normalizeOptionalUnitText(fromUnit)
    if (!normalizedFromUnit) continue
    const targetMap = parseJSONObject(targets)
    for (const [toUnit, ratio] of Object.entries(targetMap)) {
      const normalizedToUnit = normalizeOptionalUnitText(toUnit)
      const numericRatio = normalizePositiveNumber(ratio)
      if (!normalizedToUnit || numericRatio <= 0) continue
      rows.push({
        from_qty: 1,
        from_unit: normalizedFromUnit,
        to_qty: numericRatio,
        to_unit: normalizedToUnit,
      })
    }
  }
  return rows
}

export function unitConversionJSONFromRows(rows = []) {
  const out = {}
  for (const row of rows || []) {
    const fromQty = normalizePositiveNumber(row?.from_qty)
    const toQty = normalizePositiveNumber(row?.to_qty)
    const fromUnit = normalizeOptionalUnitText(row?.from_unit)
    const toUnit = normalizeOptionalUnitText(row?.to_unit)
    if (fromQty <= 0 || toQty <= 0 || !fromUnit || !toUnit) continue
    if (!out[fromUnit]) out[fromUnit] = {}
    out[fromUnit][toUnit] = trimDecimal(toQty / fromQty)
  }
  return JSON.stringify(out)
}

export function unitRuleFormFromJSON(value = {}) {
  const rule = parseJSONObject(value)
  const conversion = rule.unit_conversion_json ?? rule.conversion_json ?? {}
  const extra = { ...rule }
  for (const key of ['inventory_unit', 'quote_unit', 'order_unit', 'unit_conversion_json', 'conversion_json', 'integer_unit']) {
    delete extra[key]
  }
  return {
    inventory_unit: normalizeOptionalUnitText(rule.inventory_unit),
    quote_unit: normalizeOptionalUnitText(rule.quote_unit),
    order_unit: normalizeOptionalUnitText(rule.order_unit),
    unit_conversion_rows: unitConversionRowsFromJSON(conversion),
    integer_unit_mode: integerUnitModeFromValue(rule.integer_unit),
    unit_rule_extra: extra,
  }
}

export function unitRuleJSONFromForm(form = {}) {
  const out = sanitizeExtraObject(form.unit_rule_extra)
  const inventoryUnit = normalizeOptionalUnitText(form.inventory_unit)
  const quoteUnit = normalizeOptionalUnitText(form.quote_unit)
  const orderUnit = normalizeOptionalUnitText(form.order_unit)
  if (inventoryUnit) out.inventory_unit = inventoryUnit
  if (quoteUnit) out.quote_unit = quoteUnit
  if (orderUnit) out.order_unit = orderUnit
  const conversionJSON = unitConversionJSONFromRows(form.unit_conversion_rows || [])
  const conversion = parseJSONObject(conversionJSON)
  if (Object.keys(conversion).length) out.unit_conversion_json = conversion
  const mode = String(form.integer_unit_mode || '').trim()
  if (mode === 'integer') out.integer_unit = true
  if (mode === 'decimal') out.integer_unit = false
  return JSON.stringify(out)
}

export function specialAttrSchemaRowsFromJSON(value = []) {
  const rows = parseJSONArray(value)
  return rows
    .map((row, index) => normalizeSpecialAttrSchemaRow(row, index + 1))
    .filter((row) => row.key)
    .sort((a, b) => Number(a.position || 0) - Number(b.position || 0) || a.key.localeCompare(b.key))
}

export function specialAttrSchemaJSONFromRows(rows = []) {
  const out = []
  const seen = new Set()
  for (const row of rows || []) {
    const normalized = normalizeSpecialAttrSchemaRow(row, out.length + 1)
    if (!normalized.key || seen.has(normalized.key)) continue
    seen.add(normalized.key)
    out.push({
      key: normalized.key,
      label: normalized.label,
      value_type: normalized.value_type,
      options: normalized.options,
      required: normalized.required,
      show_in_price_list: normalized.show_in_price_list,
      position: out.length + 1,
    })
  }
  return JSON.stringify(out)
}

export function specialAttrValuesFromJSON(value = {}) {
  const parsed = parseJSONObject(value)
  const out = {}
  for (const [key, raw] of Object.entries(parsed)) {
    const normalizedKey = normalizeSpecialAttrKey(key)
    if (!normalizedKey) continue
    const normalizedValue = normalizeSpecialAttrValue(raw)
    if (normalizedValue === '') continue
    out[normalizedKey] = normalizedValue
  }
  return out
}

export function specialAttrValuesJSONFromForm(value = {}) {
  const source = typeof value === 'string' ? specialAttrValuesFromJSON(value) : (value && typeof value === 'object' && !Array.isArray(value) ? value : {})
  const out = {}
  for (const [key, raw] of Object.entries(source)) {
    const normalizedKey = normalizeSpecialAttrKey(key)
    if (!normalizedKey) continue
    const normalizedValue = normalizeSpecialAttrValue(raw)
    if (normalizedValue === '') continue
    out[normalizedKey] = normalizedValue
  }
  return JSON.stringify(out)
}

function normalizeSpecialAttrSchemaRow(row = {}, fallbackPosition = 1) {
  const key = normalizeSpecialAttrKey(row?.key)
  const label = String(row?.label || key).trim()
  const valueType = ['text', 'select', 'number', 'boolean'].includes(String(row?.value_type || '').trim())
    ? String(row.value_type).trim()
    : 'text'
  const options = valueType === 'select' ? normalizeSpecialAttrOptions(row) : []
  return {
    key,
    label,
    value_type: valueType,
    options,
    options_text: options.join('\n'),
    required: Boolean(row?.required),
    show_in_price_list: Boolean(row?.show_in_price_list ?? row?.showInPriceList),
    position: Number(row?.position || fallbackPosition || 1),
  }
}

function normalizeSpecialAttrOptions(row = {}) {
  const source = Object.prototype.hasOwnProperty.call(row, 'options_text')
    ? row.options_text
    : row?.options
  if (Array.isArray(source)) {
    return source.map((item) => String(item || '').trim()).filter(Boolean)
  }
  return String(source || '')
    .split(/[\n,，;；]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function normalizeSpecialAttrKey(value) {
  return String(value || '').trim().replace(/\s+/g, '_')
}

function normalizeSpecialAttrValue(value) {
  if (value === null || typeof value === 'undefined') return ''
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  return String(value).trim()
}

function normalizeUnitText(value, fallback = 'kg') {
  const normalized = String(value || '').trim()
  if (normalized) return normalized
  return String(fallback || '').trim() || 'kg'
}

function normalizeOptionalUnitText(value) {
  return String(value || '').trim()
}

function normalizeJSONString(value) {
  const raw = String(value || '').trim()
  return raw || '{}'
}

function normalizeJSONArrayString(value) {
  const raw = String(value || '').trim()
  return raw || '[]'
}

function parseJSONObject(value) {
  if (!value) return {}
  if (typeof value === 'object' && !Array.isArray(value)) return value
  try {
    const parsed = JSON.parse(String(value || '{}'))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function parseJSONArray(value) {
  if (!value) return []
  if (Array.isArray(value)) return value
  try {
    const parsed = JSON.parse(String(value || '[]'))
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function sanitizeExtraObject(value) {
  const parsed = parseJSONObject(value)
  return { ...parsed }
}

function optionValue(value, options = [], fallback = '') {
  const normalized = String(value || '').trim()
  return options.some((option) => option.value === normalized) ? normalized : fallback
}

function normalizePositiveNumber(value) {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) && numberValue > 0 ? numberValue : 0
}

function normalizeOptionalNumber(value) {
  if (value === null || typeof value === 'undefined' || value === '') return null
  const numberValue = Number(value)
  return Number.isFinite(numberValue) && numberValue >= 0 ? numberValue : null
}

function trimDecimal(value) {
  return Number(Number(value).toFixed(8))
}

function integerUnitModeFromValue(value) {
  if (typeof value === 'undefined' || value === null || value === '') return 'inherit'
  if (value === true) return 'integer'
  if (value === false) return 'decimal'
  const normalized = String(value).trim().toLowerCase()
  if (['true', '1', 'yes', 'integer'].includes(normalized)) return 'integer'
  if (['false', '0', 'no', 'decimal'].includes(normalized)) return 'decimal'
  return 'inherit'
}

function hasStructuredPriceRuleFields(row = {}) {
	return Object.prototype.hasOwnProperty.call(row, 'price_rule_pricing_mode')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_display_mode')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_fixed_unit_price')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_cost_plus_percent')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_rounding')
		|| Object.prototype.hasOwnProperty.call(row, 'price_rule_tax_included')
}

function hasStructuredUnitRuleFields(row = {}) {
  return Object.prototype.hasOwnProperty.call(row, 'integer_unit_mode')
    || Object.prototype.hasOwnProperty.call(row, 'unit_conversion_rows')
}
