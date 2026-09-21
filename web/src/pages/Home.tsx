import { useCallback, useEffect, useState } from 'react'
import { SkeletonMeters } from '../components/Skeleton'
import { api, errorText } from '../api'
import { readCache, writeCache } from '../cache'
import { size, speed, uptime } from '../format'
import { t } from '../i18n'
import type { Overview, StorageOverview, SystemOverview } from '../types'

export type Section = 'downloads' | 'files' | 'vms' | 'containers' | 'storage' | 'notifications'

interface Props {
  downloads: Overview | null
  onOpen: (section: Section) => void
}

/** Шкала загрузки: выше 85% подсвечивается, чтобы это бросалось в глаза. */
function Meter({ label, percent }: { label: string; percent: number }) {
  const color = percent >= 90 ? 'var(--bad)' : percent >= 75 ? '#B45309' : 'var(--ok)'
  return (
    <div className="meter">
      <span className="meter-label">{label}</span>
      <div className="meter-track">
        <div className="meter-fill" style={{ width: `${Math.min(percent, 100)}%`, background: color }} />
      </div>
      <span className="meter-value tnum" style={{ color }}>{percent}%</span>
    </div>
  )
}

export function Home({ downloads, onOpen }: Props) {
  // Начинаем с сохранённых значений: экран заполнен с первого кадра, а
  // свежие данные подменяют их, когда придут.
  const [system, setSystem] = useState<SystemOverview | null>(() => readCache('system'))
  const [storage, setStorage] = useState<StorageOverview | null>(() => readCache('storage'))
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const [sys, st] = await Promise.all([api.system(), api.storage().catch(() => null)])
      setSystem(sys)
      writeCache('system', sys)
      if (st) {
        setStorage(st)
        writeCache('storage', st)
      }
      setError(null)
    } catch (e) {
      setError(errorText(e, t('home.offline')))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  // Показатели меняются постоянно, но чаще раза в пять секунд смотреть незачем.
  useEffect(() => {
    const timer = window.setInterval(() => { void load() }, 5000)
    return () => window.clearInterval(timer)
  }, [load])

  const info = system?.info
  const usage = system?.usage
  const packages = system?.packages ?? {}
  const volumes = storage?.volumes ?? []

  const activeTasks = downloads?.tasks.filter((t) => t.active).length ?? 0
  const doneTasks = (downloads?.tasks.length ?? 0) - activeTasks

  const apps: Array<{
    id: Section; name: string; note: string; kind: string; pkg?: string
  }> = [
    {
      id: 'downloads', name: t('app.downloads'), kind: 'down', pkg: 'DownloadStation',
      note: activeTasks > 0
        ? t('home.activeNote', { active: activeTasks, done: doneTasks })
        : t('home.tasksNote', { count: doneTasks }),
    },
    { id: 'files', name: t('app.files'), kind: 'files', pkg: 'FileStation', note: t('home.filesNote') },
    { id: 'vms', name: t('app.vms'), kind: 'vm', pkg: 'Virtualization', note: t('home.vmsNote') },
    {
      id: 'containers', name: t('app.containers'), kind: 'docker',
      pkg: 'ContainerManager', note: 'Container Manager',
    },
    {
      id: 'storage', name: t('app.storage'), kind: 'disk',
      note: storage
        ? t('home.storageNote', {
          disks: t('storage.disksCount', { count: storage.disks?.length ?? 0 }),
          pools: t('storage.poolsCount', { count: storage.pools?.length ?? 0 }),
        })
        : t('home.storageFallbackNote'),
    },
    { id: 'notifications', name: t('app.notifications'), kind: 'bell', note: t('home.logNote') },
  ]

  return (
    <div className="page">
      <div className="card summary">
        <div className="home-head">
          <div className="home-icon" aria-hidden="true">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="4" width="18" height="7" rx="1.8" />
              <rect x="3" y="13" width="18" height="7" rx="1.8" />
              <path d="M7 7.5h.01" /><path d="M7 16.5h.01" />
            </svg>
          </div>
          <div className="home-title">
            <div className="home-name">{info?.hostname || 'NAS'}</div>
            <div className="muted tnum home-sub">
              {info ? `${info.model} · DSM ${info.firmware?.split('-')[0] ?? ''}` : t('home.connecting')}
            </div>
          </div>
          <span className={error ? 'badge bad' : 'badge ok'}>{error ? t('home.noLink') : t('home.online')}</span>
        </div>

        {error && <div className="home-error">{error}</div>}

        {!usage && !error && <SkeletonMeters />}

        {usage && (
          <>
            <Meter label="CPU" percent={usage.cpu_percent} />
            <Meter label="RAM" percent={usage.memory_percent} />
            <div className="home-net">
              <span className="net-in tnum">↓ {speed(usage.network_rx)}</span>
              <span className="muted tnum">↑ {speed(usage.network_tx)}</span>
              {info && <span className="muted tnum home-uptime">{uptime(info.uptime_seconds)}</span>}
            </div>
          </>
        )}
      </div>

      {volumes.length > 0 && (
        <div className="card widget">
          <div className="widget-head">
            <span className="widget-title">{t('home.storage')}</span>
            <span className={storage?.healthy ? 'widget-state ok' : 'widget-state bad'}>
              {storage?.healthy ? t('home.healthy') : t('home.needsAttention')}
            </span>
          </div>
          {volumes.map((v) => {
            const percent = v.total > 0 ? Math.round((v.used / v.total) * 100) : 0
            return (
              <div key={v.id} className="volume-row">
                <div className="volume-head">
                  <span className="volume-name">
                    {v.name || t('home.volume', { number: v.id.replace('volume_', '') })}
                  </span>
                  <span className="muted tnum">{t('home.free', { size: size(v.total - v.used) })}</span>
                </div>
                <div className="bar">
                  <div
                    className="bar-fill"
                    style={{
                      width: `${percent}%`,
                      background: percent >= 90 ? 'var(--bad)' : percent >= 75 ? '#B45309' : 'var(--accent)',
                    }}
                  />
                </div>
              </div>
            )
          })}
        </div>
      )}

      <div className="section-head">
        <span className="section-title">{t('home.apps')}</span>
      </div>

      <div className="list">
        {apps.map((app) => {
          const state = app.pkg ? packages[app.pkg] : undefined
          // Раздел без своего пакета (хранилище, журнал) доступен всегда.
          const available = !state || (state.installed && state.running)
          return (
            <button
              key={app.id}
              type="button"
              className={available ? 'card app-card' : 'card app-card off'}
              disabled={!available}
              onClick={() => onOpen(app.id)}
            >
              <span className={`icon ${app.kind}`} aria-hidden="true">{glyph(app.kind)}</span>
              <span className="app-text">
                <span className="app-name">{app.name}</span>
                <span className="app-note tnum">{app.note}</span>
              </span>
              {available ? (
                <svg width="9" height="15" viewBox="0 0 9 15" fill="none" stroke="#C7C7CC"
                     strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M1.5 1.5L7 7.5l-5.5 6" />
                </svg>
              ) : (
                <span className="app-off">
                  {state?.installed ? t('home.notRunning') : t('home.notInstalled')}
                </span>
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}

function glyph(kind: string): string {
  switch (kind) {
    case 'down': return '↓'
    case 'files': return '▤'
    case 'vm': return '▢'
    case 'docker': return '◲'
    case 'disk': return '◉'
    default: return '◔'
  }
}
