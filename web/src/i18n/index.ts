/**
 * Перевод интерфейса.
 *
 * Язык берётся у Telegram (`language_code` пользователя) и на время работы
 * приложения не меняется: клиент не сообщает о смене языка, как сообщает о
 * смене темы. Незнакомый язык получает английский — так принято в опенсорсе
 * и так меньше шансов показать человеку то, чего он не прочтёт.
 *
 * Русский словарь — источник истины, остальные обязаны повторять его ключи:
 * за этим следит тип, а не внимательность.
 */
import { ru, type Key, type Phrase } from './ru'
import { en } from './en'

export type { Key } from './ru'

const DICTS = { en, ru }

export type Locale = keyof typeof DICTS

/**
 * Метки для `Intl`: числа и даты форматируются по правилам языка, а не по
 * нашим. Португальский ведём в бразильском варианте — он массовее.
 */
const TAGS: Record<Locale, string> = { en: 'en', ru: 'ru' }

const FALLBACK: Locale = 'en'

let locale: Locale = FALLBACK
let plural = new Intl.PluralRules(TAGS[FALLBACK])

/**
 * Выбирает язык по коду от Telegram: `ru`, `en`, `pt-br`.
 *
 * Берём только основную часть кода: региональных словарей у нас нет, а
 * `pt-br` и `pt-pt` должны показать хоть что-то, а не английский.
 */
export function setLocale(code?: string): Locale {
  const base = (code ?? '').toLowerCase().split(/[-_]/)[0]
  locale = base in DICTS ? (base as Locale) : FALLBACK
  plural = new Intl.PluralRules(TAGS[locale])
  document.documentElement.lang = TAGS[locale]
  return locale
}

export function currentLocale(): Locale {
  return locale
}

/** Метка текущего языка для `Intl`. */
export function localeTag(): string {
  return TAGS[locale]
}

type Params = Record<string, string | number>

/**
 * Строка интерфейса.
 *
 * `count` в параметрах выбирает форму множественного числа по правилам
 * языка: в русском их три, в английском две, в польском свои.
 */
export function t(key: Key, params?: Params): string {
  const phrase: Phrase = DICTS[locale][key] ?? ru[key]
  return fill(select(phrase, params?.count), params)
}

function select(phrase: Phrase, count?: string | number): string {
  if (typeof phrase === 'string') return phrase
  const rule = typeof count === 'number' ? plural.select(count) : 'other'
  // Формы перечислены не все: в языке без «few» её просто нет в словаре.
  return phrase[rule] ?? phrase.other ?? phrase.many ?? phrase.one ?? ''
}

function fill(text: string, params?: Params): string {
  if (!params) return text
  return text.replace(/\{(\w+)\}/g, (whole, name: string) =>
    name in params ? String(params[name]) : whole,
  )
}

/** Число по правилам языка: разделитель дроби у всех свой. */
export function formatNumber(value: number, digits = 0): string {
  return new Intl.NumberFormat(TAGS[locale], {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(value)
}
