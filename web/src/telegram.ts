/**
 * Тонкая обёртка над Telegram WebApp.
 *
 * Работаем через глобальный объект, а не только через SDK: приложение должно
 * открываться и в обычном браузере при разработке, где ничего этого нет.
 */
interface TelegramWebApp {
  initData?: string
  colorScheme?: 'light' | 'dark'
  themeParams?: Record<string, string>
  ready?: () => void
  expand?: () => void
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
  return app?.initData ?? ''
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
