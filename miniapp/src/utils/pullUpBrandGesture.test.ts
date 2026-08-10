import { describe, expect, it } from 'vitest'
import {
  initialPullUpBrandGestureState,
  isPageAtBottom,
  reducePullUpBrandGesture,
} from './pullUpBrandGesture'

function startGesture(x = 120, y = 600) {
  return reducePullUpBrandGesture(initialPullUpBrandGestureState, {
    type: 'touch-start',
    x,
    y,
  })
}

describe('pull-up brand gesture', () => {
  it('reveals only after an upward dominant pull continues past the activation threshold at page bottom', () => {
    let state = startGesture()
    const gestureId = state.gestureId
    state = reducePullUpBrandGesture(state, {
      type: 'bottom-result',
      gestureId,
      atBottom: true,
    })

    state = reducePullUpBrandGesture(state, { type: 'touch-move', x: 120, y: 594 })
    expect(state.revealed).toBe(false)

    state = reducePullUpBrandGesture(state, { type: 'touch-move', x: 122, y: 570 })
    expect(state.revealed).toBe(true)

    state = reducePullUpBrandGesture(state, { type: 'touch-move', x: 160, y: 568 })
    expect(state.revealed).toBe(false)
  })

  it('never reveals away from the real page bottom and can begin tracking when the same gesture reaches bottom', () => {
    let state = startGesture()
    const gestureId = state.gestureId
    state = reducePullUpBrandGesture(state, { type: 'touch-move', x: 120, y: 540 })
    state = reducePullUpBrandGesture(state, {
      type: 'bottom-result',
      gestureId,
      atBottom: false,
    })
    expect(state.revealed).toBe(false)

    state = reducePullUpBrandGesture(state, {
      type: 'bottom-result',
      gestureId,
      atBottom: true,
    })
    expect(state.revealed).toBe(false)
    state = reducePullUpBrandGesture(state, { type: 'touch-move', x: 120, y: 518 })
    expect(state.revealed).toBe(true)
  })

  it.each(['touch-end', 'touch-cancel', 'page-hide'] as const)(
    '%s immediately collapses the tail and returns to idle',
    (type) => {
      let state = startGesture()
      state = reducePullUpBrandGesture(state, {
        type: 'bottom-result',
        gestureId: state.gestureId,
        atBottom: true,
      })
      state = reducePullUpBrandGesture(state, { type: 'touch-move', x: 120, y: 560 })
      expect(state.revealed).toBe(true)

      state = reducePullUpBrandGesture(state, { type })
      expect(state.phase).toBe('idle')
      expect(state.revealed).toBe(false)
      expect(state.originY).toBeNull()
      expect(state.latestY).toBeNull()
    },
  )

  it('ignores stale asynchronous bottom results after release or from a previous gesture', () => {
    const first = startGesture()
    const released = reducePullUpBrandGesture(first, { type: 'touch-end' })
    const lateAfterRelease = reducePullUpBrandGesture(released, {
      type: 'bottom-result',
      gestureId: first.gestureId,
      atBottom: true,
    })
    expect(lateAfterRelease.phase).toBe('idle')
    expect(lateAfterRelease.revealed).toBe(false)

    const second = reducePullUpBrandGesture(released, { type: 'touch-start', x: 120, y: 600 })
    const stalePreviousGesture = reducePullUpBrandGesture(second, {
      type: 'bottom-result',
      gestureId: first.gestureId,
      atBottom: true,
    })
    expect(stalePreviousGesture.phase).toBe('checking')
    expect(stalePreviousGesture.revealed).toBe(false)
  })

  it('treats roots within the viewport tolerance as bottom without accepting longer unfinished pages', () => {
    expect(isPageAtBottom({ rootBottomPx: 799, viewportHeightPx: 800 })).toBe(true)
    expect(isPageAtBottom({ rootBottomPx: 802, viewportHeightPx: 800 })).toBe(true)
    expect(isPageAtBottom({ rootBottomPx: 803, viewportHeightPx: 800 })).toBe(false)
    expect(isPageAtBottom({ rootBottomPx: 1200, viewportHeightPx: 800 })).toBe(false)
  })
})
