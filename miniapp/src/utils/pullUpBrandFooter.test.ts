import { existsSync, readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function source(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

const transientPage = 'pages/index/index'
const pages = (JSON.parse(source('src/pages.json')) as { pages: Array<{ path: string }> }).pages
  .map((page) => page.path)
  .filter((path) => path !== transientPage)

describe('pull-up brand footer', () => {
  it('covers every real miniapp page without changing the transient startup page', () => {
    expect(pages).toHaveLength(13)

    for (const page of pages) {
      const pageSource = source(`src/${page}.vue`)
      expect(pageSource, page).toContain("import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'")
      expect(pageSource, page).toContain('<PullUpBrandFooter')
      expect(pageSource, page).toContain('pull-up-brand-page')
    }

    expect(source(`src/${transientPage}.vue`)).not.toContain('PullUpBrandFooter')
  })

  it('keeps the silver signature in document flow behind a pull-up reveal spacer', () => {
    const componentPath = resolve('src/components/PullUpBrandFooter.vue')
    expect(existsSync(componentPath)).toBe(true)
    if (!existsSync(componentPath)) return

    const component = readFileSync(componentPath, 'utf8')
    expect(component).toContain('Drived By')
    expect(component).toContain('/static/branding/kefan-wordmark-silver.png')
    expect(component).toContain('pull-up-brand-reveal-spacer')
    expect(component).toContain('with-fixed-tabbar')
    expect(component).toContain('safe-area-inset-bottom')
    expect(component).not.toMatch(/position\s*:\s*fixed/)

    const app = source('src/App.vue')
    expect(app).toContain('.pull-up-brand-page')
    expect(app).toContain('.pull-up-brand-page-with-tabbar')
    expect(app).toContain('display: flex')
    expect(app).toContain('min-height: calc(100vh + 64rpx + env(safe-area-inset-bottom))')
  })

  it('uses a small transparent PNG wordmark suitable for the miniapp package', () => {
    const assetPath = resolve('src/static/branding/kefan-wordmark-silver.png')
    expect(existsSync(assetPath)).toBe(true)
    if (!existsSync(assetPath)) return

    const png = readFileSync(assetPath)
    expect(png.subarray(0, 8).toString('hex')).toBe('89504e470d0a1a0a')
    const width = png.readUInt32BE(16)
    const height = png.readUInt32BE(20)
    const colorType = png.readUInt8(25)

    expect(width).toBeLessThanOrEqual(480)
    expect(height).toBeLessThanOrEqual(180)
    expect([4, 6]).toContain(colorType)
    expect(statSync(assetPath).size).toBeLessThan(32 * 1024)
  })
})
