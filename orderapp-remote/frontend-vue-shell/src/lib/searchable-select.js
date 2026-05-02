export function normalizeSearchableText(value) {
  return String(value || '').trim().toLowerCase().replace(/\s+/g, ' ')
}

export function defaultOptionLabel(option) {
  return String(option?.name || option?.Name || option?.label || '').trim()
}

export function optionSearchText(option, label = '') {
  return normalizeSearchableText([
    label,
    option?.name,
    option?.Name,
    option?.label,
    option?.code,
    option?.Code,
    option?.py,
    option?.pyi,
    option?.sku,
    option?.origin,
    option?.Origin,
    option?.supplier,
    option?.Supplier,
    option?.batch_code,
  ].filter(Boolean).join(' '))
}

export function filterSearchableOptions(options, query, labelOf = defaultOptionLabel) {
  const terms = normalizeSearchableText(query).split(' ').filter(Boolean)
  if (!terms.length) return options || []
  return (options || []).filter((option) => {
    const haystack = optionSearchText(option, labelOf(option))
    return terms.every((term) => haystack.includes(term))
  })
}
