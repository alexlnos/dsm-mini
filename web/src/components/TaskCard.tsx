import { eta, size, speed, statusLabel, taskTitle } from '../format'
import { t } from '../i18n'
import type { Task } from '../types'
import { ProgressRing } from './ProgressRing'

interface Props {
  task: Task
  busy: boolean
  onToggle: (task: Task) => void
  onOpen: (task: Task) => void
}

/** Colour and glyph by task state. */
export function appearance(status: string) {
  switch (status) {
    case 'finished':
    case 'seeding':
      return { color: 'var(--ok)', glyph: '✓' }
    case 'error':
      return { color: 'var(--bad)', glyph: '!' }
    case 'paused':
      return { color: 'var(--muted-strong)', glyph: '❙❙' }
    default:
      return { color: 'var(--accent)', glyph: '↓' }
  }
}

export function TaskCard({ task, busy, onToggle, onOpen }: Props) {
  const look = appearance(task.status)
  const paused = task.status === 'paused' || task.status === 'error'
  const remaining = eta(task.eta_seconds)
  // A finished task is neither paused nor resumed — the button would only
  // confuse. A failed one can be restarted.
  const canToggle = task.status !== 'finished'

  return (
    <div className="card task">
      <div className="task-head">
        <button
          type="button"
          className="task-open"
          onClick={() => onOpen(task)}
          aria-label={t('downloads.detailsAria')}
        >
          <ProgressRing value={task.progress} color={look.color} glyph={look.glyph} />
        </button>

        <button type="button" className="task-text" onClick={() => onOpen(task)}>
          <span className="task-title">{taskTitle(task.title)}</span>
          <span className="task-meta">
            {task.destination || t('common.dash')} ·{' '}
            {t('common.outOf', { value: size(task.downloaded), total: size(task.size) })}
          </span>
        </button>

        {canToggle && (
        <button
          type="button"
          className="icon-button"
          disabled={busy}
          onClick={() => onToggle(task)}
          aria-label={paused ? t('downloads.resume') : t('downloads.pause')}
        >
          {paused ? (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M8 5.2v13.6a.8.8 0 0 0 1.23.67l10.4-6.8a.8.8 0 0 0 0-1.34L9.23 4.53A.8.8 0 0 0 8 5.2z" />
            </svg>
          ) : (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <rect x="6.5" y="5" width="4.2" height="14" rx="1.4" />
              <rect x="13.3" y="5" width="4.2" height="14" rx="1.4" />
            </svg>
          )}
        </button>
        )}
      </div>

      <div className="task-foot">
        <span style={{ color: look.color, fontWeight: 600 }}>
          {statusLabel(task.status)}
          {task.active && task.progress > 0 ? ` · ${Math.round(task.progress * 100)}%` : ''}
        </span>
        <span className="muted tnum">
          {task.speed_down > 0 ? speed(task.speed_down) : ''}
          {remaining ? ` · ${remaining}` : ''}
        </span>
      </div>
    </div>
  )
}
