export function keepElementHorizontallyVisible(container, item) {
  if (!container || !item) return

  const containerRect = container.getBoundingClientRect()
  const itemRect = item.getBoundingClientRect()
  const visibleLeft = containerRect.left + (container.clientLeft || 0)
  const visibleRight = visibleLeft + container.clientWidth

  if (itemRect.left < visibleLeft) {
    container.scrollLeft = Math.max(0, container.scrollLeft - (visibleLeft - itemRect.left))
  } else if (itemRect.right > visibleRight) {
    container.scrollLeft += itemRect.right - visibleRight
  }
}
