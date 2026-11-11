import { describe, expect, it } from 'vitest'
import { createNotificationPromptCoordinator, notificationPageMixin, onNotificationPageVisibility, promptSelection } from './notificationPrompt'

describe('notification preference prompt', () => {
  it('starts with no implicit consent and preserves an existing explicit choice', () => {
    expect(promptSelection(null)).toEqual({ enabled: false, events: [] })
    const previous = { enabled: true, events: ['order_accepted'] }
    expect(promptSelection(previous)).toEqual(previous)
    expect(promptSelection(previous).events).not.toBe(previous.events)
    expect(promptSelection(null, previous)).toEqual(previous)
    expect(promptSelection({ enabled: false, events: [] }, previous)).toEqual({ enabled: false, events: [] })
  })

  it('suppresses dismissal only for the same account, customer, and version within this session', () => {
    const c = createNotificationPromptCoordinator()
    const first = { user: 7, customer: 1, employee: 0, version: 2 }
    expect(c.claim(first)).toBe(true)
    expect(c.claim(first)).toBe(false)
    c.dismiss(first)
    expect(c.claim(first)).toBe(false)
    expect(c.claim({ ...first, version: 3 })).toBe(true)
    c.release({ ...first, version: 3 })
    expect(c.claim({ ...first, customer: 2 })).toBe(true)
  })

  it('releases failed requests without recording consent or suppressing a retry', () => {
    const c = createNotificationPromptCoordinator()
    const key = { user: 7, customer: 0, employee: 9, version: 1 }
    expect(c.claim(key)).toBe(true)
    c.release(key)
    expect(c.claim(key)).toBe(true)
  })
  it('forwards actual page lifecycle notifications and allows listeners to unsubscribe', () => {
    const values: boolean[] = []
    const unsubscribe = onNotificationPageVisibility(value => values.push(value))
    notificationPageMixin.onShow()
    notificationPageMixin.onHide()
    unsubscribe()
    notificationPageMixin.onShow()
    expect(values).toEqual([true, false])
  })
})
