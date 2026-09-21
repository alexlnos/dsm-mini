import { useCallback, useEffect, useState } from 'react'
import { Downloads } from './pages/Downloads'
import { Add } from './pages/Add'
import { Files } from './pages/Files'
import { TaskDetail } from './pages/TaskDetail'
import { api, ApiError } from './api'
import type { Overview, Task } from './types'

type Screen =
  | { name: 'downloads' }
  | { name: 'files' }
  | { name: 'add' }
  | { name: 'task'; id: string }

/** Как часто обновлять список, когда что-то качается. */
const ACTIVE_POLL = 2500
const IDLE_POLL = 15000

export function App() {
  const [screen, setScreen] = useState<Screen>({ name: 'downloads' })
  const [data, setData] = useState<Overview | null>(null)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      setData(await api.overview())
      setError(null)
    } catch (e) {
      if (e instanceof ApiError && e.status === 403) {
        setError('Доступ закрыт: ваш Telegram ID не в списке разрешённых.')
      } else if (e instanceof ApiError && e.status === 401) {
        setError('Откройте приложение через бота — Telegram не подтвердил вход.')
      } else {
        setError(e instanceof ApiError ? (e.detail ?? e.message) : 'Сервис недоступен')
      }
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  // Частый опрос только когда есть живые задачи: иначе Mini App зря будит
  // NAS каждые пару секунд.
  useEffect(() => {
    const anyActive = data?.tasks.some((t) => t.active) ?? false
    const delay = anyActive ? ACTIVE_POLL : IDLE_POLL
    const timer = window.setInterval(() => { void refresh() }, delay)
    return () => window.clearInterval(timer)
  }, [data, refresh])

  const openTask = (task: Task) => setScreen({ name: 'task', id: task.id })
  const current = screen.name === 'task'
    ? data?.tasks.find((t) => t.id === screen.id)
    : undefined

  // Задача могла исчезнуть, пока экран открыт.
  useEffect(() => {
    if (screen.name === 'task' && data && !current) setScreen({ name: 'downloads' })
  }, [screen, data, current])

  return (
    <div className="app">
      <main className="content">
        {screen.name === 'downloads' && (
          <Downloads
            data={data}
            error={error}
            onRefresh={() => void refresh()}
            onOpen={openTask}
            onAdd={() => setScreen({ name: 'add' })}
          />
        )}
        {screen.name === 'files' && <Files />}
        {screen.name === 'add' && (
          <Add
            data={data}
            onDone={() => { void refresh(); setScreen({ name: 'downloads' }) }}
            onBack={() => setScreen({ name: 'downloads' })}
          />
        )}
        {screen.name === 'task' && current && (
          <TaskDetail
            task={current}
            onBack={() => setScreen({ name: 'downloads' })}
            onChanged={() => void refresh()}
          />
        )}
      </main>

      {(screen.name === 'downloads' || screen.name === 'files') && (
        <nav className="tabbar">
          <button
            type="button"
            className={screen.name === 'downloads' ? 'tab active' : 'tab'}
            onClick={() => setScreen({ name: 'downloads' })}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2.1" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M12 4v11" /><path d="M7.5 10.5L12 15l4.5-4.5" /><path d="M4 19h16" />
            </svg>
            <span>Загрузки</span>
          </button>
          <button
            type="button"
            className={screen.name === 'files' ? 'tab active' : 'tab'}
            onClick={() => setScreen({ name: 'files' })}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2.1" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4l2 2.5h7A1.5 1.5 0 0 1 19 10v7.5A1.5 1.5 0 0 1 17.5 19h-13A1.5 1.5 0 0 1 3 17.5z" />
            </svg>
            <span>Файлы</span>
          </button>
        </nav>
      )}
    </div>
  )
}
