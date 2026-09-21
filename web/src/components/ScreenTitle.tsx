import { hasNativeBack } from '../telegram'

interface Props {
  title: string
  onBack: () => void
  /** Что показать справа от заголовка. */
  children?: React.ReactNode
}

/**
 * Заголовок раздела.
 *
 * Кнопка «назад» появляется только там, где нет системной: внутри Telegram
 * навигацией занимается сам клиент, и дублировать её значит городить вторую
 * стрелку рядом с первой.
 */
export function ScreenTitle({ title, onBack, children }: Props) {
  return (
    <div className="home-head">
      {!hasNativeBack() && (
        <button type="button" className="back-button" onClick={onBack} aria-label="Назад">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M15 18l-6-6 6-6" />
          </svg>
        </button>
      )}
      <h1>{title}</h1>
      {children}
    </div>
  )
}
