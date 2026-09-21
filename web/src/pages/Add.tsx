import { useEffect } from 'react'
import { ApiError } from '../api'
import { alertMessage, backButton, haptic } from '../telegram'
import { t } from '../i18n'
import type { Overview } from '../types'

/** Несохранённое содержимое экрана: живёт выше, чтобы пережить обзор NAS. */
export interface AddDraft {
  link: string
  destination: string
}

interface Props {
  data: Overview | null
  draft: AddDraft
  onDraftChange: (draft: AddDraft) => void
  onBack: () => void
  onConfigureFolders: () => void
  /** Открыть обзор NAS, начиная с указанной папки, чтобы выбрать другую. */
  onBrowse: (from: string) => void
  sending: boolean
  onSend: () => void
}

export function Add({
  data, draft, onDraftChange, onBack, onConfigureFolders, onBrowse, sending, onSend,
}: Props) {
  useEffect(() => backButton(onBack), [onBack])

  const folders = data?.folders ?? []
  const { link, destination } = draft

  // Папка, выбранная в обзоре, может не входить в список — показываем её
  // отдельной строкой, иначе выбор выглядел бы пропавшим.
  const shown = destination && !folders.includes(destination)
    ? [destination, ...folders]
    : folders

  async function paste() {
    try {
      const text = await navigator.clipboard.readText()
      if (text) onDraftChange({ ...draft, link: text.trim() })
    } catch {
      alertMessage(t('add.clipboardUnavailable'))
    }
  }

  return (
    <div className="page">
      <div className="card form">
        <h1 className="card-title">{t('add.title')}</h1>

        <label className="field-label" htmlFor="link">{t('add.linkLabel')}</label>
        <div className="input-wrap">
          <textarea
            id="link"
            rows={3}
            className="input"
            placeholder={t('add.linkPlaceholder')}
            value={link}
            onChange={(e) => onDraftChange({ ...draft, link: e.target.value })}
          />
          {link && (
            <button
              type="button"
              className="input-clear"
              onClick={() => { haptic('light'); onDraftChange({ ...draft, link: '' }) }}
              aria-label={t('add.clear')}
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="2.6" strokeLinecap="round" aria-hidden="true">
                <path d="M6 6l12 12" /><path d="M18 6L6 18" />
              </svg>
            </button>
          )}
        </div>

        <div className="row">
          <button type="button" className="button secondary" onClick={paste}>
            {t('add.fromClipboard')}
          </button>
        </div>
      </div>

      <div className="section-head row-between">
        <span className="section-title">{t('add.whereTo')}</span>
        <button type="button" className="link-button" onClick={onConfigureFolders}>
          {t('add.configure')}
        </button>
      </div>

      <div className="list">
        {shown.map((folder) => (
          <div
            key={folder}
            className={folder === destination ? 'card folder selected' : 'card folder'}
          >
            <button
              type="button"
              className="folder-main"
              onClick={() => onDraftChange({ ...draft, destination: folder })}
            >
              <span className="radio" aria-hidden="true">
                {folder === destination && <span className="radio-dot" />}
              </span>
              <span className="folder-name">{folder}</span>
            </button>
            <button
              type="button"
              className="icon-button small"
              onClick={() => onBrowse(folder)}
              aria-label={t('add.openFolderAria', { folder })}
            >
              <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4l2 2.5h7A1.5 1.5 0 0 1 19 10v7.5A1.5 1.5 0 0 1 17.5 19h-13A1.5 1.5 0 0 1 3 17.5z" />
              </svg>
            </button>
          </div>
        ))}

        <button type="button" className="card folder pick-any" onClick={() => onBrowse('')}>
          <span className="icon dir" aria-hidden="true">
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4l2 2.5h7A1.5 1.5 0 0 1 19 10v7.5A1.5 1.5 0 0 1 17.5 19h-13A1.5 1.5 0 0 1 3 17.5z" />
              <path d="M11 11.5v4" /><path d="M9 13.5h4" />
            </svg>
          </span>
          <span className="folder-name accent">{t('add.pickOnNas')}</span>
        </button>
      </div>

      <button
        type="button"
        className="main-button"
        disabled={sending || !link.trim()}
        onClick={onSend}
      >
        {sending ? t('add.submitting') : t('add.submit')}
      </button>
    </div>
  )
}

export function describeError(e: unknown): string {
  if (e instanceof ApiError) return e.detail ?? e.message
  return t('add.failed')
}
