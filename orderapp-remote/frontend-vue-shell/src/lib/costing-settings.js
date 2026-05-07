export const categorySummaries = {
  base: '影响所有成本公式的基础换算和默认出成率。',
  production: '把烘焙、包装和损耗折入熟豆每公斤成本。',
  commercialBeans: '控制商用熟豆 454g 四档、227g 两档和 kg 三档报价。',
  retailBeans: '控制零售熟豆价格中的利润、税费和物流。',
  dripBags: '控制挂耳单袋成本、包装、物流和商用/零售利润系数。',
  other: '接口返回但前端暂未识别的参数。',
}

const categoryOrder = ['base', 'production', 'commercialBeans', 'retailBeans', 'dripBags', 'other']

const settingMeta = {
  roast_yield_rate: {
    category: 'base',
    description: '默认生豆到熟豆的出成率；物料没有单独设置时用它计算熟豆成本。',
    order: 10,
  },
  kg_to_lb_factor: {
    category: 'base',
    description: '把每公斤成本换算成每磅报价的系数。',
    order: 20,
  },
  small_batch_production_cost_per_kg: {
    category: 'production',
    description: '小批量烘焙时，每公斤熟豆分摊的生产成本。',
    order: 10,
  },
  large_batch_production_cost_per_kg: {
    category: 'production',
    description: '大批量烘焙时，每公斤熟豆分摊的生产成本。',
    order: 20,
  },
  wholesale_package_cost_per_kg: {
    category: 'production',
    description: '批发熟豆每公斤包装耗材成本。',
    order: 30,
  },
  product_loss_per_kg: {
    category: 'production',
    description: '包装、分装或操作损耗折算到每公斤的成本。',
    order: 40,
  },
  retail_bean_margin_rate: {
    category: 'retailBeans',
    description: '零售熟豆价格的利润系数，会直接影响 100g、200g、227g、250g 等零售价。',
    order: 10,
  },
  retail_tax_rate: {
    category: 'retailBeans',
    description: '零售熟豆价格中的税费比例。',
    order: 20,
  },
  retail_logistics_per_kg: {
    category: 'retailBeans',
    description: '零售熟豆按每公斤分摊的物流成本。',
    order: 30,
  },
  retail_drip_logistics_per_10_bags: {
    category: 'dripBags',
    description: '零售挂耳 10 袋装分摊的物流成本。',
    order: 70,
  },
  drip_green_ratio_kg_per_bag: {
    category: 'dripBags',
    description: '每袋挂耳消耗的熟豆公斤数，用来计算挂耳基础成本。',
    order: 10,
  },
  drip_process_cost_per_bag: {
    category: 'dripBags',
    description: '每袋挂耳的加工成本。',
    order: 20,
  },
  drip_extra_cost_per_bag: {
    category: 'dripBags',
    description: '每袋挂耳的额外成本，例如贴标、人工或损耗。',
    order: 30,
  },
  drip_packing_material_per_bag: {
    category: 'dripBags',
    description: '每袋挂耳外包装材料成本。',
    order: 40,
  },
  retail_drip_multiplier: {
    category: 'dripBags',
    description: '零售挂耳价格的利润系数。',
    order: 80,
  },
  wholesale_kg_margin_rate_1: {
    category: 'commercialBeans',
    description: '商用熟豆 2包-13包档利润系数；227g 两档的第一档也使用它。',
    order: 10,
  },
  wholesale_kg_margin_rate_2: {
    category: 'commercialBeans',
    description: '商用熟豆 14包-23包档利润系数；227g 两档的第二档也使用它。',
    order: 20,
  },
  wholesale_kg_margin_rate_3: {
    category: 'commercialBeans',
    description: '商用熟豆 24包-47包档利润系数。',
    order: 30,
  },
  wholesale_kg_margin_rate_4: {
    category: 'commercialBeans',
    description: '商用熟豆 48包+ 档利润系数；kg 三档的 24-49kg 档也使用它。',
    order: 40,
  },
  wholesale_kg_margin_rate_5: {
    category: 'commercialBeans',
    description: '商用熟豆 kg 三档的 50-99kg 档利润系数。',
    order: 50,
  },
  wholesale_kg_margin_rate_6: {
    category: 'commercialBeans',
    description: '商用熟豆 kg 三档的 100-199kg 档利润系数。',
    order: 60,
  },
  wholesale_drip_multiplier_1: {
    category: 'dripBags',
    description: '商用挂耳 100 包档利润系数。',
    order: 90,
  },
  wholesale_drip_multiplier_2: {
    category: 'dripBags',
    description: '商用挂耳 200 包档利润系数。',
    order: 100,
  },
  wholesale_drip_multiplier_3: {
    category: 'dripBags',
    description: '商用挂耳 300 包档利润系数。',
    order: 110,
  },
  wholesale_drip_multiplier_4: {
    category: 'dripBags',
    description: '商用挂耳 500 包档利润系数。',
    order: 120,
  },
}

const categoryTitles = {
  base: '基础换算',
  production: '生产与包装',
  commercialBeans: '商用熟豆',
  retailBeans: '零售熟豆',
  dripBags: '挂耳',
  other: '其他参数',
}

function normalizeNumber(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}

export function enrichCostingSetting(row) {
  const meta = settingMeta[row?.key] || {}
  return {
    key: row?.key || '',
    label: row?.label || row?.key || '',
    value: normalizeNumber(row?.value),
    unit: row?.unit || '',
    updated_at: row?.updated_at || '',
    category: meta.category || 'other',
    categoryTitle: categoryTitles[meta.category || 'other'],
    description: meta.description || '未归类参数，请确认公式用途后再调整。',
    order: meta.order || 999,
  }
}

export function groupCostingSettings(rows) {
  const byCategory = new Map(categoryOrder.map((key) => [key, []]))
  for (const row of rows || []) {
    const enriched = enrichCostingSetting(row)
    byCategory.get(enriched.category)?.push(enriched)
  }
  return categoryOrder
    .map((key) => ({
      key,
      title: categoryTitles[key],
      summary: categorySummaries[key],
      rows: (byCategory.get(key) || []).sort((a, b) => a.order - b.order || a.label.localeCompare(b.label)),
    }))
    .filter((group) => group.rows.length)
}
