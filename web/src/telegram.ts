/**
 * Тонкая обёртка над Telegram WebApp.
 *
 * Работаем через глобальный объект, а не только через SDK: приложение должно
 * открываться и в обычном браузере при разработке, где ничего этого нет.
 */
import { setLocale } from './i18n'

interface TelegramWebApp {
  initData?: string
  /** Разобранные параметры запуска; подпись у них та же, что у initData. */
  initDataUnsafe?: { user?: { language_code?: string } }
  colorScheme?: 'light' | 'dark'
  themeParams?: Record<string, string>
  viewportStableHeight?: number
  isVersionAtLeast?: (version: string) => boolean
  ready?: () => void
  expand?: () => void
  disableVerticalSwipes?: () => void
  setHeaderColor?: (color: string) => void
  setBottomBarColor?: (color: string) => void
  onEvent?: (event: string, handler: () => void) => void
  showConfirm?: (message: string, callback: (ok: boolean) => void) => void
  showAlert?: (message: string, callback?: () => void) => void
  HapticFeedback?: {
    impactOccurred?: (style: string) => void
    notificationOccurred?: (type: string) => void
  }
  BackButton?: {
    show: () => void
    hide: () => void
    onClick: (cb: () => void) => void
    offClick: (cb: () => void) => void
  }
}

function webApp(): TelegramWebApp | undefined {
  return (window as unknown as { Telegram?: { WebApp?: TelegramWebApp } }).Telegram?.WebApp
}

export function initTelegram(): string {
  const app = webApp()
  app?.ready?.()
  app?.expand?.()
  applyTheme(app)
  applyViewport(app)
  applyChrome(app)

  // Тема и высота меняются уже после запуска: пользователь переключает
  // оформление, поворачивает телефон, вытягивает приложение на весь экран.
  app?.onEvent?.('themeChanged', () => {
    applyTheme(webApp())
    applyChrome(webApp())
  })
  app?.onEvent?.('viewportChanged', () => applyViewport(webApp()))

  // Вертикальный свайп сворачивает приложение вместо прокрутки списка —
  // у нас свои прокручиваемые списки, и жест только мешает. Закрыть
  // приложение по-прежнему можно крестиком и кнопкой «назад».
  if (supports(app, '7.7')) app?.disableVerticalSwipes?.()

  const initData = app?.initData || initDataFromLocation()
  setLocale(languageCode(app, initData))
  return initData
}

/**
 * Язык, выбранный человеком в Telegram.
 *
 * Клиент отдаёт его разобранным, но если скрипт не загрузился, тот же код
 * лежит в подписанных параметрах запуска. Совсем без Telegram (разработка в
 * браузере) спрашиваем браузер.
 */
function languageCode(app: TelegramWebApp | undefined, initData: string): string | undefined {
  const known = app?.initDataUnsafe?.user?.language_code
  if (known) return known
  try {
    const raw = new URLSearchParams(initData).get('user')
    if (raw) {
      const user = JSON.parse(raw) as { language_code?: string }
      if (user.language_code) return user.language_code
    }
  } catch {
    // Параметры могут быть любыми — язык не повод ломать запуск.
  }
  return navigator.language
}

/**
 * Возможности Telegram появлялись в разных версиях Bot API, и у старого
 * клиента их просто нет. Документация не обещает, что вызов неизвестного
 * метода безопасен, поэтому спрашиваем заранее.
 */
function supports(app: TelegramWebApp | undefined, version: string): boolean {
  return app?.isVersionAtLeast?.(version) ?? false
}

/**
 * Высоту берём у Telegram, а не у браузера.
 *
 * Внутри клиента `100dvh` — это высота всего экрана, тогда как приложению
 * отведена лишь часть: пока оно не раскрыто, нижняя панель вкладок уезжает
 * за край видимой области. `viewportStableHeight` — та же высота, но без
 * скачков во время жестов и анимаций.
 */
function applyViewport(app?: TelegramWebApp) {
  const height = app?.viewportStableHeight
  if (height) document.documentElement.style.setProperty('--app-height', `${height}px`)
}

/**
 * Шапка и нижняя полоса самого клиента красятся под приложение: иначе на
 * стыке видна чужая полоса другого цвета.
 *
 * Передаём не свой цвет, а имя цвета темы — тогда клиент сам подставит
 * нужный оттенок для светлого и тёмного оформления.
 */
function applyChrome(app?: TelegramWebApp) {
  if (supports(app, '6.1')) app?.setHeaderColor?.('secondary_bg_color')
  if (supports(app, '7.10')) app?.setBottomBarColor?.('bg_color')
}

/**
 * Запасной способ получить подпись.
 *
 * Telegram кладёт параметры запуска во фрагмент адреса (#tgWebAppData=…), и
 * они доступны, даже если внешний скрипт telegram-web-app.js не загрузился —
 * например, когда у клиента нет доступа к telegram.org. Без этого приложение
 * показывало бы «откройте через Telegram», будучи открытым именно из него.
 */
function initDataFromLocation(): string {
  const sources = [window.location.hash.slice(1), window.location.search.slice(1)]
  for (const source of sources) {
    if (!source) continue
    const value = new URLSearchParams(source).get('tgWebAppData')
    if (value) return value
  }
  // Telegram сохраняет параметры запуска между переходами внутри приложения.
  try {
    const stored = sessionStorage.getItem('__telegram__initParams')
    if (stored) {
      const parsed = JSON.parse(stored) as { tgWebAppData?: string }
      if (parsed.tgWebAppData) return parsed.tgWebAppData
    }
  } catch {
    // Хранилище может быть недоступно — это не повод падать.
  }
  return ''
}

/**
 * Переносим тему Telegram в CSS-переменные: Mini App должен выглядеть как
 * часть клиента, а не как посторонняя страница.
 */
function applyTheme(app?: TelegramWebApp) {
  const root = document.documentElement
  const dark = app?.colorScheme === 'dark'
  root.dataset.theme = dark ? 'dark' : 'light'

  const params = app?.themeParams ?? {}
  const map: Record<string, string | undefined> = {
    '--tg-bg': params.secondary_bg_color ?? params.bg_color,
    '--tg-surface': params.bg_color,
    '--tg-text': params.text_color,
    '--tg-hint': params.hint_color,
    '--tg-accent': params.button_color ?? params.link_color,
  }
  for (const [name, value] of Object.entries(map)) {
    if (value) root.style.setProperty(name, value)
  }
}

/** Подтверждение: нативное окно Telegram, а не браузерное. */
export function confirmAction(message: string): Promise<boolean> {
  const app = webApp()
  if (app?.showConfirm) {
    return new Promise((resolve) => app.showConfirm!(message, resolve))
  }
  return Promise.resolve(window.confirm(message))
}

export function alertMessage(message: string) {
  const app = webApp()
  if (app?.showAlert) {
    app.showAlert(message)
    return
  }
  window.alert(message)
}

export function haptic(kind: 'light' | 'success' | 'error') {
  const h = webApp()?.HapticFeedback
  if (!h) return
  if (kind === 'light') h.impactOccurred?.('light')
  else h.notificationOccurred?.(kind)
}

/**
 * Есть ли системная кнопка «назад».
 *
 * Внутри Telegram она всегда есть, а при открытии в браузере (разработка,
 * клиент без поддержки) — нет, и без запасной кнопки из раздела не выйти.
 */
export function hasNativeBack(): boolean {
  return Boolean(webApp()?.BackButton)
}

/** Системная кнопка «назад» в шапке Telegram. */
export function backButton(onBack: (() => void) | null) {
  const button = webApp()?.BackButton
  if (!button) return () => {}
  if (!onBack) {
    button.hide()
    return () => {}
  }
  button.onClick(onBack)
  button.show()
  return () => {
    button.offClick(onBack)
    button.hide()
  }
}
