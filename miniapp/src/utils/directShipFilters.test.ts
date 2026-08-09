import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  directShipDatePresetRange,
  directShipDestination,
  directShipRequestTitle,
  normalizeDirectShipDateRange,
} from './directShipFilters'

describe('direct shipment center helpers', () => {
  const today = new Date('2026-08-07T09:30:00+08:00')

  it('builds real shipment-time presets and normalizes a custom range', () => {
    expect(directShipDatePresetRange('today', today)).toEqual({ shipped_from: '2026-08-07', shipped_to: '2026-08-07' })
    expect(directShipDatePresetRange('last3', today)).toEqual({ shipped_from: '2026-08-05', shipped_to: '2026-08-07' })
    expect(directShipDatePresetRange('last7', today)).toEqual({ shipped_from: '2026-08-01', shipped_to: '2026-08-07' })
    expect(directShipDatePresetRange('month', today)).toEqual({ shipped_from: '2026-08-01', shipped_to: '2026-08-07' })
    expect(normalizeDirectShipDateRange('2026-08-07', '2026-08-03')).toEqual({ shipped_from: '2026-08-03', shipped_to: '2026-08-07' })
    expect(normalizeDirectShipDateRange('bad', '2026-08-07')).toEqual({ shipped_to: '2026-08-07' })
  })

  it('uses the Shanghai calendar date regardless of the device timezone', () => {
    const shanghaiNextDay = new Date('2026-08-07T23:30:00-07:00')
    expect(directShipDatePresetRange('today', shanghaiNextDay)).toEqual({
      shipped_from: '2026-08-08',
      shipped_to: '2026-08-08',
    })
  })

  it('uses destination and recipient as the customer-facing card title', () => {
    const row = {
      province: '云南省',
      city: '普洱市',
      district: '思茅区',
      detail_address: '咖啡路 88 号',
      recipient_name: '张三',
    }
    expect(directShipDestination(row)).toBe('云南省普洱市思茅区')
    expect(directShipRequestTitle(row)).toBe('云南省普洱市思茅区 · 张三')
    expect(directShipRequestTitle({ detail_address: '上海市徐汇区龙华路 1 号', recipient_name: '李四' })).toBe('上海市徐汇区龙华路 1 号 · 李四')
    expect(directShipRequestTitle({ detail_address: '', recipient_name: '' })).toBe('目的地待完善 · 收件人待完善')
  })

  it('wires the shipment center to server search, shipment dates, and pagination', () => {
    const page = readFileSync(resolve('src/components/CustomerDirectShipPanel.vue'), 'utf8')

    expect(page).toContain('directShipRequestTitle(item)')
    expect(page).not.toContain('{{ item.request_no }}')
    expect(page).toContain('搜索收件客户/公司、收件人、电话、目的地')
    expect(page).toContain("applyDatePreset('today')")
    expect(page).toContain("applyDatePreset('last3')")
    expect(page).toContain("applyDatePreset('last7')")
    expect(page).toContain("applyDatePreset('month')")
    expect(page).toContain('shipped_from: shippedFrom.value')
    expect(page).toContain('shipped_to: shippedTo.value')
    expect(page).toContain('共 {{ totalRows }} 条 · 共 {{ totalPages }} 页')
    expect(page).toContain('第 {{ currentPage }} / {{ totalPages }} 页')
    expect(page).toContain('上一页')
    expect(page).toContain('下一页')
    expect(page).toContain('跳页')
    expect(page).toContain('每页 {{ pageLimit }} 条')
    expect(page).toContain('let loadVersion = 0')
    expect(page).toContain('if (version !== loadVersion) return')
  })
})
