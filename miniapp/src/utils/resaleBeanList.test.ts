import { describe, expect, it } from 'vitest'
import type { BeanListSummary, ResaleBeanListCommand } from '../api/customerPortal'
import {
  buildResaleBeanListPublishPayload,
  defaultResaleBeanListDraft,
  resaleCardsPerRowOptions,
  resaleBeanListItemKey,
  resaleStyleColorPresets,
} from './resaleBeanList'
import {
  buildResaleBeanListEditorPath,
  buildResaleBeanListPDFPath,
  buildResaleBeanListPNGPath,
  buildResaleBeanListsPath,
} from '../api/customerPortal'

describe('customer resale bean list helpers', () => {
  const source: BeanListSummary = {
    id: 11,
    list_type: 'green',
    version_no: 'G-1',
    status: 'published',
    published_at: '2026-06-06 12:00',
    changelog: '工厂供货版本',
    cache_key: 'bean-list:11:G-1',
    title: '工厂供货豆单',
    brand_name: '棵凡咖啡',
    brand_intro: '源头工厂',
    groups: [{
      category: '生豆',
      items: [
        { code: 'ETH-G1', name: '埃塞瑰夏', prices: [{ label: '1kg+', value: '100/kg' }] },
        { code: '', name: '巴西黄波旁', prices: [{ label: '1kg+', value: '80/kg' }] },
      ],
    }],
  }

  it('creates a light editor draft from the source factory supply bean list', () => {
    const draft = defaultResaleBeanListDraft(source, 'V2')

    expect(draft.source_publication_id).toBe(11)
    expect(draft.version_no).toBe('V2')
    expect(draft.selected_item_codes).toEqual(['ETH-G1', '巴西黄波旁'])
    expect(draft.config.brandName).toBe('棵凡咖啡')
    expect(draft.config.brandIntro).toBe('源头工厂')
    expect(draft.price_rule.multiplier).toBe(1)
    expect(draft.price_rule.add_amount).toBe(0)
  })

  it('normalizes publish payload for source, template, style and markup fields', () => {
    const draft: ResaleBeanListCommand = {
      ...defaultResaleBeanListDraft(source, 'V2'),
      gradient_template_id: 5,
      selected_item_codes: ['ETH-G1'],
      price_rule: { add_amount: 2, multiplier: 1.1 },
      config: { brandName: '客户品牌', brandIntro: '销售说明', backgroundColor: '#fff8ee', layoutStyle: 'card' },
      item_overrides: [
        { code: 'ETH-G1', label: '1kg+', price: 999, badge_label: '推荐', recommended_use: '手冲', highlight_terms: ['推荐'] },
        { code: 'BRA-1', label: '1kg+', price: 888, highlight_terms: [] },
      ],
      changelog: '首版转售豆单',
    }

    const payload = buildResaleBeanListPublishPayload(draft)

    expect(payload).toEqual({
      source_publication_id: 11,
      version_no: 'V2',
      gradient_template_id: 5,
      selected_item_codes: ['ETH-G1'],
      price_rule: { add_amount: 2, multiplier: 1.1 },
      config: { brandName: '客户品牌', brandIntro: '销售说明', backgroundColor: '#fff8ee', layoutStyle: 'card' },
      item_overrides: [{ code: 'ETH-G1', badge_label: '推荐', recommended_use: '手冲', highlight_terms: ['推荐'] }],
      changelog: '首版转售豆单',
    })
  })

  it('uses preset colors and 1/2/3 card count controls instead of raw numeric/color fields', () => {
    expect(resaleStyleColorPresets.map((preset) => preset.key)).toContain('warm')
    expect(resaleStyleColorPresets.every((preset) => preset.backgroundColor.startsWith('#') && preset.fontColor.startsWith('#'))).toBe(true)
    expect(resaleCardsPerRowOptions).toEqual([1, 2, 3])
  })

  it('builds stable item keys and mini API paths', () => {
    expect(resaleBeanListItemKey({ code: ' ETH-G1 ', name: '埃塞瑰夏' })).toBe('ETH-G1')
    expect(resaleBeanListItemKey({ code: '', name: ' 巴西黄波旁 ' })).toBe('巴西黄波旁')
    expect(buildResaleBeanListsPath()).toBe('/api/mini/resale-bean-lists')
    expect(buildResaleBeanListEditorPath(11)).toBe('/api/mini/resale-bean-lists/11/editor')
    expect(buildResaleBeanListPDFPath(33)).toBe('/api/mini/resale-bean-lists/33.pdf')
    expect(buildResaleBeanListPNGPath(33)).toBe('/api/mini/resale-bean-lists/33.png')
  })
})
