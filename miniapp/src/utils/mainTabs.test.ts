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
        'pages/profile/profile',
      ]),
    )
  })

  it('routes startup users through reLaunch instead of leaving a blank page', () => {
    const index = readSource('src/pages/index/index.vue')

    expect(index).toContain('useSessionStore')
    expect(index).toContain("uni.reLaunch({ url: '/pages/login/login' })")
    expect(index).toContain("uni.reLaunch({ url: '/pages/home/home' })")
    expect(index).not.toContain('redirectTo')
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

  it('removes top profile links from content pages', () => {
    for (const path of ['src/pages/home/home.vue', 'src/pages/mall/mall.vue', 'src/pages/service/service.vue']) {
      const source = readSource(path)
      expect(source).not.toContain('profile-link')
      expect(source).not.toContain('openProfile')
      expect(source).not.toContain('个人中心')
    }
  })
})
