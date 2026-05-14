import { PDFDocument } from 'pdf-lib'

export function normalizeContractUploadKind({ filename = '', contentType = '' } = {}) {
  const name = String(filename || '').trim().toLowerCase()
  const type = String(contentType || '').trim().toLowerCase()
  if (name.endsWith('.pdf') || type === 'application/pdf') return 'pdf'
  if (name.endsWith('.docx') || type.includes('wordprocessingml.document')) return 'docx'
  return ''
}

export function contractPDFDrawPlacement({ pageHeight = 0, placement = {} } = {}) {
  const x = Number(placement.x || 0)
  const y = Number(placement.y || 0)
  const width = Number(placement.width || 0)
  const height = Number(placement.height || 0)
  return {
    x,
    y: Math.max(0, Number(pageHeight || 0) - y - height),
    width,
    height,
  }
}

export function moveContractStampPlacement(placement = {}, { deltaX = 0, deltaY = 0, displayScale = 1 } = {}) {
  const scale = Number(displayScale) > 0 ? Number(displayScale) : 1
  return {
    page_number: Number(placement.page_number || 1),
    x: Math.max(0, round2(Number(placement.x || 0) + Number(deltaX || 0) / scale)),
    y: Math.max(0, round2(Number(placement.y || 0) + Number(deltaY || 0) / scale)),
    width: round2(Number(placement.width || 0)),
    height: round2(Number(placement.height || 0)),
  }
}

export function contractStampPayload({ contractID = 0, sealAssetID = 0, placements = [] } = {}) {
  return {
    contract_id: Number(contractID || 0),
    seal_asset_id: Number(sealAssetID || 0),
    placements: placements.map((placement) => ({
      page_number: Number(placement.page_number || 1),
      x: round2(placement.x),
      y: round2(placement.y),
      width: round2(placement.width),
      height: round2(placement.height),
    })),
  }
}

export function contractStampOverlayStyle(placement = {}, displayScale = 1) {
  const scale = Number(displayScale) > 0 ? Number(displayScale) : 1
  return {
    left: `${Number(placement.x || 0) * scale}px`,
    top: `${Number(placement.y || 0) * scale}px`,
    width: `${Number(placement.width || 0) * scale}px`,
    height: `${Number(placement.height || 0) * scale}px`,
  }
}

export function defaultContractStampPlacement({ pageNumber = 1, pageWidth = 595, pageHeight = 842, sealAspectRatio = 0.62 } = {}) {
  const width = Math.min(120, Number(pageWidth || 595) * 0.22)
  const height = width * (Number(sealAspectRatio) > 0 ? Number(sealAspectRatio) : 0.62)
  return {
    page_number: Number(pageNumber || 1),
    x: round2(Math.max(24, Number(pageWidth || 595) - width - 48)),
    y: round2(Math.max(24, Number(pageHeight || 842) * 0.18)),
    width: round2(width),
    height: round2(height),
  }
}

export async function createStampedContractPDF({ pdfBytes, sealBytes, sealContentType = 'image/png', placements = [] } = {}) {
  if (!pdfBytes || !sealBytes) throw new Error('pdf and seal required')
  const pdfDoc = await PDFDocument.load(pdfBytes)
  const sealImage = await embedSealImage(pdfDoc, sealBytes, sealContentType)
  const pages = pdfDoc.getPages()
  for (const placement of placements || []) {
    const page = pages[Number(placement.page_number || 1) - 1]
    if (!page) continue
    const { height } = page.getSize()
    page.drawImage(sealImage, contractPDFDrawPlacement({ pageHeight: height, placement }))
  }
  return pdfDoc.save()
}

async function embedSealImage(pdfDoc, sealBytes, sealContentType) {
  const type = String(sealContentType || '').toLowerCase()
  if (type.includes('jpeg') || type.includes('jpg')) {
    return pdfDoc.embedJpg(sealBytes)
  }
  return pdfDoc.embedPng(sealBytes)
}

function round2(value) {
  const n = Number(value || 0)
  return Math.round(n * 100) / 100
}

