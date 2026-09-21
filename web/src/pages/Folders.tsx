import { useCallback, useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { alertMessage, backButton, haptic } from '../telegram'
import type { Settings, SettingsView } from '../types'

interface Props {
  onBack: () => void
  onSaved: () => void
  /** Открыть обзор NAS, чтобы выбрать папку там. */
  onPickOnNas: () => void
  /** Папка, выбранная в обзоре NAS и ожидающая добавления. */
  incoming?: string
}

export function Folders({ onBack, onSaved, onPickOnNas, incoming }: Props) {
  const [view, setView] = useState<SettingsView | null>(null)
  const [pinned, setPinned] = useState<string[]>([])
  const [showRecent, setShowRecent] = useState(true)
  const [manual, setManual] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async () => {
    try {
      const data = await api.settings()
      setView(data)
      setPinned(data.pinned_folders ?? [])
      setShowRecent(data.show_recent)
    } catch (e) {
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не загрузить настройки')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  // Папка, выбранная в обзоре NAS, добавляется сразу по возвращении.
  useEffect(() => {
    if (!incoming) return
    setPinned((current) => (current.includes(incoming) ? current : [...current, incoming]))
  }, [incoming])

  function add(folder: string) {
    const clean = folder.trim().replace(/^\/+|\/+$/g, '')
    if (!clean) return
    if (pinned.includes(clean)) {
      alertMessage('Такая папка уже закреплена')
      return
    }
    setPinned([...pinned, clean])
    setManual('')
    haptic('light')
  }

  function move(index: number, delta: number) {
    const target = index + delta
    if (target < 0 || target >= pinned.length) return
    const next = [...pinned]
    ;[next[index], next[target]] = [next[target], next[index]]
    setPinned(next)
    haptic('light')
  }

  async function save() {
    setSaving(true)
    const payload: Settings = {
      pinned_folders: pinned,
      show_recent: showRecent,
      last_used: view?.last_used ?? '',
    }
    try {
      await api.saveSettings(payload)
      haptic('success')
      onSaved()
      onBack()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не сохранить')
    } finally {
      setSaving(false)
    }
  }

  const suggested = (view?.suggested ?? []).filter((f) => !pinned.includes(f))

  return (
    <div className="page">
      <div className="card summary">
        <h1>Папки</h1>
        <div className="muted">
          Эти папки появятся на экране добавления в том же порядке.
        </div>
      </div>

      <div className="section-head">
        <span className="section-title">Закреплённые</span>
      </div>

      <div className="list">
        {pinned.map((folder, i) => (
          <div key={folder} className="card entry">
            <button
              type="button"
              className="icon-button small danger"
              onClick={() => setPinned(pinned.filter((f) => f !== folder))}
              aria-label={`Убрать ${folder}`}
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
                    disabled={i === 0} aria-label="Выше">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M6 14l6-6 6 6" />
              </svg>
            </button>
            <button type="button" className="icon-button small" onClick={() => move(i, 1)}
                    disabled={i === pinned.length - 1} aria-label="Ниже">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M6 10l6 6 6-6" />
              </svg>
            </button>
          </div>
        ))}

        {pinned.length === 0 && (
          <div className="card empty-text">
            Ничего не закреплено — на экране добавления покажутся недавние папки.
          </div>
        )}
      </div>

      <div className="section-head">
        <span className="section-title">Добавить</span>
      </div>

      <div className="card">
        <div className="settings-row">
          <input
            className="input inline"
            placeholder="Media/Podcasts"
            value={manual}
            onChange={(e) => setManual(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') add(manual) }}
          />
          <button type="button" className="button compact" onClick={() => add(manual)}>
            Добавить
          </button>
        </div>
        <div className="divider" />
        <button type="button" className="settings-link" onClick={onPickOnNas}>
          <span>Выбрать на NAS</span>
          <svg width="9" height="15" viewBox="0 0 9 15" fill="none" stroke="currentColor"
               strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M1.5 1.5L7 7.5l-5.5 6" />
          </svg>
        </button>
      </div>

      {suggested.length > 0 && (
        <>
          <div className="section-head">
            <span className="section-title">Используются сейчас</span>
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
            <span>Показывать недавние</span>
            <span className="muted small">
              Папки последних задач появятся под закреплёнными
            </span>
          </span>
          <input
            id="recent"
            type="checkbox"
            role="switch"
            checked={showRecent}
            onChange={(e) => setShowRecent(e.target.checked)}
          />
        </label>
      </div>

      <button type="button" className="main-button" disabled={saving} onClick={() => void save()}>
        {saving ? 'Сохраняю…' : 'Сохранить'}
      </button>
    </div>
  )
}
