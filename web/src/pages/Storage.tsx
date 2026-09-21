import { useCallback, useEffect, useState } from 'react'
import { ScreenTitle } from '../components/ScreenTitle'
import { Bar, SkeletonDisks } from '../components/Skeleton'
import { api, errorText } from '../api'
import { readCache, writeCache } from '../cache'
import { size } from '../format'
import { t } from '../i18n'
import { backButton } from '../telegram'
import type { Disk, StorageOverview } from '../types'

interface Props {
  onBack: () => void
}

/** The pool number out of a DSM identifier: reuse_1 → 1. */
function poolNumber(id: string): string {
  return id.replace('reuse_', '')
}

/** Disk purpose: the server sends a code, we build the caption in our language. */
function diskRole(disk: Disk): string {
  switch (disk.role) {
    case 'pool': return t('storage.rolePool', { pool: poolNumber(disk.pool ?? '') })
    case 'cache': return t('storage.roleCache')
    case 'free': return t('storage.roleFree')
    default: return disk.role ?? ''
  }
}

/** Above 50 °C deserves attention, above 55 — alarm. */
function tempClass(temp: number): string {
  if (temp >= 55) return 'temp hot'
  if (temp >= 50) return 'temp warm'
  return 'temp'
}

export function Storage({ onBack }: Props) {
  // The same key as on the home screen: if it was opened, the storage state
  // is already stored and draws immediately.
  const [data, setData] = useState<StorageOverview | null>(() => readCache('storage'))
  const [error, setError] = useState<string | null>(null)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async () => {
    try {
      const fresh = await api.storage()
      setData(fresh)
      writeCache('storage', fresh)
      setError(null)
    } catch (e) {
      setError(errorText(e, t('storage.failed')))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const disks = data?.disks ?? []
  const sata = disks.filter((d) => !d.id.startsWith('nvme'))
  const nvme = disks.filter((d) => d.id.startsWith('nvme'))
  const volumes = data?.volumes ?? []
  const pools = data?.pools ?? []

  // There are usually more bays than occupied ones: empty ones are dashed, as in DSM.
  const bayCount = Math.max(sata.length, 8)
  const bays: Array<Disk | null> = Array.from({ length: bayCount }, (_, i) =>
    sata.find((d) => d.slot === i + 1) ?? sata[i] ?? null,
  )

  return (
    <div className="page">
      <div className="card summary">
        <ScreenTitle title={t('storage.title')} onBack={onBack}>
          {data && (
            <span className={data.healthy ? 'badge ok' : 'badge bad'}>
              {data.healthy ? t('storage.healthy') : t('storage.attention')}
            </span>
          )}
        </ScreenTitle>
        {data ? (
          <div className="muted tnum">
            {t('storage.disksCount', { count: disks.length })}
            {' · '}{t('storage.poolsCount', { count: pools.length })}
            {' · '}{t('storage.volumesCount', { count: volumes.length })}
          </div>
        ) : (
          <Bar width="52%" height={13} />
        )}

        <div className="bays">
          <div className="bays-label">{t('storage.baysSata')}</div>
          <div className="bays-grid">
            {bays.map((disk, i) => (
              <div key={i} className={disk ? 'bay' : data ? 'bay empty' : 'bay sk'}>
                {(disk || data) && <span className="bay-num">{i + 1}</span>}
                {disk && <span className="bay-temp tnum">{disk.temp}°</span>}
              </div>
            ))}
          </div>

          {nvme.length > 0 && (
            <>
              <div className="bays-label">{t('storage.slotsM2')}</div>
              <div className="bays-row">
                {nvme.map((disk, i) => (
                  <div key={disk.id} className="bay m2">
                    <span className="bay-num">{i + 1}</span>
                    <span className="bay-temp tnum">{disk.temp}°</span>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>

      {error && <div className="card error-card">{error}</div>}

      {volumes.length > 0 && (
        <>
          <div className="section-head"><span className="section-title">{t('storage.volumes')}</span></div>
          <div className="list">
            {volumes.map((v) => {
              const percent = v.total > 0 ? Math.round((v.used / v.total) * 100) : 0
              return (
                <div key={v.id} className="card volume">
                  <div className="volume-head">
                    <span className="volume-name">
                      {v.name || t('home.volume', { number: v.id.replace('volume_', '') })}
                    </span>
                    <span className="muted tnum">
                      {t('common.outOf', { value: size(v.used), total: size(v.total) })}
                    </span>
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
                  <div className="volume-foot">
                    <span className="muted">{v.fs_type?.toUpperCase()}</span>
                    <span className="vm-status on">
                      {v.status === 'normal' ? t('storage.volumeOk') : v.status}
                    </span>
                  </div>
                </div>
              )
            })}
          </div>
        </>
      )}

      {pools.length > 0 && (
        <>
          <div className="section-head"><span className="section-title">{t('storage.pools')}</span></div>
          <div className="card">
            {pools.map((p, i) => (
              <div key={p.id}>
                {i > 0 && <div className="divider" />}
                <div className="pool">
                  <div className="volume-head">
                    <span className="volume-name">
                      {t('storage.pool', { number: poolNumber(p.id) })}
                    </span>
                    <span className="muted tnum">{size(p.total)}</span>
                  </div>
                  <div className="muted tnum pool-disks">
                    {p.disks.join(', ')}
                    {p.raid ? ` · ${p.raid}` : ''}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      <div className="section-head"><span className="section-title">{t('storage.disks')}</span></div>
      {!data && !error && <SkeletonDisks count={4} />}
      {data && (
      <div className="card">
        {disks.map((d, i) => (
          <div key={d.id}>
            {i > 0 && <div className="divider" />}
            <div className="disk">
              <span className={d.healthy ? 'dot on' : 'dot bad'} aria-hidden="true" />
              <div className="disk-text">
                <span className="disk-name">{d.id} · {d.model}</span>
                <span className="disk-meta tnum">{size(d.size)} · {diskRole(d)}</span>
              </div>
              <span className={tempClass(d.temp)}>{d.temp} °C</span>
            </div>
          </div>
        ))}
      </div>
      )}
    </div>
  )
}
