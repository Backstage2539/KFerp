export type PromptChoice = { enabled: boolean; events: string[] }
export type PromptKey = { user: number; customer: number; employee: number; version: number }

export function promptSelection(choice: PromptChoice | null, existing: PromptChoice | null = null): PromptChoice {
  const selected = choice ?? existing
  return selected ? { enabled: selected.enabled, events: [...selected.events] } : { enabled: false, events: [] }
}

function keyOf(key: PromptKey): string {
  return `${key.user}:${key.customer}:${key.employee}:${key.version}`
}

// Kept in memory only: postponing does not count as accepting or rejecting.
export function createNotificationPromptCoordinator() {
  const postponed = new Set<string>()
  let active = ''
  return {
    claim(key: PromptKey): boolean {
      const id = keyOf(key)
      if (active || postponed.has(id)) return false
      active = id
      return true
    },
    release(key: PromptKey) {
      if (active === keyOf(key)) active = ''
    },
    dismiss(key: PromptKey) {
      const id = keyOf(key)
      if (active === id) active = ''
      postponed.add(id)
    },
  }
}

export const notificationPromptCoordinator = createNotificationPromptCoordinator()

const visibilityListeners = new Set<(visible: boolean) => void>()
export function onNotificationPageVisibility(listener: (visible: boolean) => void): () => void {
  visibilityListeners.add(listener)
  return () => { visibilityListeners.delete(listener) }
}

// Installed on page instances: WeChat component pageLifetimes do not call
// the component's onShow hook, so page events must be forwarded explicitly.
export const notificationPageMixin = {
  onShow() { visibilityListeners.forEach(listener => listener(true)) },
  onHide() { visibilityListeners.forEach(listener => listener(false)) },
}
