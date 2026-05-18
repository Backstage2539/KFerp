import type { BeanListProductSummary, BeanListSummary } from '../api/customerPortal'

export type HighlightPart = {
  text: string
  red: boolean
}

export type BeanListQualityLine = {
  label: string
  value: string
}

export function beanListDisplayStyle(item?: BeanListSummary | null): Record<string, string> {
  const style: Record<string, string> = {
    backgroundColor: item?.background_color || '#f8f1e5',
    color: item?.font_color || '#171717',
  }
  if (item?.background_image) {
    style.backgroundImage = `url("${item.background_image}")`
  }
  return style
}

export function beanListCardRows(items: BeanListProductSummary[] = [], cardsPerRow = 1): BeanListProductSummary[][] {
  const size = clampCardsPerRow(cardsPerRow)
  const rows: BeanListProductSummary[][] = []
  for (let index = 0; index < items.length; index += size) {
    rows.push(items.slice(index, index + size))
  }
  return rows
}

export function splitBeanListHighlight(text = '', terms: string[] = []): HighlightPart[] {
  const needles = [...new Set(terms.map((term) => term.trim()).filter(Boolean))].sort((a, b) => b.length - a.length)
  if (!text || needles.length === 0) return text ? [{ text, red: false }] : []
  const parts: HighlightPart[] = []
  let index = 0
  while (index < text.length) {
    let match = ''
    let matchIndex = text.length
    for (const term of needles) {
      const found = text.indexOf(term, index)
      if (found >= 0 && found < matchIndex) {
        match = term
        matchIndex = found
      }
    }
    if (!match) {
      parts.push({ text: text.slice(index), red: false })
      break
    }
    if (matchIndex > index) {
      parts.push({ text: text.slice(index, matchIndex), red: false })
    }
    parts.push({ text: match, red: true })
    index = matchIndex + match.length
  }
  return parts
}

export function beanListQualityLines(item?: BeanListProductSummary | null): BeanListQualityLine[] {
  const quality = item?.bean_list_quality
  if (!quality) return []
  return [
    { label: '工厂风味', value: stringField(quality.factory_flavor_description) },
    { label: '水分', value: stringField(quality.moisture) },
    { label: '密度', value: stringField(quality.density) },
    { label: '质检时间', value: stringField(quality.inspection_created_at) },
    { label: '质检单号', value: stringField(quality.inspection_reference_no) },
  ].filter((line) => line.value)
}

function clampCardsPerRow(value: number): number {
  if (!Number.isFinite(value)) return 1
  const rounded = Math.floor(value)
  if (rounded < 1) return 1
  if (rounded > 4) return 4
  return rounded
}

function stringField(value: unknown): string {
  return String(value ?? '').trim()
}
