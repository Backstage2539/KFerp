export function fixedPricePath(entry) {
  return `pages/price-list/price-list?entry=${encodeURIComponent(entry.key)}`
}
export function menuToEditor(menu) {
  const source = menu?.menu || menu?.selfmenu_info || menu || { button: [] }
  return JSON.parse(JSON.stringify(source.button || [])).map((group) => {
    if (group.sub_button && !Array.isArray(group.sub_button))
      group.sub_button = group.sub_button.list || []
    return group
  })
}
export function menuFromEditor(groups) {
  return {
    button: JSON.parse(JSON.stringify(groups)).map((group) => {
      if (group.sub_button?.length)
        return { name: group.name, sub_button: group.sub_button }
      delete group.sub_button
      return group
    }),
  }
}
export function menuAction(button) {
  if (button.type === 'miniprogram') {
    if (button.pagepath?.startsWith('pages/price-list/')) return 'price'
    if (
      [
        'pages/service/service?key=orders',
        'pages/service/service?key=orders&source=official',
      ].includes(button.pagepath)
    )
      return 'orders'
    if (button.pagepath === 'pages/service/service?key=productOrder')
      return 'order'
    return button.pagepath === 'pages/index/index' ? 'home' : 'custom'
  }
  if (button.type === 'click')
    return button.key === 'ORDERS_RECENT_3'
      ? 'recent3'
      : button.key === 'ORDERS_RECENT_1'
        ? 'recent1'
        : 'custom'
  return button.type || 'view'
}
export function setMenuAction(button, action, appid, entry = '') {
  const name = button.name
  const oldURL = button.url
  for (const key of Object.keys(button)) delete button[key]
  button.name = name
  if (action.startsWith('recent')) {
    Object.assign(button, {
      type: 'click',
      key: action === 'recent3' ? 'ORDERS_RECENT_3' : 'ORDERS_RECENT_1',
    })
    return
  }
  if (action === 'view') {
    Object.assign(button, {
      type: 'view',
      url: oldURL || 'https://erp.qacoohee.com/app/',
    })
    return
  }
  const paths = {
    price: fixedPricePath({ key: entry }),
    orders: 'pages/service/service?key=orders&source=official',
    order: 'pages/service/service?key=productOrder',
    home: 'pages/index/index',
  }
  Object.assign(button, {
    type: 'miniprogram',
    appid,
    pagepath: paths[action] || paths.home,
    url: oldURL || 'https://erp.qacoohee.com/app/',
  })
}
