export const salesDocumentPageWidthMM = 210
export const salesDocumentSealHeightRatio = 1

export function salesSealMMToPDFPlacement(seal = {}, page = {}, options = {}) {
  const pointPerMM = pdfPointsPerMM(page)
  const widthMM = positiveNumber(seal.width_mm ?? seal.seal_width_mm, 36)
  const sealAspectRatio = normalizePDFStampAspectRatio(
    options.sealAspectRatio ?? seal.aspect_ratio ?? seal.seal_aspect_ratio,
    salesDocumentSealHeightRatio,
  )
  return {
    page_number: Number(page.pageNumber || page.page_number || seal.page_number || 1),
    x: round2(nonNegativeNumber(seal.x_mm ?? seal.seal_x_mm, 32) * pointPerMM),
    y: round2(nonNegativeNumber(seal.y_mm ?? seal.seal_y_mm, 5) * pointPerMM),
    width: round2(widthMM * pointPerMM),
    height: round2(widthMM * sealAspectRatio * pointPerMM),
  }
}

export function pdfPlacementToSalesSealMM(placement = {}, page = {}) {
  const pointPerMM = pdfPointsPerMM(page)
  return {
    seal_x_mm: Math.round(nonNegativeNumber(placement.x, 0) / pointPerMM),
    seal_y_mm: Math.round(nonNegativeNumber(placement.y, 0) / pointPerMM),
    seal_width_mm: Math.round(positiveNumber(placement.width, 0) / pointPerMM),
  }
}

export function movePDFStampPlacement(placement = {}, { deltaX = 0, deltaY = 0, displayScale = 1 } = {}) {
  const scale = positiveNumber(displayScale, 1)
  return {
    page_number: Number(placement.page_number || 1),
    x: round2(Math.max(0, Number(placement.x || 0) + Number(deltaX || 0) / scale)),
    y: round2(Math.max(0, Number(placement.y || 0) + Number(deltaY || 0) / scale)),
    width: round2(Number(placement.width || 0)),
    height: round2(Number(placement.height || 0)),
  }
}

export function scalePDFStampPlacement(placement = {}, { width, sealAspectRatio } = {}) {
  const nextWidth = positiveNumber(width, positiveNumber(placement.width, 0))
  const nextAspectRatio = normalizePDFStampAspectRatio(
    sealAspectRatio ?? (positiveNumber(placement.width, 0) ? Number(placement.height || 0) / Number(placement.width || 1) : undefined),
    salesDocumentSealHeightRatio,
  )
  return {
    page_number: Number(placement.page_number || 1),
    x: round2(Number(placement.x || 0)),
    y: round2(Number(placement.y || 0)),
    width: round2(nextWidth),
    height: round2(nextWidth * nextAspectRatio),
  }
}

export function pdfPointsPerMM(page = {}) {
  return positiveNumber(page.pageWidth ?? page.width, 595.28) / salesDocumentPageWidthMM
}

export function normalizePDFStampAspectRatio(value, fallback = salesDocumentSealHeightRatio) {
  const n = Number(value)
  const fallbackNumber = positiveNumber(fallback, salesDocumentSealHeightRatio)
  return Number.isFinite(n) && n > 0 ? n : fallbackNumber
}

function nonNegativeNumber(value, fallback) {
  const n = Number(value)
  return Number.isFinite(n) && n >= 0 ? n : fallback
}

function positiveNumber(value, fallback) {
  const n = Number(value)
  return Number.isFinite(n) && n > 0 ? n : fallback
}

function round2(value) {
  const n = Number(value || 0)
  return Math.round(n * 100) / 100
}
