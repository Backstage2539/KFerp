import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

type MiniappPageConfig = {
  path: string
}

function readSource(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('miniapp page share registration', () => {
  it('registers share menu refresh and WeChat share hooks on every declared page', () => {
    const pages = (JSON.parse(readSource('src/pages.json')) as { pages: MiniappPageConfig[] }).pages

    for (const page of pages) {
      const sourcePath = `src/${page.path}.vue`
      const source = readSource(sourcePath)

      expect(source, sourcePath).toContain('refreshMiniappShareMenu')
      expect(source, sourcePath).toContain('onShareAppMessage(defaultMiniappShare)')
      expect(source, sourcePath).toContain('onShareTimeline(defaultMiniappTimelineShare)')
    }
  })
})
