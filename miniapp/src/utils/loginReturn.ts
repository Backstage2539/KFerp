export function safeLoginReturn(raw: unknown): string {
  const path = typeof raw === 'string' ? raw : ''
  if (/^\/pages\/price-list\/price-list\?entry=[a-zA-Z0-9_-]{1,64}$/.test(path))
    return path
  if (
    /^\/pages\/service\/service\?key=(orders|productOrder|directShip)$/.test(
      path,
    )
  )
    return path
  if (path === '/pages/service/service?key=orders&source=official') return path
  if (path === '/pages/profile/profile') return path
  return ''
}
export function loginRouteFor(destination: string): string {
  const safe = safeLoginReturn(destination)
  return `/pages/login/login${safe ? `?return_to=${encodeURIComponent(safe)}` : ''}`
}
