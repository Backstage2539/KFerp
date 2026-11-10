import type { HomeEntry } from './capabilities'

export function previewSafeEntries(entries: HomeEntry[]): HomeEntry[] {
  return entries.filter(entry => entry.key === 'inventory')
}
