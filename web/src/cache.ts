/**
 * A cache of answers between app launches.
 *
 * Telegram does not keep a Mini App in memory for long, and on every open the
 * screen started empty while the request to the NAS was in flight. Stored
 * values give an instant picture, and fresh ones replace it when they arrive.
 *
 * The storage may be unavailable (private mode, site data blocked), so every
 * access to it is safe for the app: if it did not work, we simply carry on
 * without the cache.
 */
const PREFIX = 'dsm-mini:'

/** Data older than this is not shown: an empty screen beats yesterday's one. */
const MAX_AGE_MS = 10 * 60 * 1000

interface Envelope<T> {
  at: number
  value: T
}

export function readCache<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(PREFIX + key)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Envelope<T>
    if (!parsed || typeof parsed.at !== 'number') return null
    if (Date.now() - parsed.at > MAX_AGE_MS) return null
    return parsed.value
  } catch {
    return null
  }
}

export function writeCache<T>(key: string, value: T): void {
  try {
    localStorage.setItem(PREFIX + key, JSON.stringify({ at: Date.now(), value }))
  } catch {
    // A quota overflow or a write ban is no reason to break the screen.
  }
}
