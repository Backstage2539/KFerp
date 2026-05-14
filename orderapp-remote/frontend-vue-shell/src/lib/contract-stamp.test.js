import assert from 'node:assert/strict'
import test from 'node:test'
import {
  contractPDFDrawPlacement,
  contractStampPayload,
  moveContractStampPlacement,
  normalizeContractUploadKind,
} from './contract-stamp.js'

test('normalizes contract upload kind from file names and content types', () => {
  assert.equal(normalizeContractUploadKind({ filename: '合作合同.PDF', contentType: 'application/octet-stream' }), 'pdf')
  assert.equal(normalizeContractUploadKind({ filename: '合作合同.docx', contentType: 'application/octet-stream' }), 'docx')
  assert.equal(normalizeContractUploadKind({ filename: 'x.bin', contentType: 'application/pdf' }), 'pdf')
  assert.equal(
    normalizeContractUploadKind({
      filename: 'x.bin',
      contentType: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    }),
    'docx',
  )
  assert.equal(normalizeContractUploadKind({ filename: 'x.txt', contentType: 'text/plain' }), '')
})

test('converts top-left preview placement into pdf-lib bottom-left placement', () => {
  assert.deepEqual(
    contractPDFDrawPlacement({
      pageHeight: 842,
      placement: { page_number: 2, x: 42, y: 84, width: 120, height: 74 },
    }),
    { x: 42, y: 684, width: 120, height: 74 },
  )
})

test('moves contract stamp placement using display scale', () => {
  const placement = moveContractStampPlacement(
    { page_number: 1, x: 30, y: 40, width: 90, height: 56 },
    { deltaX: 24, deltaY: -12, displayScale: 2 },
  )
  assert.deepEqual(placement, { page_number: 1, x: 42, y: 34, width: 90, height: 56 })
})

test('serializes stable stamped contract placement payload', () => {
  const payload = contractStampPayload({
    contractID: 7,
    sealAssetID: 9,
    placements: [{ page_number: 1, x: 12.345, y: 67.891, width: 88.8, height: 55.5 }],
  })
  assert.deepEqual(payload, {
    contract_id: 7,
    seal_asset_id: 9,
    placements: [{ page_number: 1, x: 12.35, y: 67.89, width: 88.8, height: 55.5 }],
  })
})

