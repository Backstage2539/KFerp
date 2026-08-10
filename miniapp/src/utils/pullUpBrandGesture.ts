export type PullUpBrandGesturePhase = 'idle' | 'checking' | 'tracking' | 'pulling'

export type PullUpBrandGestureState = {
  phase: PullUpBrandGesturePhase
  gestureId: number
  originX: number | null
  originY: number | null
  latestX: number | null
  latestY: number | null
  revealed: boolean
}

export type PullUpBrandGestureEvent =
  | { type: 'touch-start'; x: number; y: number }
  | { type: 'touch-move'; x: number; y: number }
  | { type: 'bottom-result'; gestureId: number; atBottom: boolean }
  | { type: 'touch-end' }
  | { type: 'touch-cancel' }
  | { type: 'page-hide' }

export type PullUpBrandGestureConfig = {
  activationPx: number
  verticalDominanceRatio: number
}

const defaultConfig: PullUpBrandGestureConfig = {
  activationPx: 12,
  verticalDominanceRatio: 1,
}

export const initialPullUpBrandGestureState: PullUpBrandGestureState = {
  phase: 'idle',
  gestureId: 0,
  originX: null,
  originY: null,
  latestX: null,
  latestY: null,
  revealed: false,
}

function resetGesture(state: PullUpBrandGestureState): PullUpBrandGestureState {
  return {
    ...initialPullUpBrandGestureState,
    gestureId: state.gestureId,
  }
}

export function reducePullUpBrandGesture(
  state: PullUpBrandGestureState,
  event: PullUpBrandGestureEvent,
  config: PullUpBrandGestureConfig = defaultConfig,
): PullUpBrandGestureState {
  if (event.type === 'touch-start') {
    return {
      phase: 'checking',
      gestureId: state.gestureId + 1,
      originX: null,
      originY: null,
      latestX: event.x,
      latestY: event.y,
      revealed: false,
    }
  }

  if (event.type === 'touch-end' || event.type === 'touch-cancel' || event.type === 'page-hide') {
    return resetGesture(state)
  }

  if (event.type === 'bottom-result') {
    if (event.gestureId !== state.gestureId || state.phase !== 'checking') return state
    if (!event.atBottom || state.latestX === null || state.latestY === null) {
      return {
        ...state,
        originX: null,
        originY: null,
        revealed: false,
      }
    }
    return {
      ...state,
      phase: 'tracking',
      originX: state.latestX,
      originY: state.latestY,
      revealed: false,
    }
  }

  if (state.phase === 'idle') return state

  const next = {
    ...state,
    latestX: event.x,
    latestY: event.y,
  }
  if (state.phase === 'checking' || state.originX === null || state.originY === null) return next

  const upwardDistance = state.originY - event.y
  const horizontalDistance = Math.abs(event.x - state.originX)
  const revealed = upwardDistance > config.activationPx
    && upwardDistance > horizontalDistance * config.verticalDominanceRatio

  return {
    ...next,
    phase: revealed ? 'pulling' : 'tracking',
    revealed,
  }
}

export function isPageAtBottom(input: {
  rootBottomPx: number
  viewportHeightPx: number
  tolerancePx?: number
}): boolean {
  const tolerancePx = input.tolerancePx ?? 2
  if (!Number.isFinite(input.rootBottomPx) || !Number.isFinite(input.viewportHeightPx)) return false
  return input.rootBottomPx <= input.viewportHeightPx + tolerancePx
}
