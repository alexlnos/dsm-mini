import { useEffect } from 'react'
import { ProgressRing } from '../components/ProgressRing'
import { appearance } from '../components/TaskCard'
import { api, ApiError } from '../api'
import { eta, size, speed, statusLabel, taskTitle } from '../format'
import { alertMessage, backButton, confirmAction, haptic } from '../telegram'
import type { Task } from '../types'

interface Props {
  task: Task
  onBack: () => void
  onChanged: () => void
}

export function TaskDetail({ task, onBack, onChanged }: Props) {
  useEffect(() => backButton(onBack), [onBack])

  const look = appearance(task.status)
  const paused = task.status === 'paused' || task.status === 'error'

  async function act(action: 'pause' | 'resume') {
    try {
      await api.action(action, [task.id])
      haptic('light')
      onChanged()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получилось')
    }
  }

  async function remove() {
    if (!(await confirmAction('Удалить задачу? Это необратимо.'))) return
    try {
      await api.action('delete', [task.id])
      haptic('success')
      onBack()
      onChanged()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не удалить')
    }
  }

  const tiles: Array<[string, string]> = [
    ['Скорость', speed(task.speed_down)],
    ['Сиды / пиры', `${task.seeders} / ${task.leechers}`],
    ['Роздано', size(task.uploaded)],
    ['Тип', task.type.toUpperCase()],
  ]

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
          {size(task.downloaded)} из {size(task.size)} · {task.destination || '—'}
        </div>

        <div className="row">
          <button type="button" className="button secondary" onClick={() => void act(paused ? 'resume' : 'pause')}>
            {paused ? 'Возобновить' : 'Пауза'}
          </button>
          <button type="button" className="button danger" onClick={() => void remove()}>
            Удалить
          </button>
        </div>
      </div>

      <div className="tiles grid">
        {tiles.map(([label, value]) => (
          <div key={label} className="card tile-card">
            <span className="tile-label">{label}</span>
            <span className="tile-value tnum">{value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
