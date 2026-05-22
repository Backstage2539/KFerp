import assert from 'node:assert/strict'
import test from 'node:test'
import {
  movePDFStampPlacement,
  pdfPlacementToSalesLayoutBox,
  pdfPlacementToSalesSealMM,
  resizePDFStampPlacement,
  salesLayoutBoxMMToPDFPlacement,
  salesSealMMToPDFPlacement,
  scalePDFStampPlacement,
} from './document-pdf-stamp.js'

test('converts A4 millimeter seal position to PDF point placement without distorting round seal', () => {
  const got = salesSealMMToPDFPlacement(
    { x_mm: 32, y_mm: 5, width_mm: 36 },
    { pageNumber: 1, pageWidth: 595.28, pageHeight: 841.89 },
    { sealAspectRatio: 1 },
  )
  assert.deepEqual(got, {
    page_number: 1,
    x: 90.71,
    y: 14.17,
    width: 102.05,
    height: 102.05,
  })
})

test('converts PDF point placement back to sales-order seal millimeters', () => {
  const got = pdfPlacementToSalesSealMM(
    { page_number: 1, x: 90.71, y: 14.17, width: 102.05, height: 63.27 },
    { pageWidth: 595.28, pageHeight: 841.89 },
  )
  assert.deepEqual(got, { seal_x_mm: 32, seal_y_mm: 5, seal_width_mm: 36 })
})

test('moves PDF stamp placement using display scale', () => {
  const got = movePDFStampPlacement(
    { page_number: 2, kind: 'payment_text', x: 40, y: 60, width: 120, height: 74.4 },
    { deltaX: 30, deltaY: -15, displayScale: 1.5 },
  )
  assert.deepEqual(got, { page_number: 2, kind: 'payment_text', x: 60, y: 50, width: 120, height: 74.4 })
})

test('converts sales-order layout boxes between millimeters and PDF points', () => {
  const placement = salesLayoutBoxMMToPDFPlacement(
    { x_mm: 126, y_mm: 106, width_mm: 72, height_mm: 122 },
    { pageNumber: 1, pageWidth: 595.28, pageHeight: 841.89 },
    { kind: 'payment_code', label: '收款码位置和大小' },
  )
  assert.deepEqual(placement, {
    kind: 'payment_code',
    label: '收款码位置和大小',
    page_number: 1,
    x: 357.17,
    y: 300.47,
    width: 204.1,
    height: 345.83,
  })

  assert.deepEqual(pdfPlacementToSalesLayoutBox(placement, { pageWidth: 595.28, pageHeight: 841.89 }), {
    x_mm: 126,
    y_mm: 106,
    width_mm: 72,
    height_mm: 122,
  })
})

test('resizes PDF layout placements without dropping metadata', () => {
  const got = resizePDFStampPlacement(
    { page_number: 1, kind: 'payment_code', x: 20, y: 30, width: 100, height: 140 },
    { deltaX: 24, deltaY: -12, displayScale: 2, minWidth: 60, minHeight: 80 },
  )
  assert.deepEqual(got, { page_number: 1, kind: 'payment_code', x: 20, y: 30, width: 112, height: 134 })
})

test('scales PDF stamp placement by width while preserving seal aspect ratio', () => {
  const got = scalePDFStampPlacement(
    { page_number: 1, x: 10, y: 20, width: 80, height: 49.6 },
    { width: 120, sealAspectRatio: 1 },
  )
  assert.deepEqual(got, { page_number: 1, x: 10, y: 20, width: 120, height: 120 })
})
