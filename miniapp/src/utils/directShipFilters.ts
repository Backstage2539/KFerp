import { normalizeDateRange } from './orderFilters'

export type DirectShipDatePreset = 'today' | 'last3' | 'last7' | 'month'

export type DirectShipDateRange = {
  shipped_from?: string
  shipped_to?: string
}

export type DirectShipAddressLike = {
  province?: string
  city?: string
  district?: string
  detail_address?: string
  recipient_name?: string
}

const SHANGHAI_OFFSET_MS = 8 * 60 * 60 * 1000

function formatUTCDate(value: Date): string {
  const year = value.getUTCFullYear()
  const month = String(value.getUTCMonth() + 1).padStart(2, '0')
  const day = String(value.getUTCDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function directShipDatePresetRange(
  preset: DirectShipDatePreset,
  now = new Date(),
): Required<DirectShipDateRange> {
  const shanghaiNow = new Date(now.getTime() + SHANGHAI_OFFSET_MS)
  const year = shanghaiNow.getUTCFullYear()
  const month = shanghaiNow.getUTCMonth()
  const day = shanghaiNow.getUTCDate()
  const end = new Date(Date.UTC(year, month, day))
  const start = new Date(end)

  if (preset === 'last3') start.setUTCDate(start.getUTCDate() - 2)
  if (preset === 'last7') start.setUTCDate(start.getUTCDate() - 6)
  if (preset === 'month') start.setUTCDate(1)

  return {
    shipped_from: formatUTCDate(start),
    shipped_to: formatUTCDate(end),
  }
}

export function normalizeDirectShipDateRange(shippedFrom?: string, shippedTo?: string): DirectShipDateRange {
  const range = normalizeDateRange(shippedFrom, shippedTo)
  return {
    ...(range.date_from ? { shipped_from: range.date_from } : {}),
    ...(range.date_to ? { shipped_to: range.date_to } : {}),
  }
}

export function directShipDestination(row: DirectShipAddressLike): string {
  const regions = [row.province, row.city, row.district]
    .map((value) => String(value || '').trim())
    .filter((value, index, values) => Boolean(value) && value !== values[index - 1])
  return regions.join('') || String(row.detail_address || '').trim()
}

export function directShipRequestTitle(row: DirectShipAddressLike): string {
  const destination = directShipDestination(row) || '目的地待完善'
  const recipient = String(row.recipient_name || '').trim() || '收件人待完善'
  return `${destination} · ${recipient}`
}
