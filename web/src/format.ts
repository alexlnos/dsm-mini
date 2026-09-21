import { formatNumber, localeTag, t } from './i18n'

/** Ключи единиц по возрастанию: каждый следующий больше предыдущего в 1024 раза. */
const UNITS = ['unit.kb', 'unit.mb', 'unit.gb', 'unit.tb'] as const

/** Размер в байтах человеческим языком. */
export function size(bytes: number): string {
  if (!bytes || bytes < 0) return `0 ${t('unit.bytes')}`
  if (bytes < 1024) return `${bytes} ${t('unit.bytes')}`
  let value = bytes / 1024
  let unit = 0
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024
    unit++
  }
  // Дробный разделитель у каждого языка свой: в русском запятая, в английском точка.
  return `${formatNumber(value, value >= 100 ? 0 : 1)} ${t(UNITS[unit])}`
}

/** Скорость. Ноль показываем прочерком: «0 Б/с» в списке читается как шум. */
export function speed(bytesPerSecond: number): string {
  if (!bytesPerSecond) return t('common.dash')
  return t('unit.perSecond', { size: size(bytesPerSecond) })
}

/** Оставшееся время. */
export function eta(seconds?: number): string {
  if (!seconds || seconds <= 0) return ''
  if (seconds >= 86400) return t('unit.days', { value: Math.round(seconds / 86400) })
  if (seconds >= 3600) {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.round((seconds % 3600) / 60)
    return minutes
      ? t('unit.hoursMinutes', { hours, minutes })
      : t('unit.hours', { value: hours })
  }
  if (seconds >= 60) return t('unit.minutes', { value: Math.round(seconds / 60) })
  return t('unit.seconds', { value: Math.round(seconds) })
}

/**
 * Пока Download Station не получил метаданные от трекера, названием задачи
 * служит вся magnet-ссылка. Достаём из неё человеческое имя, иначе список
 * первые секунды выглядит кашей.
 */
export function taskTitle(raw: string): string {
  if (!raw.startsWith('magnet:')) return raw
  const match = /[?&]dn=([^&]+)/.exec(raw)
  if (!match) return t('status.fetchingInfo')
  try {
    return decodeURIComponent(match[1].replace(/\+/g, ' '))
  } catch {
    return t('status.fetchingInfo')
  }
}

export function statusLabel(status: string): string {
  switch (status) {
    case 'waiting': return t('status.waiting')
    case 'downloading': return t('status.downloading')
    case 'paused': return t('status.paused')
    case 'finishing': return t('status.finishing')
    case 'finished': return t('status.finished')
    case 'hash_checking': return t('status.hashChecking')
    case 'seeding': return t('status.seeding')
    case 'extracting': return t('status.extracting')
    case 'error': return t('status.error')
    default: return t('status.unknown')
  }
}

/** Аптайм человеческим языком. */
export function uptime(seconds: number): string {
  if (!seconds) return t('common.dash')
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days > 0) return t('unit.daysHours', { days, hours })
  const minutes = Math.floor((seconds % 3600) / 60)
  return hours > 0
    ? t('unit.hoursMinutes', { hours, minutes })
    : t('unit.minutes', { value: minutes })
}

/** Дата и время записи журнала. */
export function logTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString(localeTag(), { hour: '2-digit', minute: '2-digit' })
}

/** День для группировки записей журнала. */
export function logDay(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return t('day.earlier')
  const today = new Date()
  const yesterday = new Date(today)
  yesterday.setDate(today.getDate() - 1)
  const same = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
  if (same(d, today)) return t('day.today')
  if (same(d, yesterday)) return t('day.yesterday')
  return d.toLocaleDateString(localeTag(), { day: 'numeric', month: 'long' })
}
