type Versioned = { revision: string; unchanged?: boolean }
type FetchVersion<T extends Versioned> = (known: string) => Promise<T>

export function createEntrySessionCache() {
  let owner = ''
  let generation = 0
  const records = new Map<string, Versioned>()
  const pending = new Map<string, Promise<Versioned>>()

  function clear() {
    owner = ''
    generation++
    records.clear()
    pending.clear()
  }
  function remember<T extends Versioned>(session: string, key: string, value: T) {
    if (session !== owner) return
    records.delete(key)
    records.set(key, value)
    if (records.size > 32) records.delete(records.keys().next().value!)
  }
  function read<T extends Versioned>(session: string, key: string, fetch: FetchVersion<T>): Promise<T> {
    if (!session) return Promise.reject(new Error('请重新登录'))
    if (owner !== session) { clear(); owner = session }
    const inFlight = pending.get(key)
    if (inFlight) return inFlight as Promise<T>
    const cached = records.get(key) as T | undefined
    const start = generation
    const task = fetch(cached?.revision || '').then(reply => {
      if (start !== generation) throw new Error('录单授权已变化')
      if (!reply.revision || (reply.unchanged && reply.revision !== cached?.revision)) throw new Error('配置版本无法核对，请重试')
      if (reply.unchanged && cached) return cached
      remember(session, key, reply)
      return reply
    }).finally(() => { if (pending.get(key) === task) pending.delete(key) })
    pending.set(key, task)
    return task
  }
  return { read, clear, remember }
}
export const employeeEntryCache = createEntrySessionCache()
