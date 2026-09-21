import { useCallback, useEffect, useState } from 'react'
import { ProgressRing } from '../components/ProgressRing'
import { appearance } from '../components/TaskCard'
import { api, ApiError } from '../api'
import { eta, size, speed, statusLabel, taskTitle } from '../format'
import { alertMessage, backButton, confirmAction, haptic } from '../telegram'
import type { FilePriority, Task, TaskDetails } from '../types'

interface Props {
  task: Task
  /** Папки для переноса: закреплённые пользователем. */
  folders: string[]
  onBack: () => void
  onChanged: () => void
  /** Выбрать папку в обзоре NAS. */
  onBrowse: (from: string) => void
}

const PRIORITIES: Array<[FilePriority, string]> = [
  ['low', 'Низкий'],
  ['normal', 'Обычный'],
  ['high', 'Высокий'],
]

export function TaskDetail({ task, folders, onBack, onChanged, onBrowse }: Props) {
  const [details, setDetails] = useState<TaskDetails | null>(null)
  const [busy, setBusy] = useState(false)
  const [showFolders, setShowFolders] = useState(false)

  useEffect(() => backButton(onBack), [onBack])

  const look = appearance(task.status)
  const paused = task.status === 'paused' || task.status === 'error'
  const canToggle = task.status !== 'finished'

  const load = useCallback(async () => {
    try {
      setDetails(await api.taskDetails(task.id))
    } catch (e) {
      // Подробности не критичны: основной экран остаётся полезным.
      setDetails({ files: null, trackers: null })
      if (!(e instanceof ApiError)) return
    }
  }, [task.id])

  useEffect(() => { void load() }, [load])

  async function run(action: () => Promise<unknown>, after?: () => void) {
    setBusy(true)
    try {
      await action()
      haptic('success')
      onChanged()
      await load()
      after?.()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получилось')
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    if (!(await confirmAction('Удалить задачу? Это необратимо.'))) return
    await run(() => api.action('delete', [task.id]))
    onBack()
  }

  const files = details?.files ?? []
  const notActive = details?.not_active === true

  return (
    <div className="page">
      <div className="card detail">
        <ProgressRing value={task.progress} color={look.color} size={132} />
        <div className="detail-percent tnum">{Math.round(task.progress * 100)}%</div>
        <div className="detail-status" style={{ color: look.color }}>
          {statusLabel(task.status)}
          {task.eta_seconds ? ` · осталось ${eta(task.eta_seconds)}` : ''}
        </div>

        <div className="detail-title">{taskTitle(task.title)}</div>
        <div className="muted tnum detail-sub">
          {size(task.downloaded)} из {size(task.size)} · {speed(task.speed_down)}
        </div>

        <div className="row">
          {canToggle && (
            <button
              type="button"
              className="button secondary"
              disabled={busy}
              onClick={() => void run(() => api.action(paused ? 'resume' : 'pause', [task.id]))}
            >
              {paused ? 'Возобновить' : 'Пауза'}
            </button>
          )}
          <button type="button" className="button danger" disabled={busy} onClick={() => void remove()}>
            Удалить
          </button>
        </div>
      </div>

      <div className="tiles grid">
        {([
          ['Сиды / пиры', `${task.seeders} / ${task.leechers}`],
          ['Роздано', size(task.uploaded)],
        ] as Array<[string, string]>).map(([label, value]) => (
          <div key={label} className="card tile-card">
            <span className="tile-label">{label}</span>
            <span className="tile-value tnum">{value}</span>
          </div>
        ))}
      </div>

      <div className="section-head">
        <span className="section-title">Приоритет в очереди</span>
      </div>
      <div className="card">
        <div className="priority-row">
          {PRIORITIES.map(([value, label]) => (
            <button
              key={value}
              type="button"
              className="chip"
              disabled={busy}
              onClick={() => void run(() => api.setPriority([task.id], value))}
            >
              {label}
            </button>
          ))}
        </div>
        {/*
          Download Station принимает приоритет задачи, но не возвращает его ни
          в одном ответе — показать текущее значение нечем, поэтому это
          кнопки-действия, а не переключатель с отмеченным состоянием.
        */}
        <div className="muted small priority-note">
          NAS не сообщает текущий приоритет — кнопки задают новый.
        </div>
      </div>

      <div className="section-head row-between">
        <span className="section-title">Папка</span>
        <button type="button" className="link-button" onClick={() => setShowFolders((v) => !v)}>
          {showFolders ? 'Свернуть' : 'Перенести'}
        </button>
      </div>
      <div className="card">
        <div className="settings-row">
          <span className="entry-name">{task.destination || '—'}</span>
        </div>
        {showFolders && (
          <>
            <div className="divider" />
            {folders.filter((f) => f !== task.destination).map((folder) => (
              <button
                key={folder}
                type="button"
                className="settings-link"
                disabled={busy}
                onClick={() => void run(
                  () => api.setDestination([task.id], folder),
                  () => setShowFolders(false),
                )}
              >
                <span>{folder}</span>
                <span className="muted">перенести</span>
              </button>
            ))}
            <div className="divider" />
            <button type="button" className="settings-link" onClick={() => onBrowse(task.destination)}>
              <span>Выбрать на NAS</span>
              <svg width="9" height="15" viewBox="0 0 9 15" fill="none" stroke="currentColor"
                   strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M1.5 1.5L7 7.5l-5.5 6" />
              </svg>
            </button>
          </>
        )}
      </div>

      <div className="section-head">
        <span className="section-title">
          Файлы{files.length > 0 ? ` · ${files.length}` : ''}
        </span>
      </div>

      {notActive && (
        <div className="card empty-text">
          Состав раздачи виден, только пока задача качается или раздаётся: NAS
          закрывает её при остановке. Возобновите задачу, чтобы увидеть файлы.
        </div>
      )}

      {!notActive && files.length === 0 && (
        <div className="card empty-text">Получаю список файлов…</div>
      )}

      <div className="list">
        {files.map((file) => (
          <div key={file.index} className={file.wanted ? 'card file' : 'card file skipped'}>
            <div className="file-head">
              <label className="file-check">
                <input
                  type="checkbox"
                  checked={file.wanted}
                  disabled={busy}
                  onChange={(e) => void run(
                    () => api.setFile(task.id, [file.index], { wanted: e.target.checked }),
                  )}
                />
              </label>
              <span className="file-name">{file.name}</span>
            </div>

            <div className="bar thin">
              <div className="bar-fill" style={{ width: `${Math.round(file.progress * 100)}%` }} />
            </div>

            <div className="file-foot">
              <span className="muted tnum">
                {size(file.downloaded)} из {size(file.size)}
              </span>
              <div className="file-priority">
                {PRIORITIES.map(([value, label]) => (
                  <button
                    key={value}
                    type="button"
                    className={file.priority === value ? 'chip small on' : 'chip small'}
                    disabled={busy || !file.wanted}
                    onClick={() => void run(
                      () => api.setFile(task.id, [file.index], { priority: value }),
                    )}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>

      {details?.trackers && details.trackers.length > 0 && (
        <>
          <div className="section-head">
            <span className="section-title">Трекеры</span>
          </div>
          <div className="card">
            {details.trackers.map((tracker, i) => (
              <div key={tracker.url}>
                {i > 0 && <div className="divider" />}
                <div className="tracker">
                  <span className="tracker-url">{tracker.url}</span>
                  <span className="muted tnum small">
                    {tracker.status} · сидов {tracker.seeds} · пиров {tracker.peers}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
