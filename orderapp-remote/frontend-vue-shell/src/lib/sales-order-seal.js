const defaultSealXMM = 32
const defaultSealYMM = 5
const defaultSealWidthMM = 36

export const salesOrderSealMinWidthMM = 20
export const salesOrderSealMaxWidthMM = 120
export const salesOrderPreviewDesignWidthPX = 1240
export const salesOrderPreviewPageWidthMM = 210
export const salesOrderSealPreviewScale = salesOrderPreviewDesignWidthPX / salesOrderPreviewPageWidthMM

export function normalizeSalesOrderSeal(seal = {}) {
  const x = Number(seal.x_mm ?? seal.seal_x_mm ?? defaultSealXMM)
  const y = Number(seal.y_mm ?? seal.seal_y_mm ?? defaultSealYMM)
  const width = Number(seal.width_mm ?? seal.seal_width_mm ?? defaultSealWidthMM)
  return {
    x_mm: Number.isFinite(x) ? x : defaultSealXMM,
    y_mm: Number.isFinite(y) ? y : defaultSealYMM,
    width_mm: Number.isFinite(width) && width > 0 ? width : defaultSealWidthMM,
  }
}

export function salesOrderSealStyle(seal = {}, scale = salesOrderSealPreviewScale) {
  const pos = normalizeSalesOrderSeal(seal)
  const width = Math.max(salesOrderSealMinWidthMM, pos.width_mm)
  return {
    left: `${Math.max(0, pos.x_mm) * scale}px`,
    top: `${Math.max(0, pos.y_mm) * scale}px`,
    width: `${width * scale}px`,
    height: `${width * 0.62 * scale}px`,
  }
}

export function beginSalesOrderSealDrag({ seal = {}, clientX = 0, clientY = 0, scale = salesOrderSealPreviewScale } = {}) {
  const pos = normalizeSalesOrderSeal(seal)
  return {
    originXMM: pos.x_mm,
    originYMM: pos.y_mm,
    widthMM: pos.width_mm,
    startClientX: Number(clientX) || 0,
    startClientY: Number(clientY) || 0,
    scale: Number(scale) > 0 ? Number(scale) : salesOrderSealPreviewScale,
  }
}

export function moveSalesOrderSealDrag(state, { clientX = 0, clientY = 0 } = {}) {
  if (!state) {
    return normalizeSalesOrderSeal()
  }
  const scale = Number(state.scale) > 0 ? Number(state.scale) : salesOrderSealPreviewScale
  return {
    x_mm: Math.max(0, Math.round(Number(state.originXMM || 0) + (Number(clientX || 0) - Number(state.startClientX || 0)) / scale)),
    y_mm: Math.max(0, Math.round(Number(state.originYMM || 0) + (Number(clientY || 0) - Number(state.startClientY || 0)) / scale)),
    width_mm: Number(state.widthMM || defaultSealWidthMM),
  }
}
