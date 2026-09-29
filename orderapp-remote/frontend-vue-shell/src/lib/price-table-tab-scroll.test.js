import test from 'node:test'
import assert from 'node:assert/strict'
import { keepElementHorizontallyVisible } from './price-table-tab-scroll.js'

function element(left, right, { clientLeft = 0, clientWidth = 100, scrollLeft = 0 } = {}) {
  return {
    clientLeft,
    clientWidth,
    scrollLeft,
    getBoundingClientRect: () => ({ left, right, width: right - left }),
  }
}

test('active tab scrolls only its strip far enough to reveal the right edge', () => {
  const strip = element(0, 100)
  keepElementHorizontallyVisible(strip, element(80, 124))
  assert.equal(strip.scrollLeft, 24)
})

test('active tab scrolls only its strip far enough to reveal the left edge', () => {
  const strip = element(0, 100, { scrollLeft: 30 })
  keepElementHorizontallyVisible(strip, element(-18, 20))
  assert.equal(strip.scrollLeft, 12)
})

test('visible tabs do not move the strip', () => {
  const strip = element(10, 110, { clientLeft: 5, scrollLeft: 12 })
  keepElementHorizontallyVisible(strip, element(30, 70))
  assert.equal(strip.scrollLeft, 12)
})
