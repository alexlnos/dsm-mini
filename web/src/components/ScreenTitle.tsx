import { hasNativeBack } from '../telegram'
import { t } from '../i18n'

interface Props {
  title: string
  onBack: () => void
  /** What to show to the right of the title. */
  children?: React.ReactNode
}

/**
 * A section title.
 *
 * The back button appears only where there is no system one: inside Telegram
 * the client handles navigation, and duplicating it means putting a second
 * arrow next to the first.
 */
export function ScreenTitle({ title, onBack, children }: Props) {
  return (
    <div className="home-head">
      {!hasNativeBack() && (
        <button type="button" className="back-button" onClick={onBack} aria-label={t('common.back')}>
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
