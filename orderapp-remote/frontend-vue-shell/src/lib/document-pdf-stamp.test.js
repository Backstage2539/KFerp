import assert from 'node:assert/strict'
import test from 'node:test'
import {
  movePDFStampPlacement,
  pdfPlacementToSalesSealMM,
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
    { page_number: 2, x: 40, y: 60, width: 120, height: 74.4 },
    { deltaX: 30, deltaY: -15, displayScale: 1.5 },
  )
  assert.deepEqual(got, { page_number: 2, x: 60, y: 50, width: 120, height: 74.4 })
})

test('scales PDF stamp placement by width while preserving seal aspect ratio', () => {
  const got = scalePDFStampPlacement(
    { page_number: 1, x: 10, y: 20, width: 80, height: 49.6 },
    { width: 120, sealAspectRatio: 1 },
  )
  assert.deepEqual(got, { page_number: 1, x: 10, y: 20, width: 120, height: 120 })
})
