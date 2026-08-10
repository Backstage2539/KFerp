import { computed, getCurrentInstance, ref } from 'vue'
import { onHide } from '@dcloudio/uni-app'
import {
  initialPullUpBrandGestureState,
  isPageAtBottom,
  reducePullUpBrandGesture,
  type PullUpBrandGestureEvent,
} from '../utils/pullUpBrandGesture'

type TouchPoint = {
  clientX?: number
  clientY?: number
  pageX?: number
  pageY?: number
}

type PullUpBrandTouchEvent = Pick<TouchEvent, 'touches' | 'changedTouches'>

const BOTTOM_CHECK_INTERVAL_MS = 72

function eventTouchPoint(event: PullUpBrandTouchEvent): { x: number; y: number } | null {
  const touch = event.touches?.[0] || event.changedTouches?.[0]
  if (!touch) return null
  const x = Number(touch.clientX ?? touch.pageX)
  const y = Number(touch.clientY ?? touch.pageY)
  if (!Number.isFinite(x) || !Number.isFinite(y)) return null
  return { x, y }
}

export function usePullUpBrandGesture() {
  const componentInstance = getCurrentInstance()
  const gestureState = ref({ ...initialPullUpBrandGestureState })
  let pendingGestureId: number | null = null
  let lastBottomCheckAt = 0

  function dispatch(event: PullUpBrandGestureEvent) {
    gestureState.value = reducePullUpBrandGesture(gestureState.value, event)
  }

  function measurePageBottom(force = false) {
    if (gestureState.value.phase !== 'checking') return
    const gestureId = gestureState.value.gestureId
    const now = Date.now()
    if (pendingGestureId === gestureId) return
    if (!force && now - lastBottomCheckAt < BOTTOM_CHECK_INTERVAL_MS) return

    pendingGestureId = gestureId
    lastBottomCheckAt = now
    const windowInfo = typeof uni.getWindowInfo === 'function'
      ? uni.getWindowInfo()
      : uni.getSystemInfoSync()
    const viewportHeightPx = Number(windowInfo.windowHeight)
    const query = uni.createSelectorQuery()
    const scopedQuery = componentInstance?.proxy
      ? query.in(componentInstance.proxy as never)
      : query

    scopedQuery
      .select('.pull-up-brand-page')
      .boundingClientRect((rect) => {
        if (pendingGestureId === gestureId) pendingGestureId = null
        const node = Array.isArray(rect) ? rect[0] : rect
        dispatch({
          type: 'bottom-result',
          gestureId,
          atBottom: isPageAtBottom({
            rootBottomPx: Number(node?.bottom),
            viewportHeightPx,
          }),
        })
      })
      .exec()
  }

  function handlePullUpBrandTouchStart(event: PullUpBrandTouchEvent) {
    const point = eventTouchPoint(event)
    if (!point) return
    dispatch({ type: 'touch-start', ...point })
    pendingGestureId = null
    lastBottomCheckAt = 0
    measurePageBottom(true)
  }

  function handlePullUpBrandTouchMove(event: PullUpBrandTouchEvent) {
    const point = eventTouchPoint(event)
    if (!point) return
    dispatch({ type: 'touch-move', ...point })
    measurePageBottom()
  }

  function handlePullUpBrandTouchEnd() {
    dispatch({ type: 'touch-end' })
  }

  function handlePullUpBrandTouchCancel() {
    dispatch({ type: 'touch-cancel' })
  }

  onHide(() => {
    dispatch({ type: 'page-hide' })
  })

  return {
    pullUpBrandRevealed: computed(() => gestureState.value.revealed),
    handlePullUpBrandTouchStart,
    handlePullUpBrandTouchMove,
    handlePullUpBrandTouchEnd,
    handlePullUpBrandTouchCancel,
  }
}
