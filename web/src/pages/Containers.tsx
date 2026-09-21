import { useCallback, useEffect, useState } from 'react'
import { ScreenTitle } from '../components/ScreenTitle'
import { api, ApiError } from '../api'
import { size } from '../format'
import { alertMessage, backButton, haptic } from '../telegram'
import type { Container } from '../types'

interface Props {
  onBack: () => void
}

export function Containers({ onBack }: Props) {
  const [list, setList] = useState<Container[]>([])
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async () => {
    try {
      const data = await api.containers()
      setList(data.containers ?? [])
      setError(null)
    } catch (e) {
      setError(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получить список контейнеров')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function act(container: Container, action: 'start' | 'stop' | 'restart') {
    setBusy(container.name)
    try {
      await api.containerAction(container.name, action)
      haptic('success')
      await load()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получилось')
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="page">
      <div className="card summary">
        <ScreenTitle title="Контейнеры" onBack={onBack}>
          {list.length > 0 && (
            <span className="badge">{list.filter((c) => c.running).length} из {list.length}</span>
          )}
        </ScreenTitle>
      </div>

      {error && <div className="card error-card">{error}</div>}

      {!error && list.length === 0 && (
        <div className="card empty">
          <div className="empty-icon" aria-hidden="true">
            <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="10" width="5" height="5" rx="1" />
              <rect x="9.5" y="10" width="5" height="5" rx="1" />
              <rect x="16" y="10" width="5" height="5" rx="1" />
              <rect x="9.5" y="4" width="5" height="5" rx="1" />
              <path d="M3 18h18" />
            </svg>
          </div>
          <div className="empty-title">Контейнеров нет</div>
          <div className="empty-text">
            Container Manager работает, но ни один контейнер не создан.
          </div>
        </div>
      )}

      <div className="list">
        {list.map((c) => (
          <div key={c.id || c.name} className="card vm">
            <div className="vm-head">
              <span className={c.running ? 'dot on' : 'dot'} aria-hidden="true" />
              <div className="vm-text">
                <span className="vm-name">{c.name}</span>
                <span className="vm-specs tnum">{c.image}</span>
              </div>
              <button
                type="button"
                className={c.running ? 'icon-button danger' : 'icon-button ok'}
                disabled={busy === c.name}
                onClick={() => void act(c, c.running ? 'stop' : 'start')}
                aria-label={c.running ? 'Остановить' : 'Запустить'}
              >
                {c.running ? (
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                    <rect x="6" y="6" width="12" height="12" rx="2" />
                  </svg>
                ) : (
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                    <path d="M8 5.2v13.6a.8.8 0 0 0 1.23.67l10.4-6.8a.8.8 0 0 0 0-1.34L9.23 4.53A.8.8 0 0 0 8 5.2z" />
                  </svg>
                )}
              </button>
            </div>
            <div className="vm-foot">
              <span className={c.running ? 'vm-status on' : 'vm-status'}>
                {c.running ? 'Работает' : c.status || 'Остановлен'}
              </span>
              {c.running && (
                <span className="muted tnum">
                  {c.cpu.toFixed(1).replace('.', ',')}% · {size(c.memory)}
                </span>
              )}
              {c.running && (
                <button type="button" className="link-button" disabled={busy === c.name}
                        onClick={() => void act(c, 'restart')}>
                  перезапустить
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
