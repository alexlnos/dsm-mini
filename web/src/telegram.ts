/**
 * A thin wrapper over the Telegram WebApp.
 *
 * We go through the global object rather than the SDK alone: the app must
 * also open in a plain browser during development, where none of this exists.
 */
import { setLocale } from './i18n'

interface TelegramWebApp {
  initData?: string
  /** Parsed launch parameters; they carry the same signature as initData. */
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

  // Theme and height change after startup too: the user switches the colour
  // scheme, turns the phone, pulls the app to full screen.
  app?.onEvent?.('themeChanged', () => {
    applyTheme(webApp())
    applyChrome(webApp())
  })
  app?.onEvent?.('viewportChanged', () => applyViewport(webApp()))

  // A vertical swipe minimises the app instead of scrolling a list — we have
  // our own scrollable lists and the gesture only gets in the way. The app
  // can still be closed with the cross and the back button.
  if (supports(app, '7.7')) app?.disableVerticalSwipes?.()

  const initData = app?.initData || initDataFromLocation()
  setLocale(languageCode(app, initData))
  return initData
}

/**
 * The language the person picked in Telegram.
 *
 * The client hands it over parsed, but if the script did not load, the same
 * code sits in the signed launch parameters. With no Telegram at all
 * (development in a browser) we ask the browser.
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
    // The parameters can be anything — a language is no reason to break startup.
  }
  return navigator.language
}

/**
 * Telegram features appeared in different Bot API versions, and an old client
 * simply does not have them. The documentation does not promise that calling
 * an unknown method is safe, so we ask beforehand.
 */
function supports(app: TelegramWebApp | undefined, version: string): boolean {
  return app?.isVersionAtLeast?.(version) ?? false
}

/**
 * We take the height from Telegram rather than from the browser.
 *
 * Inside the client `100dvh` is the height of the whole screen, while the app
 * gets only a part of it: until it is expanded, the bottom tab bar slides off
 * the visible area. `viewportStableHeight` is the same height but without the
 * jumps during gestures and animations.
 */
function applyViewport(app?: TelegramWebApp) {
  const height = app?.viewportStableHeight
  if (height) document.documentElement.style.setProperty('--app-height', `${height}px`)
}

/**
 * The client's own header and bottom bar are painted to match the app:
 * otherwise a strip of a different colour shows at the seam.
 *
 * We pass a theme colour name rather than our own colour — then the client
 * picks the right shade for the light and dark schemes itself.
 */
function applyChrome(app?: TelegramWebApp) {
  if (supports(app, '6.1')) app?.setHeaderColor?.('secondary_bg_color')
  if (supports(app, '7.10')) app?.setBottomBarColor?.('bg_color')
}

/**
 * The fallback way to get the signature.
 *
 * Telegram puts the launch parameters into the address fragment
 * (#tgWebAppData=…), and they are available even if the external
 * telegram-web-app.js did not load — for example when the client has no
 * access to telegram.org. Without it the app would say "open it from Telegram"
 */
function initDataFromLocation(): string {
  const sources = [window.location.hash.slice(1), window.location.search.slice(1)]
  for (const source of sources) {
    if (!source) continue
    const value = new URLSearchParams(source).get('tgWebAppData')
    if (value) return value
  }
  // Telegram keeps the launch parameters across navigation inside the app.
  try {
    const stored = sessionStorage.getItem('__telegram__initParams')
    if (stored) {
      const parsed = JSON.parse(stored) as { tgWebAppData?: string }
      if (parsed.tgWebAppData) return parsed.tgWebAppData
    }
  } catch {
    // The storage may be unavailable — that is no reason to crash.
  }
  return ''
}

/**
 * We carry the Telegram theme into CSS variables: a Mini App must look like
 * part of the client rather than a foreign page.
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

/** Confirmation: Telegram's native dialog, not the browser one. */
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
 * Whether there is a system back button.
 *
 * Inside Telegram it is always there, while in a browser (development, a
 * client without support) it is not, and without a fallback there is no way out.
 */
export function hasNativeBack(): boolean {
  return Boolean(webApp()?.BackButton)
}

/** The system back button in the Telegram header. */
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
