import { ref } from 'vue'

// A quote belongs to the latest catalog and form revision, even after page returns.
export function useCustomerOrderQuote<T>(options: {
  validate: () => string
  quote: () => Promise<T>
  clear: () => void
  apply: (value: T) => void
  error: (message: string) => void
}) {
  const ready = ref(false), previewing = ref(false)
  let quoteVersion = 0, refreshVersion = 0, disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  function invalidate() {
    quoteVersion++
    if (timer) clearTimeout(timer)
    timer = undefined
    previewing.value = false
    options.clear()
  }
  async function preview(silent = false): Promise<T | null> {
    if (disposed || !ready.value) return null
    const validation = options.validate()
    if (validation) { if (!silent) options.error(validation); return null }
    const version = ++quoteVersion
    previewing.value = true
    try {
      const value = await options.quote()
      if (disposed || version !== quoteVersion) return null
      options.apply(value)
      return value
    } catch (error) {
      if (!disposed && version === quoteVersion) options.error(error instanceof Error ? error.message : '自动核价失败，请重试')
      return null
    } finally {
      if (version === quoteVersion) previewing.value = false
    }
  }
  function schedule() {
    if (timer) clearTimeout(timer)
    if (disposed || !ready.value || options.validate()) return
    timer = setTimeout(() => { timer = undefined; void preview(true) }, 350)
  }
  async function refresh(load: (current: () => boolean) => Promise<void>): Promise<boolean> {
    const version = ++refreshVersion
    ready.value = false
    invalidate()
    const current = () => !disposed && version === refreshVersion
    try {
      await load(current)
      if (!current()) return false
      ready.value = true
      await preview(true)
      return current()
    } catch (error) {
      if (current()) options.error(error instanceof Error ? error.message : '库存加载失败，请重试')
      return false
    }
  }
  function suspend() { refreshVersion++; ready.value = false; invalidate() }
  function dispose() { disposed = true; suspend() }
  async function completeSubmission(save: () => Promise<unknown>, reset: () => void, reload: () => Promise<unknown> | void) {
    await save()
    suspend()
    reset()
    await reload()
  }
  return { ready, previewing, invalidate, preview, schedule, refresh, suspend, dispose, completeSubmission }
}
