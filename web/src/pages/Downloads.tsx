import { useEffect, useMemo, useState } from 'react'
import { TaskCard } from '../components/TaskCard'
import { SkeletonTasks } from '../components/Skeleton'
import { api, ApiError, errorText } from '../api'
import { size, speed } from '../format'
import { t } from '../i18n'
import { alertMessage, haptic } from '../telegram'
import type { Overview, Task } from '../types'

export type DownloadsFilter = 'active' | 'done' | 'all'

interface Props {
  data: Overview | null
  error: string | null
  /** The chosen tab; null means the person has not chosen yet. */
  filter: DownloadsFilter | null
  onFilterChange: (filter: DownloadsFilter) => void
  onRefresh: () => void
  onOpen: (task: Task) => void
  onAdd: () => void
}

export function Downloads({
  data, error, filter, onFilterChange, onRefresh, onOpen, onAdd,
}: Props) {
  const [busy, setBusy] = useState<string | null>(null)

  const tasks = data?.tasks ?? []

  const groups = useMemo(() => {
    const active = tasks.filter((t) => t.active || t.status === 'paused')
    return { active, done: tasks.filter((t) => !active.includes(t)), all: tasks }
  }, [tasks])

  const shownFilter = filter ?? 'active'
  const shown = groups[shownFilter]
  const activeCount = tasks.filter((t) => t.active).length

  // When there are no active tasks but finished ones exist, we fill in "All":
  // an empty screen over a non-empty list is confusing. Only until the person
  // picks a tab themselves — their choice beats our guess.
  useEffect(() => {
    if (filter !== null || !data) return
    onFilterChange(groups.active.length === 0 && groups.all.length > 0 ? 'all' : 'active')
  }, [data, groups, filter, onFilterChange])
  const volume = data?.volumes?.[0]

  async function toggle(task: Task) {
    const paused = task.status === 'paused' || task.status === 'error'
    setBusy(task.id)
    haptic('light')
    try {
      await api.action(paused ? 'resume' : 'pause', [task.id])
      onRefresh()
    } catch (e) {
      haptic('error')
      alertMessage(describe(e))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="page">
      <div className="card summary">
        <div className="summary-head">
          <h1>{t('downloads.title')}</h1>
          {activeCount > 0 && (
            <span className="badge">{t('downloads.activeBadge', { count: activeCount })}</span>
          )}
        </div>

        <div className="tiles">
          <div className="tile">
            <span className="tile-label ok">{t('downloads.rx')}</span>
            <span className="tile-value tnum">{speed(data?.stats.speed_down ?? 0)}</span>
          </div>
          <div className="tile">
            <span className="tile-label">{t('downloads.tx')}</span>
            <span className="tile-value tnum">{speed(data?.stats.speed_up ?? 0)}</span>
          </div>
        </div>

        {volume && (
          <div className="volume">
            <div className="volume-head">
              <span className="muted">{volume.mount_point}</span>
              <span className="muted tnum">
                {t('downloads.freeOf', {
                  free: size(volume.size_free),
                  total: size(volume.size_total),
                })}
              </span>
            </div>
            <div className="bar">
              <div
                className="bar-fill"
                style={{
                  width: `${Math.round(((volume.size_total - volume.size_free) / volume.size_total) * 100)}%`,
                }}
              />
            </div>
          </div>
        )}
      </div>

      <div className="segments" role="tablist">
        {([
          ['active', t('downloads.tabActive')],
          ['done', t('downloads.tabDone')],
          ['all', t('downloads.tabAll')],
        ] as const).map(([id, label]) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={shownFilter === id}
            className={shownFilter === id ? 'segment active' : 'segment'}
            onClick={() => onFilterChange(id)}
          >
            {label}
            {groups[id].length > 0 && (
              <span className="segment-count tnum">{groups[id].length}</span>
            )}
          </button>
        ))}
      </div>

      {error && <div className="card error-card">{error}</div>}

      {!error && !data && <SkeletonTasks count={3} />}

      <div className="list">
        {shown.map((task) => (
          <TaskCard
            key={task.id}
            task={task}
            busy={busy === task.id}
            onToggle={toggle}
            onOpen={onOpen}
          />
        ))}

        {!error && data && shown.length === 0 && (
          <div className="card empty">
            <div className="empty-icon" aria-hidden="true">
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 4v11" /><path d="M7.5 10.5L12 15l4.5-4.5" /><path d="M4 19h16" />
              </svg>
            </div>
            <div className="empty-title">{t('downloads.emptyTitle')}</div>
            <div className="empty-text">
              {shownFilter === 'done'
                ? t('downloads.emptyDone')
                : t('downloads.emptyOther')}
            </div>
          </div>
        )}
      </div>

      <button type="button" className="fab" onClick={onAdd} aria-label={t('downloads.addAria')}>
        <svg width="25" height="25" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2.6" strokeLinecap="round" aria-hidden="true">
          <path d="M12 5v14" /><path d="M5 12h14" />
        </svg>
      </button>
    </div>
  )
}

function describe(e: unknown): string {
  if (e instanceof ApiError) return errorText(e, t('error.noService'))
  return t('error.noService')
}
