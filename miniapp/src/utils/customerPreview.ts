import type { HomeEntry } from './capabilities'

export function previewSafeEntries(entries: HomeEntry[]): HomeEntry[] {
  return entries.filter(entry => ['inventory', 'directShip', 'onDemandShip'].includes(entry.key))
}
