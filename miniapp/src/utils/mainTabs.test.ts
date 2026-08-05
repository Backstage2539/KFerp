import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function readSource(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('miniapp startup route and main tabs', () => {
  it('uses an index startup page as the first published miniapp route', () => {
    const pages = JSON.parse(readSource('src/pages.json')) as { pages: { path: string }[] }

    expect(pages.pages[0]?.path).toBe('pages/index/index')
    expect(pages.pages.map((page) => page.path)).toEqual(
      expect.arrayContaining([
        'pages/home/home',
        'pages/service/service',
        'pages/factory-products/factory-products',
        'pages/customer-products/customer-products',
        'pages/price-table-settings/price-table-settings',
        'pages/employee-customers/employee-customers',
        'pages/profile/profile',
      ]),
    )
  })

  it('adds employee customer maintenance to the simple ERP home', () => {
    const home = readSource('src/pages/home/home.vue')
    const customerPage = readSource('src/pages/employee-customers/employee-customers.vue')
    const editor = readSource('src/components/EmployeeCustomerEditor.vue')

    expect(home).toContain("label: '客户维护'")
    expect(home).toContain('/pages/employee-customers/employee-customers')
    expect(home).toContain("session.permissions.includes('customers.read')")
    expect(home).toContain("session.permissions.includes('customers.write')")
    expect(customerPage).toContain('EmployeeCustomerEditor')
    expect(customerPage).toContain('fetchEmployeeCustomers')
    expect(customerPage).toContain("context?.is_admin ? '可维护全部客户' : '仅显示并维护本人负责的客户'")
    expect(editor).toContain('v-if="isAdmin"')
    expect(editor).toContain('负责人')
    expect(editor).toContain("isAdmin.value && !Number(form.responsible_employee_id || 0)")
    expect(editor).toContain('允许登录客户门户')
  })

  it('lets employees open a personal center and leave a persisted employee session', () => {
    const home = readSource('src/pages/home/home.vue')
    const profile = readSource('src/pages/profile/profile.vue')

    expect(home).toContain("{ key: 'employeeProfile', label: '个人中心', url: '/pages/profile/profile' }")
    expect(profile).toContain("const isEmployee = computed(() => session.accountType === 'employee')")
    expect(profile).toContain("session.employeeName || '员工'")
    expect(profile).toContain('当前员工')
    expect(profile).toContain('v-if="!isEmployee"')
    expect(profile).toContain('<MainTabBar v-if="!isEmployee" current="mine" />')
    expect(profile).toContain('切换用户')
    expect(profile).toContain('退出登录')
  })

  it('shows the global image-share entrance switch only to employee administrators', () => {
    const profile = readSource('src/pages/profile/profile.vue')
    const api = readSource('src/api/customerPortal.ts')

    expect(profile).toContain("session.accountType === 'employee'")
    expect(profile).toContain("session.roles.includes('admin')")
    expect(profile).toContain("session.permissions.includes('settings.write')")
    expect(profile).toContain('v-if="canManageShareSettings"')
    expect(profile).toContain('分享图片时携带小程序入口')
    expect(profile).toContain('fetchEmployeeShareSettings')
    expect(profile).toContain('saveEmployeeShareSettings')
    expect(profile).toContain('isAuthenticationExpiredRequestError')
    expect(profile).toContain('redirectExpiredShareSettingsSession(error)')
    expect(profile).toContain('clearAndLogin()')
    expect(api).toContain("'/api/mini/employee/share-settings'")
  })

  it('routes startup users through reLaunch instead of leaving a blank page', () => {
    const index = readSource('src/pages/index/index.vue')

    expect(index).toContain('useSessionStore')
    expect(index).toContain("uni.reLaunch({ url: '/pages/login/login' })")
    expect(index).toContain("uni.reLaunch({ url: '/pages/home/home' })")
    expect(index).not.toContain('redirectTo')
  })

  it('revalidates a persisted session before rendering its employee or customer persona', () => {
    const index = readSource('src/pages/index/index.vue')
    const home = readSource('src/pages/home/home.vue')

    expect(index).toContain("uni.reLaunch({ url: '/pages/home/home' })")
    expect(home).toContain('fetchMe(session.token)')
    expect(home).toContain('session.applyContext(response)')
    expect(home).toContain('session.clearSession()')
    expect(home).toContain("uni.reLaunch({ url: '/pages/login/login' })")
  })

  it('renders four bottom main entries on authenticated top-level pages', () => {
    const tabBar = readSource('src/components/MainTabBar.vue')
    const pages = [
      readSource('src/pages/home/home.vue'),
      readSource('src/pages/mall/mall.vue'),
      readSource('src/pages/service/service.vue'),
      readSource('src/pages/profile/profile.vue'),
    ]

    for (const label of ['首页', '订单中心', '费用中心', '个人中心']) {
      expect(tabBar).toContain(label)
    }
    for (const url of [
      '/pages/home/home',
      '/pages/service/service?key=orders',
      '/pages/service/service?key=settlement',
      '/pages/profile/profile',
    ]) {
      expect(tabBar).toContain(url)
    }
    expect(tabBar).toContain('uni.reLaunch')
    expect(tabBar).toContain('display: flex')
    expect(tabBar).not.toContain('display: grid')
    for (const page of pages) {
      expect(page).toContain('MainTabBar')
    }
  })

  it('lets customers open sales order and outbound note PDFs from the order tab', () => {
    const servicePage = readSource('src/pages/service/service.vue')
    const api = readSource('src/api/customerPortal.ts')

    expect(api).toContain('sales_order_url?: string')
    expect(api).toContain('delivery_note_url?: string')
    expect(servicePage).toContain('销售单')
    expect(servicePage).toContain('出库单')
    expect(servicePage).toContain('openOrderDocument')
    expect(servicePage).toContain('uni.downloadFile')
    expect(servicePage).toContain('uni.openDocument')
    expect(servicePage).toContain('Authorization: `Bearer ${session.token}`')
  })

  it('places my products in profile instead of the home shortcuts', () => {
    const capabilities = readSource('src/utils/capabilities.ts')
    const profile = readSource('src/pages/profile/profile.vue')

    expect(capabilities).not.toContain('beanList')
    expect(capabilities).not.toContain('我的商品')
    expect(profile).toContain('工厂商品表')
    expect(profile).toContain('我的商品')
    expect(profile).toContain('/pages/factory-products/factory-products')
    expect(profile).toContain('/pages/customer-products/customer-products')
  })

  it('splits factory product tables, my products, and price table settings into focused pages', () => {
    const factoryPage = readSource('src/pages/factory-products/factory-products.vue')
    const customerPage = readSource('src/pages/customer-products/customer-products.vue')
    const settingsPage = readSource('src/pages/price-table-settings/price-table-settings.vue')
    const servicePage = readSource('src/pages/service/service.vue')

    expect(factoryPage).toContain('工厂商品表')
    expect(factoryPage).toContain('factory_price_table_groups')
    expect(factoryPage).toContain('PDF')
    expect(factoryPage).toContain('长图')
    expect(factoryPage).toContain('openBeanListOutput')

    expect(customerPage).toContain('我的商品')
    expect(customerPage).toContain('已发布商品价格表')
    expect(customerPage).toContain('价格表设置')
    expect(customerPage).toContain('/pages/price-table-settings/price-table-settings')
    expect(customerPage).not.toContain('统一加价')
    expect(customerPage).not.toContain('倍率加价')

    expect(settingsPage).toContain('复制来源')
    expect(settingsPage).toContain('工厂价格表')
    expect(settingsPage).toContain('我的已发布价格表')
    expect(settingsPage).toContain('商品配置')
    expect(settingsPage).toContain('标红词')
    expect(settingsPage).toContain('发布商品价格表')

    for (const source of [factoryPage, customerPage, settingsPage, servicePage]) {
      expect(source).not.toContain('预览 PDF')
      expect(source).not.toContain('预览长图')
      expect(source).not.toContain('覆盖档位')
      expect(source).not.toContain('单品价')
      expect(source).not.toContain('placeholder="背景色 #f8f1e5"')
      expect(source).not.toContain('placeholder="每行卡片数"')
    }
  })

  it('removes redundant top profile links from content pages', () => {
    const employeeHome = readSource('src/pages/home/home.vue')
    expect(employeeHome).not.toContain('profile-link')
    expect(employeeHome).not.toContain('openProfile')

    for (const path of ['src/pages/mall/mall.vue', 'src/pages/service/service.vue']) {
      const source = readSource(path)
      expect(source).not.toContain('profile-link')
      expect(source).not.toContain('openProfile')
      expect(source).not.toContain('个人中心')
    }
  })
})
