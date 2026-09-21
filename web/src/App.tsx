import { useCallback, useEffect, useState } from 'react'
import { Downloads } from './pages/Downloads'
import { Add, describeError, type AddDraft } from './pages/Add'
import { Files } from './pages/Files'
import { Folders } from './pages/Folders'
import { TaskDetail } from './pages/TaskDetail'
import { api, ApiError } from './api'
import { alertMessage, haptic } from './telegram'
import type { Overview, Task } from './types'

type Screen =
  | { name: 'downloads' }
  | { name: 'files' }
  | { name: 'add' }
  | { name: 'task'; id: string }
  | { name: 'folders' }
  // Обзор NAS в режиме выбора папки: открывается из настройки папок.
  | { name: 'pickFolder' }
  // Обзор NAS для выбора папки текущей загрузки, начиная с указанной.
  | { name: 'pickDestination'; from: string }
  // Обзор NAS для переноса уже созданной задачи.
  | { name: 'pickTaskFolder'; from: string; taskId: string }

/** Как часто обновлять список, когда что-то качается. */
const ACTIVE_POLL = 2500
const IDLE_POLL = 15000

export function App() {
  const [screen, setScreen] = useState<Screen>({ name: 'downloads' })
  const [data, setData] = useState<Overview | null>(null)
  const [error, setError] = useState<string | null>(null)
  // Содержимое экрана добавления: переход в обзор NAS размонтирует его, и без
  // этого введённая ссылка пропадала бы.
  const [addDraft, setAddDraft] = useState<AddDraft>({ link: '', destination: '' })
  const [sending, setSending] = useState(false)


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

  // Папка, выбранная в обзоре NAS, закрепляется сразу: экрана с кнопкой
  // «Сохранить» больше нет, и терять нечего.
  const pinFolder = useCallback(async (path: string) => {
    setScreen({ name: 'folders' })
    if (!path) return
    try {
      const settings = await api.settings()
      const pinned = settings.pinned_folders ?? []
      if (pinned.includes(path)) return
      await api.saveSettings({
        pinned_folders: [...pinned, path],
        show_recent: settings.show_recent,
        last_used: settings.last_used,
      })
      void refresh()
    } catch {
      // Экран настройки покажет актуальное состояние и сообщит об ошибке сам.
    }
  }, [refresh])

  // Папка по умолчанию подставляется, пока пользователь не выбрал свою.
  useEffect(() => {
    if (addDraft.destination || !data) return
    const fallback = data.settings?.last_used || data.folders?.[0] || data.default_destination
    if (fallback) setAddDraft((current) => ({ ...current, destination: fallback }))
  }, [data, addDraft.destination])

  const submit = useCallback(async () => {
    const urls = addDraft.link.split('\n').map((s) => s.trim()).filter(Boolean)
    if (urls.length === 0) {
      alertMessage('Вставьте ссылку')
      return
    }
    setSending(true)
    try {
      await api.create(urls, addDraft.destination)
      haptic('success')
      setAddDraft({ link: '', destination: addDraft.destination })
      void refresh()
      setScreen({ name: 'downloads' })
    } catch (e) {
      haptic('error')
      alertMessage(describeError(e))
    } finally {
      setSending(false)
    }
  }, [addDraft, refresh])

  // Перенос задачи в папку, выбранную в обзоре NAS.
  const moveTask = useCallback(async (taskId: string, path: string) => {
    setScreen({ name: 'task', id: taskId })
    if (!path) return
    try {
      await api.setDestination([taskId], path)
      haptic('success')
      void refresh()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не сменить папку')
    }
  }, [refresh])

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
        {screen.name === 'pickFolder' && (
          <Files
            pickMode
            onPick={(path) => { void pinFolder(path) }}
            onCancelPick={() => setScreen({ name: 'folders' })}
          />
        )}
        {screen.name === 'folders' && (
          <Folders
            onBack={() => setScreen({ name: 'add' })}
            onChanged={() => void refresh()}
            onPickOnNas={() => setScreen({ name: 'pickFolder' })}
          />
        )}
        {screen.name === 'add' && (
          <Add
            data={data}
            draft={addDraft}
            onDraftChange={setAddDraft}
            sending={sending}
            onSend={() => void submit()}
            onBack={() => setScreen({ name: 'downloads' })}
            onConfigureFolders={() => setScreen({ name: 'folders' })}
            onBrowse={(from) => setScreen({ name: 'pickDestination', from })}
          />
        )}
        {screen.name === 'pickDestination' && (
          <Files
            pickMode
            initialPath={screen.from}
            onPick={(path) => {
              setAddDraft((current) => ({ ...current, destination: path }))
              setScreen({ name: 'add' })
            }}
            onCancelPick={() => setScreen({ name: 'add' })}
          />
        )}
        {screen.name === 'task' && current && (
          <TaskDetail
            task={current}
            folders={data?.folders ?? []}
            onBack={() => setScreen({ name: 'downloads' })}
            onChanged={() => void refresh()}
            onBrowse={(from) => setScreen({ name: 'pickTaskFolder', from, taskId: current.id })}
          />
        )}
        {screen.name === 'pickTaskFolder' && (
          <Files
            pickMode
            initialPath={screen.from}
            onPick={(path) => { void moveTask(screen.taskId, path) }}
            onCancelPick={() => setScreen({ name: 'task', id: screen.taskId })}
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
