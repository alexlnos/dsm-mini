import { useCallback, useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { alertMessage, backButton, haptic } from '../telegram'
import type { Settings, SettingsView } from '../types'

interface Props {
  onBack: () => void
  /** Настройки изменились — обновить данные на остальных экранах. */
  onChanged: () => void
  /** Открыть обзор NAS, чтобы выбрать папку там. */
  onPickOnNas: () => void
}

/**
 * Настройка папок назначения.
 *
 * Изменения сохраняются сразу: кнопки «Сохранить» нет, поэтому нечего
 * потерять при переходе в обзор NAS или закрытии приложения. Интерфейс
 * обновляется до ответа сервера, а при ошибке возвращается к прежнему
 * состоянию — иначе каждое нажатие ощущалось бы как задержка.
 */
export function Folders({ onBack, onChanged, onPickOnNas }: Props) {
  const [view, setView] = useState<SettingsView | null>(null)
  const [manual, setManual] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async () => {
    try {
      setView(await api.settings())
      setError(null)
    } catch (e) {
      setError(describe(e, 'Не загрузить настройки'))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const pinned = view?.pinned_folders ?? []
  const showRecent = view?.show_recent ?? true

  const apply = useCallback(
    async (next: Partial<Settings>) => {
      if (!view) return
      const previous = view
      const payload: Settings = {
        pinned_folders: next.pinned_folders ?? pinned,
        show_recent: next.show_recent ?? showRecent,
        last_used: view.last_used,
      }
      // Показываем результат сразу, не дожидаясь сервера.
      setView({ ...view, ...payload })
      try {
        setView(await api.saveSettings(payload))
        setError(null)
        onChanged()
      } catch (e) {
        setView(previous)
        haptic('error')
        alertMessage(describe(e, 'Не сохранить'))
      }
    },
    [view, pinned, showRecent, onChanged],
  )

  function add(folder: string) {
    const clean = folder.trim().replace(/^\/+|\/+$/g, '')
    if (!clean) return
    if (pinned.includes(clean)) {
      alertMessage('Такая папка уже закреплена')
      return
    }
    setManual('')
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
        <h1>Папки</h1>
        <div className="muted">
          Появятся на экране добавления в этом порядке. Изменения сохраняются сразу.
        </div>
      </div>

      {error && <div className="card error-card">{error}</div>}

      <div className="section-head">
        <span className="section-title">Закреплённые</span>
      </div>

      <div className="list">
        {pinned.map((folder, i) => (
          <div key={folder} className="card entry">
            <button
              type="button"
              className="icon-button small danger"
              onClick={() => unpin(folder)}
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
            onChange={(e) => void apply({ show_recent: e.target.checked })}
          />
        </label>
      </div>
    </div>
  )
}

function describe(e: unknown, fallback: string): string {
  if (e instanceof ApiError) return e.detail ?? e.message
  return fallback
}
