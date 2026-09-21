import { useCallback, useEffect, useState } from 'react'
import { Downloads, type DownloadsFilter } from './pages/Downloads'
import { Home, type Section } from './pages/Home'
import { VMs } from './pages/VMs'
import { Containers } from './pages/Containers'
import { Storage } from './pages/Storage'
import { Notifications } from './pages/Notifications'
import { Add, describeError, type AddDraft } from './pages/Add'
import { Files } from './pages/Files'
import { Folders } from './pages/Folders'
import { TaskDetail } from './pages/TaskDetail'
import { api, ApiError, errorText } from './api'
import { readCache, writeCache } from './cache'
import { alertMessage, haptic } from './telegram'
import { t } from './i18n'
import type { Overview, Task } from './types'

type Screen =
  | { name: 'home' }
  | { name: 'vms' }
  | { name: 'containers' }
  | { name: 'storage' }
  | { name: 'notifications' }
  | { name: 'downloads' }
  | { name: 'files' }
  | { name: 'add' }
  | { name: 'task'; id: string }
  | { name: 'folders' }
  // The NAS browser in folder-picking mode: opened from the folder settings.
  | { name: 'pickFolder' }
  // The NAS browser for picking the folder of the current download, from a start.
  | { name: 'pickDestination'; from: string }
  // The NAS browser for moving an already created task.
  | { name: 'pickTaskFolder'; from: string; taskId: string }
  // Picking the folder to copy or move the selected files into.
  | { name: 'transferTarget'; paths: string[]; move: boolean }

/** Screens that belong to the downloads tab. */
function inDownloads(name: Screen['name']): boolean {
  return name === 'downloads' || name === 'add' || name === 'folders' || name === 'task'
}

/** How often to refresh the list while something is downloading. */
const ACTIVE_POLL = 2500
const IDLE_POLL = 15000

export function App() {
  const [screen, setScreen] = useState<Screen>({ name: 'home' })
  const [data, setData] = useState<Overview | null>(() => readCache('overview'))
  const [error, setError] = useState<string | null>(null)
  // Contents of the add screen: going to the NAS browser unmounts it, and
  // without this the typed link would be lost.
  const [addDraft, setAddDraft] = useState<AddDraft>({ link: '', destination: '' })
  const [sending, setSending] = useState(false)
  // The open folder survives switching tabs: sending the person back to the
  // root every time means making them walk the same path again.
  const [filesPath, setFilesPath] = useState('/')
  // A counter rebuilds the browser: pressing the active tab again opens the
  // section anew, from the root.
  const [filesRun, setFilesRun] = useState(0)
  // The chosen downloads tab survives navigation too. null means the person
  // has not chosen yet, so a sensible one can be filled in.
  const [downloadsFilter, setDownloadsFilter] = useState<DownloadsFilter | null>(null)
  // The path typed by hand in the folder settings: the screen unmounts when
  // going to the NAS browser or another tab, and the text would be lost.
  const [folderDraft, setFolderDraft] = useState('')
  // Where the person left the downloads tab: the list, adding, folder
  // settings or an open task. Returning to the tab brings them back there.
  const [downloadsScreen, setDownloadsScreen] = useState<Screen>({ name: 'downloads' })


  const refresh = useCallback(async () => {
    try {
      const fresh = await api.overview()
      setData(fresh)
      writeCache('overview', fresh)
      setError(null)
    } catch (e) {
      if (e instanceof ApiError && e.status === 403) {
        setError(t('access.forbidden'))
      } else if (e instanceof ApiError && e.status === 401) {
        setError(t('access.unauthorized'))
      } else {
        setError(errorText(e, t('access.unavailable')))
      }
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  // Frequent polling only while there are live tasks: otherwise the Mini App
  // wakes the NAS every couple of seconds for nothing.
  useEffect(() => {
    const anyActive = data?.tasks.some((t) => t.active) ?? false
    const delay = anyActive ? ACTIVE_POLL : IDLE_POLL
    const timer = window.setInterval(() => { void refresh() }, delay)
    return () => window.clearInterval(timer)
  }, [data, refresh])

  // A folder picked in the NAS browser is pinned at once: there is no screen
  // with a Save button any more, so there is nothing to lose.
  const pinFolder = useCallback(async (path: string) => {
    if (path) {
      try {
        const settings = await api.settings()
        const pinned = settings.pinned_folders ?? []
        if (!pinned.includes(path)) {
          await api.saveSettings({
            pinned_folders: [...pinned, path],
            show_recent: settings.show_recent,
            last_used: settings.last_used,
          })
        }
        await refresh()
      } catch {
        // The settings screen will show the current state and report the error itself.
      }
    }
    // Only now: the settings screen reads the folders when it mounts, so
    // switching to it before the write lands showed the list as it was a
    // moment ago — the folder just picked was missing from it.
    setScreen({ name: 'folders' })
  }, [refresh])

  // The default folder is filled in until the user picks their own.
  useEffect(() => {
    if (addDraft.destination || !data) return
    const fallback = data.settings?.last_used || data.folders?.[0] || data.default_destination
    if (fallback) setAddDraft((current) => ({ ...current, destination: fallback }))
  }, [data, addDraft.destination])

  const submit = useCallback(async () => {
    const urls = addDraft.link.split('\n').map((s) => s.trim()).filter(Boolean)
    if (urls.length === 0) {
      alertMessage(t('add.needLink'))
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

  // Moving a task into the folder picked in the NAS browser.
  const moveTask = useCallback(async (taskId: string, path: string) => {
    setScreen({ name: 'task', id: taskId })
    if (!path) return
    try {
      await api.setDestination([taskId], path)
      haptic('success')
      void refresh()
    } catch (e) {
      haptic('error')
      alertMessage(errorText(e, t('task.moveFailed')))
    }
  }, [refresh])

  const openTask = (task: Task) => setScreen({ name: 'task', id: task.id })
  const current = screen.name === 'task'
    ? data?.tasks.find((t) => t.id === screen.id)
    : undefined

  // The task may have vanished while the screen was open.
  useEffect(() => {
    if (screen.name === 'task' && data && !current) setScreen({ name: 'downloads' })
  }, [screen, data, current])

  // Adding, folder settings and an open task are parts of the downloads
  // section rather than separate places: the tab bar stays on them, otherwise
  // the only way out was back.
  const onDownloads = inDownloads(screen.name)
  const showTabbar = onDownloads || screen.name === 'home' || screen.name === 'files'

  useEffect(() => {
    if (inDownloads(screen.name)) setDownloadsScreen(screen)
  }, [screen])

  return (
    <div className="app">
      <main className="content">
        {screen.name === 'home' && (
          <Home
            downloads={data}
            onOpen={(section: Section) => {
              // One by one: every screen has its own type in the union.
              switch (section) {
                case 'downloads': setScreen({ name: 'downloads' }); break
                case 'files': setScreen({ name: 'files' }); break
                case 'vms': setScreen({ name: 'vms' }); break
                case 'containers': setScreen({ name: 'containers' }); break
                case 'storage': setScreen({ name: 'storage' }); break
                case 'notifications': setScreen({ name: 'notifications' }); break
              }
            }}
          />
        )}
        {screen.name === 'vms' && <VMs onBack={() => setScreen({ name: 'home' })} />}
        {screen.name === 'containers' && <Containers onBack={() => setScreen({ name: 'home' })} />}
        {screen.name === 'storage' && <Storage onBack={() => setScreen({ name: 'home' })} />}
        {screen.name === 'notifications' && (
          <Notifications onBack={() => setScreen({ name: 'home' })} />
        )}
        {screen.name === 'downloads' && (
          <Downloads
            data={data}
            error={error}
            filter={downloadsFilter}
            onFilterChange={setDownloadsFilter}
            onRefresh={() => void refresh()}
            onOpen={openTask}
            onAdd={() => setScreen({ name: 'add' })}
          />
        )}
        {screen.name === 'files' && (
          <Files
            key={filesRun}
            initialPath={filesPath}
            onPathChange={setFilesPath}
            onTransfer={(paths, move) => setScreen({ name: 'transferTarget', paths, move })}
          />
        )}
        {screen.name === 'transferTarget' && (
          <Files
            pickMode
            pending={{ paths: screen.paths, move: screen.move }}
            onPick={() => setScreen({ name: 'files' })}
            onCancelPick={() => setScreen({ name: 'files' })}
          />
        )}
        {screen.name === 'pickFolder' && (
          <Files
            pickMode
            onPick={(path) => { void pinFolder(path) }}
            onCancelPick={() => setScreen({ name: 'folders' })}
          />
        )}
        {screen.name === 'folders' && (
          <Folders
            manual={folderDraft}
            onManualChange={setFolderDraft}
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

      {showTabbar && (
        <nav className="tabbar">
          <button
            type="button"
            className={screen.name === 'home' ? 'tab active' : 'tab'}
            onClick={() => setScreen({ name: 'home' })}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2.1" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M4 10.5L12 4l8 6.5" /><path d="M6 10v9h12v-9" />
            </svg>
            <span>{t('tab.home')}</span>
          </button>
          <button
            type="button"
            className={onDownloads ? 'tab active' : 'tab'}
            onClick={() => {
              // Pressing it again goes to the start of the section; from
              // another tab it returns where the person was interrupted.
              setScreen(onDownloads ? { name: 'downloads' } : downloadsScreen)
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2.1" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M12 4v11" /><path d="M7.5 10.5L12 15l4.5-4.5" /><path d="M4 19h16" />
            </svg>
            <span>{t('tab.downloads')}</span>
          </button>
          <button
            type="button"
            className={screen.name === 'files' ? 'tab active' : 'tab'}
            onClick={() => {
              // Pressing the tab you are already on returns to the start of
              // the section — familiar behaviour in mobile apps.
              if (screen.name === 'files') {
                setFilesPath('/')
                setFilesRun((run) => run + 1)
                return
              }
              setScreen({ name: 'files' })
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2.1" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4l2 2.5h7A1.5 1.5 0 0 1 19 10v7.5A1.5 1.5 0 0 1 17.5 19h-13A1.5 1.5 0 0 1 3 17.5z" />
            </svg>
            <span>{t('tab.files')}</span>
          </button>
        </nav>
      )}
    </div>
  )
}
