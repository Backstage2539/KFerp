import type {
  BeanListProductSummary,
  BeanListSummary,
  ResaleBeanListCommand,
  ResaleBeanListItemOverride,
} from '../api/customerPortal'

export function resaleBeanListItemKey(item: Pick<BeanListProductSummary, 'code' | 'name'>): string {
  return String(item.code || '').trim() || String(item.name || '').trim()
}

export function defaultResaleBeanListDraft(source: BeanListSummary, nextVersionNo = 'V1'): ResaleBeanListCommand {
  const selected = new Set<string>()
  for (const group of source.groups || []) {
    for (const item of group.items || []) {
      const key = resaleBeanListItemKey(item)
      if (key) selected.add(key)
    }
  }
  return {
    source_publication_id: Number(source.id || 0),
    version_no: String(nextVersionNo || 'V1').trim() || 'V1',
    gradient_template_id: 0,
    selected_item_codes: Array.from(selected),
    config: {
      brandName: String(source.brand_name || '').trim(),
      brandIntro: String(source.brand_intro || '').trim(),
      backgroundColor: source.background_color || '#f8f1e5',
      fontColor: source.font_color || '#171717',
      backgroundImage: source.background_image || '',
      logoImage: source.logo_image || '',
      layoutStyle: source.layout_style || 'card',
      cardsPerRow: Number(source.cards_per_row || 2),
      showVersion: source.show_version !== false,
      showChangelog: source.show_changelog !== false,
      showCategoryNumbers: source.show_category_numbers !== false,
    },
    price_rule: { add_amount: 0, multiplier: 1 },
    item_overrides: [],
    changelog: '',
  }
}

export function buildResaleBeanListPublishPayload(draft: ResaleBeanListCommand): ResaleBeanListCommand {
  return {
    source_publication_id: Number(draft.source_publication_id || 0),
    version_no: String(draft.version_no || '').trim(),
    gradient_template_id: Number(draft.gradient_template_id || 0),
    selected_item_codes: normalizeStringList(draft.selected_item_codes),
    price_rule: {
      add_amount: normalizeNumber(draft.price_rule?.add_amount, 0),
      multiplier: normalizeNumber(draft.price_rule?.multiplier, 1),
    },
    config: normalizeConfig(draft.config),
    item_overrides: normalizeItemOverrides(draft.item_overrides || []),
    changelog: String(draft.changelog || '').trim(),
  }
}

function normalizeStringList(rows: string[] = []): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const row of rows) {
    const value = String(row || '').trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    out.push(value)
  }
  return out
}

function normalizeConfig(config: Record<string, unknown> = {}): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(config || {})) {
    if (typeof value === 'string') {
      out[key] = value.trim()
    } else {
      out[key] = value
    }
  }
  return out
}

function normalizeItemOverrides(rows: ResaleBeanListItemOverride[]): ResaleBeanListItemOverride[] {
  return rows
    .map((row) => ({
      code: String(row.code || '').trim(),
      label: String(row.label || '').trim() || undefined,
      price: row.price == null ? undefined : normalizeNumber(row.price, 0),
      badge_label: String(row.badge_label || '').trim() || undefined,
      recommended_use: String(row.recommended_use || '').trim() || undefined,
      description: String(row.description || '').trim() || undefined,
      highlight_terms: normalizeStringList(row.highlight_terms || []),
    }))
    .filter((row) => row.code && hasMeaningfulItemOverride(row))
}

function normalizeNumber(value: unknown, fallback: number): number {
  const n = Number(value)
  return Number.isFinite(n) ? n : fallback
}

function hasMeaningfulItemOverride(row: ResaleBeanListItemOverride): boolean {
  return Boolean(
    row.label ||
      row.price != null ||
      row.badge_label ||
      row.recommended_use ||
      row.description ||
      (row.highlight_terms && row.highlight_terms.length > 0),
  )
}
