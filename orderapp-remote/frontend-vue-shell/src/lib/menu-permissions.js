export function actorHasFullViewAccess(actor) {
  if (!actor) return false
  if (actor.basic_auth_admin) return true
  return Array.isArray(actor.roles) && actor.roles.some((role) => String(role?.code || '').toLowerCase() === 'admin')
}

export function isCustomerAccountMode(actor) {
  if (!actor) return false
  if (String(actor.account_type || '').trim().toLowerCase() === 'channel_customer') return true
  if (actor.customer_account_mode === true) return true
  return Array.isArray(actor.allowed_views) &&
    actor.allowed_views.includes('customerProcessingPortal') &&
    !actorHasFullViewAccess(actor)
}

export function isViewAllowed(key, allowedViews) {
  if (!Array.isArray(allowedViews)) return true
  return allowedViews.includes(key)
}

function isCustomerWorkspaceMode(options = {}) {
  return String(options.workspaceMode || '').trim().toLowerCase() === 'customer' || isCustomerAccountMode(options.actor)
}

export function filterMenuGroups(groups, allowedViews, options = {}) {
  const hideCustomerFulfillment = !!options.hideCustomerAccountFulfillment && isCustomerWorkspaceMode(options)
  return (groups || [])
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => {
        if (hideCustomerFulfillment && item.key === 'customerFulfillment') return false
        if (!isViewAllowed(item.key, allowedViews)) return false
        return true
      }),
    }))
    .filter((group) => group.items.length > 0)
}
