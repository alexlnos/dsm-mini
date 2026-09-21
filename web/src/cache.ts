/**
 * Кэш ответов между запусками приложения.
 *
 * Telegram держит Mini App в памяти недолго, и при каждом открытии экран
 * начинался с пустоты, пока шёл запрос к NAS. Сохранённые значения дают
 * мгновенную картинку, а свежие подменяют её, когда придут.
 *
 * Хранилище может быть недоступно (приватный режим, запрет на данные сайта),
 * поэтому любое обращение к нему безопасно для приложения: не вышло — просто
 * работаем без кэша.
 */
const PREFIX = 'dsm-mini:'

/** Данные старше этого срока не показываем: лучше пустой экран, чем вчерашний. */
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
    // Переполнение или запрет на запись — не повод ломать экран.
  }
}
