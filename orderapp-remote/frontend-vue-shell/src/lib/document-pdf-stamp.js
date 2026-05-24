export const salesDocumentPageWidthMM = 210
export const salesDocumentPageTopMarginMM = 14
export const salesDocumentPageBottomMarginMM = 18
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
    ...placement,
    page_number: Number(placement.page_number || 1),
    x: round2(Math.max(0, Number(placement.x || 0) + Number(deltaX || 0) / scale)),
    y: round2(Math.max(0, Number(placement.y || 0) + Number(deltaY || 0) / scale)),
    width: round2(Number(placement.width || 0)),
    height: round2(Number(placement.height || 0)),
  }
}

export function movePDFStampPlacementAcrossPages(placement = {}, { deltaX = 0, deltaY = 0, displayScale = 1, pages = [] } = {}) {
  const validPages = Array.isArray(pages) ? pages.filter(Boolean) : []
  if (validPages.length <= 1) return movePDFStampPlacement(placement, { deltaX, deltaY, displayScale })
  const scale = positiveNumber(displayScale, 1)
  let pageIndex = validPages.findIndex((page) => Number(page.pageNumber || page.page_number || 1) === Number(placement.page_number || 1))
  if (pageIndex < 0) pageIndex = 0
  let y = Number(placement.y || 0) + Number(deltaY || 0) / scale
  while (y < 0 && pageIndex > 0) {
    pageIndex -= 1
    y += pdfPageHeight(validPages[pageIndex])
  }
  while (pageIndex < validPages.length - 1 && y + Number(placement.height || 0) > pdfPageHeight(validPages[pageIndex])) {
    y -= pdfPageHeight(validPages[pageIndex])
    pageIndex += 1
  }
  const pageHeight = pdfPageHeight(validPages[pageIndex])
  const maxY = Math.max(0, pageHeight - Number(placement.height || 0))
  return {
    ...placement,
    page_number: Number(validPages[pageIndex].pageNumber || validPages[pageIndex].page_number || pageIndex + 1),
    x: round2(Math.max(0, Number(placement.x || 0) + Number(deltaX || 0) / scale)),
    y: round2(Math.min(maxY, Math.max(0, y))),
    width: round2(Number(placement.width || 0)),
    height: round2(Number(placement.height || 0)),
  }
}

export function resizePDFStampPlacement(placement = {}, { deltaX = 0, deltaY = 0, displayScale = 1, minWidth = 24, minHeight = 24, lockAspectRatio = false } = {}) {
  const scale = positiveNumber(displayScale, 1)
  const width = Math.max(positiveNumber(minWidth, 24), Number(placement.width || 0) + Number(deltaX || 0) / scale)
  const aspectRatio = positiveNumber(placement.height, 1) / positiveNumber(placement.width, 1)
  const height = lockAspectRatio
    ? width * aspectRatio
    : Math.max(positiveNumber(minHeight, 24), Number(placement.height || 0) + Number(deltaY || 0) / scale)
  return {
    ...placement,
    page_number: Number(placement.page_number || 1),
    x: round2(Math.max(0, Number(placement.x || 0))),
    y: round2(Math.max(0, Number(placement.y || 0))),
    width: round2(width),
    height: round2(height),
  }
}

export function scalePDFStampPlacement(placement = {}, { width, sealAspectRatio } = {}) {
  const nextWidth = positiveNumber(width, positiveNumber(placement.width, 0))
  const nextAspectRatio = normalizePDFStampAspectRatio(
    sealAspectRatio ?? (positiveNumber(placement.width, 0) ? Number(placement.height || 0) / Number(placement.width || 1) : undefined),
    salesDocumentSealHeightRatio,
  )
  return {
    ...placement,
    page_number: Number(placement.page_number || 1),
    x: round2(Number(placement.x || 0)),
    y: round2(Number(placement.y || 0)),
    width: round2(nextWidth),
    height: round2(nextWidth * nextAspectRatio),
  }
}

export function salesLayoutBoxMMToPDFPlacement(box = {}, page = {}, options = {}) {
  const pointPerMM = pdfPointsPerMM(page)
  return {
    ...options,
    page_number: Number(page.pageNumber || page.page_number || options.page_number || 1),
    x: round2(nonNegativeNumber(box.x_mm ?? box.XMM, 0) * pointPerMM),
    y: round2(nonNegativeNumber(box.y_mm ?? box.YMM, 0) * pointPerMM),
    width: round2(positiveNumber(box.width_mm ?? box.WidthMM, 1) * pointPerMM),
    height: round2(positiveNumber(box.height_mm ?? box.HeightMM, 1) * pointPerMM),
  }
}

export function salesLayoutBoxMMToPDFPreviewPlacement(box = {}, pages = [], options = {}) {
  const page = salesLayoutBoxPreviewPage(pages, box.page_number ?? box.pageNumber ?? options.page_number)
  const fittedBox = fitSalesLayoutBoxWithinPDFPreviewPage(box, page)
  return salesLayoutBoxMMToPDFPlacement(fittedBox, page, options)
}

export function salesLayoutBoxPreviewPage(pages = [], preferredPageNumber) {
  const validPages = Array.isArray(pages) ? pages.filter(Boolean) : []
  if (validPages.length === 0) return {}
  const preferredPage = validPages.find((page) => Number(page.pageNumber || page.page_number || 1) === Number(preferredPageNumber || 0))
  if (preferredPage) return preferredPage
  return validPages.length > 1 ? validPages[validPages.length - 1] : validPages[0]
}

export function fitSalesLayoutBoxWithinPDFPreviewPage(box = {}, page = {}) {
  const pointPerMM = pdfPointsPerMM(page)
  const pageHeightMM = positiveNumber(page.pageHeight ?? page.height, 841.89) / pointPerMM
  const topMarginMM = salesDocumentPageTopMarginMM
  const maxBottomMM = pageHeightMM - salesDocumentPageBottomMarginMM
  const maxHeightMM = Math.max(1, maxBottomMM - topMarginMM)
  const heightMM = Math.min(positiveNumber(box.height_mm ?? box.HeightMM, 1), maxHeightMM)
  let yMM = nonNegativeNumber(box.y_mm ?? box.YMM, 0)
  if (yMM + heightMM > maxBottomMM) yMM = maxBottomMM - heightMM
  if (yMM < topMarginMM) yMM = topMarginMM
  return {
    x_mm: nonNegativeNumber(box.x_mm ?? box.XMM, 0),
    y_mm: round2(yMM),
    width_mm: positiveNumber(box.width_mm ?? box.WidthMM, 1),
    height_mm: round2(heightMM),
  }
}

export function pdfPlacementToSalesLayoutBox(placement = {}, page = {}) {
  const pointPerMM = pdfPointsPerMM(page)
  return {
    x_mm: Math.round(nonNegativeNumber(placement.x, 0) / pointPerMM),
    y_mm: Math.round(nonNegativeNumber(placement.y, 0) / pointPerMM),
    width_mm: Math.round(positiveNumber(placement.width, 1) / pointPerMM),
    height_mm: Math.round(positiveNumber(placement.height, 1) / pointPerMM),
  }
}

export function pdfPointsPerMM(page = {}) {
  return positiveNumber(page.pageWidth ?? page.width, 595.28) / salesDocumentPageWidthMM
}

function pdfPageHeight(page = {}) {
  return positiveNumber(page.pageHeight ?? page.height, 841.89)
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
