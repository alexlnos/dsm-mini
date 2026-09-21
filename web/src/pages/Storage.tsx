import { useCallback, useEffect, useState } from 'react'
import { ScreenTitle } from '../components/ScreenTitle'
import { Bar, SkeletonDisks } from '../components/Skeleton'
import { api, ApiError } from '../api'
import { readCache, writeCache } from '../cache'
import { size } from '../format'
import { backButton } from '../telegram'
import type { Disk, StorageOverview } from '../types'

interface Props {
  onBack: () => void
}

/** Температура выше 50 °C заслуживает внимания, выше 55 — тревоги. */
function tempClass(temp: number): string {
  if (temp >= 55) return 'temp hot'
  if (temp >= 50) return 'temp warm'
  return 'temp'
}

export function Storage({ onBack }: Props) {
  // Тот же ключ, что и на главном экране: если она открывалась, состояние
  // хранилища уже сохранено и рисуется сразу.
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
      setError(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получить состояние хранилища')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const disks = data?.disks ?? []
  const sata = disks.filter((d) => !d.id.startsWith('nvme'))
  const nvme = disks.filter((d) => d.id.startsWith('nvme'))
  const volumes = data?.volumes ?? []
  const pools = data?.pools ?? []

  // Отсеков обычно больше, чем занятых: пустые рисуем пунктиром, как в DSM.
  const bayCount = Math.max(sata.length, 8)
  const bays: Array<Disk | null> = Array.from({ length: bayCount }, (_, i) =>
    sata.find((d) => d.slot === i + 1) ?? sata[i] ?? null,
  )

  return (
    <div className="page">
      <div className="card summary">
        <ScreenTitle title="Хранилище" onBack={onBack}>
          {data && (
            <span className={data.healthy ? 'badge ok' : 'badge bad'}>
              {data.healthy ? 'исправно' : 'внимание'}
            </span>
          )}
        </ScreenTitle>
        {data ? (
          <div className="muted tnum">
            {disks.length} дисков · {pools.length} пула · {volumes.length} тома
          </div>
        ) : (
          <Bar width="52%" height={13} />
        )}

        <div className="bays">
          <div className="bays-label">Отсеки SATA</div>
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
              <div className="bays-label">Слоты M.2</div>
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
          <div className="section-head"><span className="section-title">Тома</span></div>
          <div className="list">
            {volumes.map((v) => {
              const percent = v.total > 0 ? Math.round((v.used / v.total) * 100) : 0
              return (
                <div key={v.id} className="card volume">
                  <div className="volume-head">
                    <span className="volume-name">{v.name || v.id.replace('volume_', 'Том ')}</span>
                    <span className="muted tnum">{size(v.used)} из {size(v.total)}</span>
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
                    <span className="vm-status on">{v.status === 'normal' ? 'исправен' : v.status}</span>
                  </div>
                </div>
              )
            })}
          </div>
        </>
      )}

      {pools.length > 0 && (
        <>
          <div className="section-head"><span className="section-title">Пулы</span></div>
          <div className="card">
            {pools.map((p, i) => (
              <div key={p.id}>
                {i > 0 && <div className="divider" />}
                <div className="pool">
                  <div className="volume-head">
                    <span className="volume-name">{p.id.replace('reuse_', 'Пул ')}</span>
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

      <div className="section-head"><span className="section-title">Диски</span></div>
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
                <span className="disk-meta tnum">{size(d.size)} · {d.role}</span>
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
