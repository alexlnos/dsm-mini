const UNITS = ['КБ', 'МБ', 'ГБ', 'ТБ']

/** Размер в байтах человеческим языком. */
export function size(bytes: number): string {
  if (!bytes || bytes < 0) return '0 Б'
  if (bytes < 1024) return `${bytes} Б`
  let value = bytes / 1024
  let unit = 0
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024
    unit++
  }
  return `${value.toFixed(value >= 100 ? 0 : 1).replace('.', ',')} ${UNITS[unit]}`
}

/** Скорость. Ноль показываем прочерком: «0 Б/с» в списке читается как шум. */
export function speed(bytesPerSecond: number): string {
  if (!bytesPerSecond) return '—'
  return `${size(bytesPerSecond)}/с`
}

/** Оставшееся время. */
export function eta(seconds?: number): string {
  if (!seconds || seconds <= 0) return ''
  if (seconds >= 86400) return `${Math.round(seconds / 86400)} дн`
  if (seconds >= 3600) {
    const h = Math.floor(seconds / 3600)
    const m = Math.round((seconds % 3600) / 60)
    return m ? `${h} ч ${m} мин` : `${h} ч`
  }
  if (seconds >= 60) return `${Math.round(seconds / 60)} мин`
  return `${Math.round(seconds)} с`
}

/**
 * Пока Download Station не получил метаданные от трекера, названием задачи
 * служит вся magnet-ссылка. Достаём из неё человеческое имя, иначе список
 * первые секунды выглядит кашей.
 */
export function taskTitle(raw: string): string {
  if (!raw.startsWith('magnet:')) return raw
  const match = /[?&]dn=([^&]+)/.exec(raw)
  if (!match) return 'Получение сведений…'
  try {
    return decodeURIComponent(match[1].replace(/\+/g, ' '))
  } catch {
    return 'Получение сведений…'
  }
}

export function statusLabel(status: string): string {
  switch (status) {
    case 'waiting': return 'В очереди'
    case 'downloading': return 'Загружается'
    case 'paused': return 'На паузе'
    case 'finishing': return 'Завершается'
    case 'finished': return 'Готово'
    case 'hash_checking': return 'Проверка'
    case 'seeding': return 'Раздаётся'
    case 'extracting': return 'Распаковка'
    case 'error': return 'Ошибка'
    default: return 'Неизвестно'
  }
}

/** Аптайм человеческим языком. */
export function uptime(seconds: number): string {
  if (!seconds) return '—'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return `${days} дн ${hours} ч`
  const minutes = Math.floor((seconds % 3600) / 60)
  return hours > 0 ? `${hours} ч ${minutes} мин` : `${minutes} мин`
}

/** Дата и время записи журнала. */
export function logTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

/** День для группировки записей журнала. */
export function logDay(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return 'Ранее'
  const today = new Date()
  const yesterday = new Date(today)
  yesterday.setDate(today.getDate() - 1)
  const same = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
  if (same(d, today)) return 'Сегодня'
  if (same(d, yesterday)) return 'Вчера'
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
}
