import { useCallback, useEffect, useState } from 'react'
import { ProgressRing } from '../components/ProgressRing'
import { appearance } from '../components/TaskCard'
import { api, errorText } from '../api'
import { eta, size, speed, statusLabel, taskTitle } from '../format'
import { t } from '../i18n'
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

/** Подписи считаются при отрисовке: язык известен только в рантайме. */
function priorities(): Array<[FilePriority, string]> {
  return [
    ['low', t('task.priorityLow')],
    ['normal', t('task.priorityNormal')],
    ['high', t('task.priorityHigh')],
  ]
}

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
    } catch {
      // Подробности не критичны: основной экран остаётся полезным.
      setDetails({ files: null, trackers: null })
    }
  }, [task.id])

  // Перезапрашиваем состав не только при открытии, но и когда меняется
  // состояние задачи: после снятия с паузы NAS заново открывает BT-сессию,
  // и файлы, которых только что не было, появляются.
  useEffect(() => { void load() }, [load, task.status])

  // Пока задача работает, состав подтягивается сам: список приходит не
  // мгновенно после возобновления, а доли загруженного меняются на ходу.
  useEffect(() => {
    if (!task.active) return
    const timer = window.setInterval(() => { void load() }, 5000)
    return () => window.clearInterval(timer)
  }, [task.active, load])

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
      alertMessage(errorText(e, t('common.failed')))
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    if (!(await confirmAction(t('task.confirmDelete')))) return
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
          {task.eta_seconds ? t('task.etaLeft', { eta: eta(task.eta_seconds) }) : ''}
        </div>

        <div className="detail-title">{taskTitle(task.title)}</div>
        <div className="muted tnum detail-sub">
          {t('common.outOf', { value: size(task.downloaded), total: size(task.size) })}
          {' · '}{speed(task.speed_down)}
        </div>

        <div className="row">
          {canToggle && (
            <button
              type="button"
              className="button secondary"
              disabled={busy}
              onClick={() => void run(() => api.action(paused ? 'resume' : 'pause', [task.id]))}
            >
              {paused ? t('task.resume') : t('task.pause')}
            </button>
          )}
          <button type="button" className="button danger" disabled={busy} onClick={() => void remove()}>
            {t('common.delete')}
          </button>
        </div>
      </div>

      <div className="tiles grid">
        {([
          [t('task.seedsPeers'), `${task.seeders} / ${task.leechers}`],
          [t('task.uploaded'), size(task.uploaded)],
        ] as Array<[string, string]>).map(([label, value]) => (
          <div key={label} className="card tile-card">
            <span className="tile-label">{label}</span>
            <span className="tile-value tnum">{value}</span>
          </div>
        ))}
      </div>

      <div className="section-head row-between">
        <span className="section-title">{t('task.folder')}</span>
        <button type="button" className="link-button" onClick={() => setShowFolders((v) => !v)}>
          {showFolders ? t('task.collapse') : t('task.moveTo')}
        </button>
      </div>
      <div className="card">
        <div className="settings-row">
          <span className="entry-name">{task.destination || t('common.dash')}</span>
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
                <span className="muted">{t('task.moveHint')}</span>
              </button>
            ))}
            <div className="divider" />
            <button type="button" className="settings-link" onClick={() => onBrowse(task.destination)}>
              <span>{t('common.pickOnNas')}</span>
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
          {t('task.files')}{files.length > 0 ? ` · ${files.length}` : ''}
        </span>
      </div>

      {notActive && (
        <div className="card empty-text">
          {t('task.filesClosed')}
        </div>
      )}

      {!notActive && files.length === 0 && (
        <div className="card empty-text">
          {task.active
            ? t('task.filesLoading')
            : t('task.filesEmpty')}
        </div>
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
                {t('common.outOf', { value: size(file.downloaded), total: size(file.size) })}
              </span>
              <div className="file-priority">
                {priorities().map(([value, label]) => (
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
            <span className="section-title">{t('task.trackers')}</span>
          </div>
          <div className="card">
            {details.trackers.map((tracker, i) => (
              <div key={tracker.url}>
                {i > 0 && <div className="divider" />}
                <div className="tracker">
                  <span className="tracker-url">{tracker.url}</span>
                  <span className="muted tnum small">
                    {t('task.trackerLine', {
                      status: tracker.status,
                      seeds: tracker.seeds,
                      peers: tracker.peers,
                    })}
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
