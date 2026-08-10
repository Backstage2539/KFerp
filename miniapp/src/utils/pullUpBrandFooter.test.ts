import { existsSync, readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { createHash } from 'node:crypto'
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
      expect(pageSource, page).toMatch(
        /<view class="pull-up-brand-footer-anchor">\s*<PullUpBrandFooter[\s\S]*?\/>\s*<\/view>/,
      )
    }

    expect(source(`src/${transientPage}.vue`)).not.toContain('PullUpBrandFooter')
  })

  it('keeps the complete silver signature box below the first viewport', () => {
    const componentPath = resolve('src/components/PullUpBrandFooter.vue')
    expect(existsSync(componentPath)).toBe(true)
    if (!existsSync(componentPath)) return

    const component = readFileSync(componentPath, 'utf8')
    expect(component).toContain('Drived By')
    expect(component).toContain('/static/branding/kefan-wordmark-silver.png')
    expect(component).toContain('pull-up-brand-reveal-spacer')
    expect(component).toContain('with-fixed-tabbar')
    expect(component).toContain('safe-area-inset-bottom')
    expect(component).toMatch(/\.pull-up-brand-reveal-spacer\s*\{[^}]*min-height:\s*104rpx/s)
    expect(component).toMatch(/\.pull-up-brand-signature\s*\{[^}]*min-height:\s*58rpx/s)
    expect(component).toMatch(/\.with-fixed-tabbar\s+\.pull-up-brand-bottom-clearance\s*\{[^}]*166rpx/s)
    expect(component).not.toMatch(/position\s*:\s*(?:fixed|absolute|sticky)/)

    const app = source('src/App.vue')
    expect(app).toContain('.pull-up-brand-page')
    expect(app).toContain('.pull-up-brand-page-with-tabbar')
    expect(app).toContain('display: flex')
    expect(app).toContain('box-sizing: content-box !important')
    expect(app).toContain('min-height: calc(100vh + 162rpx + env(safe-area-inset-bottom))')
    expect(app).toContain('min-height: calc(100vh + 328rpx + env(safe-area-inset-bottom))')
    expect(app).toMatch(
      /\.pull-up-brand-page\s*>\s*\.pull-up-brand-footer-anchor\s*\{[^}]*margin-top:\s*auto/s,
    )
    expect(app).toMatch(
      /\.pull-up-brand-page\s*>\s*\.pull-up-brand-footer-anchor\s*\{[^}]*order:\s*999/s,
    )
    expect(app).not.toContain('100vh + 64rpx')
    expect(app).not.toContain('100vh + 230rpx')
    expect(app).not.toMatch(/position\s*:\s*(?:fixed|absolute|sticky)/)
  })

  it('uses a legible exact Chinese wordmark and a small transparent PNG', () => {
    const assetPath = resolve('src/static/branding/kefan-wordmark-silver.png')
    const sourcePath = resolve('scripts/assets/kefan-wordmark-silver.svg')
    expect(existsSync(assetPath)).toBe(true)
    expect(existsSync(sourcePath)).toBe(true)
    if (!existsSync(assetPath) || !existsSync(sourcePath)) return

    const png = readFileSync(assetPath)
    const wordmarkSource = readFileSync(sourcePath, 'utf8')
    expect(wordmarkSource).toContain('>棵凡咖啡</text>')
    expect(createHash('sha256').update(png).digest('hex')).toBe(
      '8cb4f61def4cbf8cc96f03aa584283f92ed3380b81138c29ca51e2ff651c91ab',
    )
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
