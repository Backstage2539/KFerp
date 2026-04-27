export function relativeURLForHistory(url) {
  const parsed = url instanceof URL ? url : new URL(String(url), window.location.href)
  return `${parsed.pathname}${parsed.search}${parsed.hash}`
}

export function replaceHistoryURL(url) {
  window.history.replaceState({}, '', relativeURLForHistory(url))
}
