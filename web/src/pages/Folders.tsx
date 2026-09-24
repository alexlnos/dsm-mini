import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, errorText } from '../api'
import { alertMessage, backButton, haptic } from '../telegram'
import { t } from '../i18n'
import type { NotifyMode, Settings, SettingsView } from '../types'

interface Props {
  /** The path typed by hand: held above so it survives leaving the screen. */
  manual: string
  onManualChange: (value: string) => void
  onBack: () => void
  /** The settings changed — refresh the data on the other screens. */
  onChanged: () => void
  /** Open the NAS browser to pick a folder there. */
  onPickOnNas: () => void
}

/**
 * Destination folder settings.
 *
 * Changes are saved right away: there is no Save button, so there is nothing
 * to lose when going to the NAS browser or closing the app. The interface
 * updates before the server answers and rolls back on an error — otherwise
 * every tap would feel like a delay.
 */
export function Folders({ manual, onManualChange, onBack, onChanged, onPickOnNas }: Props) {
  const [view, setView] = useState<SettingsView | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async () => {
    try {
      setView(await api.settings())
      setError(null)
    } catch (e) {
      setError(describe(e, t('folders.loadFailed')))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const pinned = view?.pinned_folders ?? []
  const showRecent = view?.show_recent ?? true
  const notify = view?.notifications ?? 'downloads'

  const apply = useCallback(
    async (next: Partial<Settings>, notifications?: NotifyMode) => {
      if (!view) return
      const previous = view
      const payload: Settings = {
        pinned_folders: next.pinned_folders ?? pinned,
        show_recent: next.show_recent ?? showRecent,
        last_used: view.last_used,
      }
      // Show the result at once, without waiting for the server.
      setView({ ...view, ...payload, ...(notifications ? { notifications } : {}) })
      try {
        setView(await api.saveSettings(payload, notifications))
        setError(null)
        onChanged()
      } catch (e) {
        setView(previous)
        haptic('error')
        alertMessage(describe(e, t('folders.saveFailed')))
      }
    },
    [view, pinned, showRecent, onChanged],
  )

  function add(folder: string) {
    const clean = folder.trim().replace(/^\/+|\/+$/g, '')
    if (!clean) return
    if (pinned.includes(clean)) {
      alertMessage(t('folders.alreadyPinned'))
      return
    }
    onManualChange('')
    haptic('light')
    void apply({ pinned_folders: [...pinned, clean] })
  }

  function move(index: number, delta: number) {
    const target = index + delta
    if (target < 0 || target >= pinned.length) return
    const next = [...pinned]
    ;[next[index], next[target]] = [next[target], next[index]]
    haptic('light')
    void apply({ pinned_folders: next })
  }

  function unpin(folder: string) {
    haptic('light')
    void apply({ pinned_folders: pinned.filter((f) => f !== folder) })
  }

  const suggested = (view?.suggested ?? []).filter((f) => !pinned.includes(f))

  return (
    <div className="page">
      <div className="card summary">
        <h1>{t('folders.title')}</h1>
        <div className="muted">{t('folders.hint')}</div>
      </div>

      {error && <div className="card error-card">{error}</div>}

      <div className="section-head">
        <span className="section-title">{t('folders.pinned')}</span>
      </div>

      <div className="list">
        {pinned.map((folder, i) => (
          <div key={folder} className="card entry">
            <button
              type="button"
              className="icon-button small danger"
              onClick={() => unpin(folder)}
              aria-label={t('folders.removeAria', { folder })}
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="3" strokeLinecap="round" aria-hidden="true">
                <path d="M5 12h14" />
              </svg>
            </button>
            <span className="entry-text">
              <span className="entry-name">{folder}</span>
            </span>
            <button type="button" className="icon-button small" onClick={() => move(i, -1)}
                    disabled={i === 0} aria-label={t('folders.up')}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M6 14l6-6 6 6" />
              </svg>
            </button>
            <button type="button" className="icon-button small" onClick={() => move(i, 1)}
                    disabled={i === pinned.length - 1} aria-label={t('folders.down')}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M6 10l6 6 6-6" />
              </svg>
            </button>
          </div>
        ))}

        {pinned.length === 0 && (
          <div className="card empty-text">
            {t('folders.empty')}
          </div>
        )}
      </div>

      <div className="section-head">
        <span className="section-title">{t('folders.addSection')}</span>
      </div>

      <div className="card">
        <div className="settings-row">
          <input
            className="input inline"
            placeholder="Media/Podcasts"
            value={manual}
            onChange={(e) => onManualChange(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') add(manual) }}
          />
          <button type="button" className="button compact" onClick={() => add(manual)}>
            {t('common.add')}
          </button>
        </div>
        <div className="divider" />
        <button type="button" className="settings-link" onClick={onPickOnNas}>
          <span>{t('common.pickOnNas')}</span>
          <svg width="9" height="15" viewBox="0 0 9 15" fill="none" stroke="currentColor"
               strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M1.5 1.5L7 7.5l-5.5 6" />
          </svg>
        </button>
      </div>

      {suggested.length > 0 && (
        <>
          <div className="section-head">
            <span className="section-title">{t('folders.inUse')}</span>
          </div>
          <div className="chips">
            {suggested.map((folder) => (
              <button key={folder} type="button" className="chip" onClick={() => add(folder)}>
                + {folder}
              </button>
            ))}
          </div>
        </>
      )}

      <div className="card">
        <label className="settings-row toggle" htmlFor="recent">
          <span className="settings-text">
            <span>{t('folders.showRecent')}</span>
            <span className="muted small">{t('folders.showRecentHint')}</span>
          </span>
          <input
            id="recent"
            type="checkbox"
            role="switch"
            checked={showRecent}
            onChange={(e) => void apply({ show_recent: e.target.checked })}
          />
        </label>
      </div>

      <div className="section-head"><span className="section-title">{t('notify.title')}</span></div>
      <div className="card">
        <div className="muted small notify-hint">{t('notify.hint')}</div>
        {NOTIFY_MODES.map((mode, i) => (
          <div key={mode}>
            {i > 0 && <div className="divider" />}
            <label className="settings-row pick" htmlFor={`notify-${mode}`}>
              <span className="settings-text">
                <span>{t(`notify.${mode}` as const)}</span>
                <span className="muted small">{t(`notify.${mode}Hint` as const)}</span>
              </span>
              <input
                id={`notify-${mode}`}
                type="radio"
                name="notify"
                checked={notify === mode}
                onChange={() => void apply({}, mode)}
              />
            </label>
          </div>
        ))}
      </div>
    </div>
  )
}

// The order is deliberate: least to most. A person scanning the list stops at
// the first thing that sounds right, and "everything" should not be it.
const NOTIFY_MODES = ['off', 'downloads', 'all'] as const

function describe(e: unknown, fallback: string): string {
  if (e instanceof ApiError) return errorText(e, fallback)
  return fallback
}
