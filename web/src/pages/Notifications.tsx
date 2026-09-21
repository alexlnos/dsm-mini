import { useCallback, useEffect, useMemo, useState } from 'react'
import { ScreenTitle } from '../components/ScreenTitle'
import { SkeletonEvents, SkeletonTiles } from '../components/Skeleton'
import { api, errorText } from '../api'
import { logDay, logTime } from '../format'
import { t } from '../i18n'
import { backButton } from '../telegram'
import type { LogEntry } from '../types'

interface Props {
  onBack: () => void
}

type Filter = 'all' | 'problems'

export function Notifications({ onBack }: Props) {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [filter, setFilter] = useState<Filter>('all')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async (mode: Filter) => {
    setLoading(true)
    try {
      const data = await api.systemLog(60, mode === 'problems')
      setEntries(data.entries ?? [])
      setError(null)
    } catch (e) {
      setError(errorText(e, t('log.failed')))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load(filter) }, [load, filter])

  // Записи приходят по времени, группируем их по дням для читаемости.
  const groups = useMemo(() => {
    const byDay = new Map<string, LogEntry[]>()
    for (const entry of entries) {
      const day = logDay(entry.time)
      const list = byDay.get(day)
      if (list) list.push(entry)
      else byDay.set(day, [entry])
    }
    return Array.from(byDay, ([day, items]) => ({ day, items }))
  }, [entries])

  const problems = entries.filter((e) => e.level === 'error' || e.level === 'warn').length

  return (
    <div className="page">
      <div className="card summary">
        <ScreenTitle title={t('log.title')} onBack={onBack}>
          <span className="muted tnum">{t('log.subtitle')}</span>
        </ScreenTitle>
        {loading && entries.length === 0 ? <SkeletonTiles /> : (
        <div className="tiles">
          <div className="tile">
            <span className="tile-label">{t('log.records')}</span>
            <span className="tile-value tnum">{entries.length}</span>
          </div>
          <div className="tile">
            <span className="tile-label">{t('log.problems')}</span>
            <span className="tile-value tnum" style={{ color: problems ? 'var(--bad)' : undefined }}>
              {problems}
            </span>
          </div>
        </div>
        )}
      </div>

      <div className="segments" role="tablist">
        {([['all', t('log.tabAll')], ['problems', t('log.tabProblems')]] as const).map(([id, label]) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={filter === id}
            className={filter === id ? 'segment active' : 'segment'}
            onClick={() => setFilter(id)}
          >
            {label}
          </button>
        ))}
      </div>

      {error && <div className="card error-card">{error}</div>}
      {loading && entries.length === 0 && <SkeletonEvents count={5} />}

      {!loading && !error && entries.length === 0 && (
        <div className="card empty-text">
          {filter === 'problems' ? t('log.emptyProblems') : t('log.empty')}
        </div>
      )}

      {groups.map((group) => (
        <div key={group.day}>
          <div className="section-head"><span className="section-title">{group.day}</span></div>
          <div className="card">
            {group.items.map((entry, i) => (
              <div key={`${entry.time}-${i}`}>
                {i > 0 && <div className="divider" />}
                <div className="event">
                  <span className={`dot ${levelClass(entry.level)}`} aria-hidden="true" />
                  <div className="event-text">
                    <span className="event-message">{entry.message}</span>
                    <span className="event-meta tnum">
                      {logTime(entry.time)} · {entry.type}
                      {entry.who ? ` · ${entry.who}` : ''}
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

function levelClass(level: string): string {
  switch (level) {
    case 'error': return 'bad'
    case 'warn': return 'warn'
    default: return 'info'
  }
}
