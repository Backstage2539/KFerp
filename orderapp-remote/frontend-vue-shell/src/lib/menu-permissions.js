export function isViewAllowed(key, allowedViews) {
  if (!Array.isArray(allowedViews)) return true
  return allowedViews.includes(key)
}

export function filterMenuGroups(groups, allowedViews) {
  if (!Array.isArray(allowedViews)) return groups
  return groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => isViewAllowed(item.key, allowedViews)),
    }))
    .filter((group) => group.items.length > 0)
}
