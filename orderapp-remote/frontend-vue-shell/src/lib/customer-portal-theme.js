export const defaultCustomerPortalThemeKey = 'coffee_factory'

export const customerPortalThemeOptions = [
  {
    key: 'coffee_factory',
    label: '咖啡工厂专业风',
    description: '暖咖啡色，品牌感强，适合大多数客户',
    swatchClass: 'theme-swatch-coffee',
  },
  {
    key: 'clean_ops',
    label: '清爽业务工具风',
    description: '克制清楚，适合高频订单、物流、库存查询',
    swatchClass: 'theme-swatch-clean',
  },
  {
    key: 'premium_partner',
    label: '品牌会员高级风',
    description: '质感更强，适合合作伙伴和对外展示',
    swatchClass: 'theme-swatch-premium',
  },
]

const customerPortalThemeKeys = new Set(customerPortalThemeOptions.map((item) => item.key))

export function normalizeCustomerPortalThemeKey(value) {
  const key = String(value || '').trim()
  return customerPortalThemeKeys.has(key) ? key : defaultCustomerPortalThemeKey
}
