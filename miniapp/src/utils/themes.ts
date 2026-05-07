export type MiniappThemeKey = 'coffee_factory' | 'clean_ops' | 'premium_partner'

export type MiniappThemeOption = {
  key: MiniappThemeKey
  className: string
  label: string
  eyebrow: string
  subtitle: string
}

export const defaultMiniappThemeKey: MiniappThemeKey = 'coffee_factory'

export const miniappThemeOptions: MiniappThemeOption[] = [
  {
    key: 'coffee_factory',
    className: 'theme-coffee-factory',
    label: '咖啡工厂专业风',
    eyebrow: 'QACOOHEE SERVICE',
    subtitle: '豆单、订单、代发、库存和结算集中处理。',
  },
  {
    key: 'clean_ops',
    className: 'theme-clean-ops',
    label: '清爽业务工具风',
    eyebrow: '客户服务台',
    subtitle: '高频业务优先，订单、物流和库存更容易扫读。',
  },
  {
    key: 'premium_partner',
    className: 'theme-premium-partner',
    label: '品牌会员高级风',
    eyebrow: 'ROASTERY PARTNER',
    subtitle: '围绕豆单、定制服务和结算的合作伙伴入口。',
  },
]

const themesByKey = new Map(miniappThemeOptions.map((item) => [item.key, item]))

export function normalizeMiniappThemeKey(value?: string): MiniappThemeKey {
  const key = String(value || '').trim() as MiniappThemeKey
  return themesByKey.has(key) ? key : defaultMiniappThemeKey
}

export function miniappThemeMeta(value?: string): MiniappThemeOption {
  return themesByKey.get(normalizeMiniappThemeKey(value)) || miniappThemeOptions[0]
}

export function miniappThemeClass(value?: string): string {
  return miniappThemeMeta(value).className
}
