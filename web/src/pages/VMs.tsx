import { useCallback, useEffect, useState } from 'react'
import { ScreenTitle } from '../components/ScreenTitle'
import { SkeletonRows, SkeletonTiles } from '../components/Skeleton'
import { api, ApiError } from '../api'
import { readCache, writeCache } from '../cache'
import type { Guest, VMHost } from '../types'
import { alertMessage, backButton, confirmAction, haptic } from '../telegram'
import { formatNumber, t } from '../i18n'

interface Props {
  onBack: () => void
}

function ram(mb: number): string {
  if (mb >= 1024) {
    const gb = mb / 1024
    return `${formatNumber(gb, gb % 1 === 0 ? 0 : 1)} ${t('unit.gb')}`
  }
  return `${mb} ${t('unit.mb')}`
}

export function VMs({ onBack }: Props) {
  // Начинаем с сохранённого ответа: экран заполнен с первого кадра, а
  // свежий список подменяет его, когда придёт.
  const [guests, setGuests] = useState<Guest[]>(() => readCache<Guest[]>('vms') ?? [])
  const [host, setHost] = useState<VMHost | null>(() => readCache<VMHost>('vm-host'))
  const [loaded, setLoaded] = useState(() => readCache<Guest[]>('vms') !== null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async () => {
    try {
      const data = await api.vms()
      setGuests(data.guests ?? [])
      setHost(data.host)
      writeCache('vms', data.guests ?? [])
      writeCache('vm-host', data.host)
      setError(null)
    } catch (e) {
      setError(e instanceof ApiError ? (e.detail ?? e.message) : t('vms.listFailed'))
    } finally {
      setLoaded(true)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  // Пока машина запускается или гасится, состояние меняется — следим.
  useEffect(() => {
    const timer = window.setInterval(() => { void load() }, 6000)
    return () => window.clearInterval(timer)
  }, [load])

  async function act(guest: Guest) {
    const starting = !guest.running
    if (!starting) {
      const ok = await confirmAction(
        t('vms.confirmShutdown', { name: guest.name }),
      )
      if (!ok) return
    }
    setBusy(guest.id)
    try {
      await api.vmAction(guest.id, starting ? 'start' : 'shutdown')
      haptic('success')
      await load()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : t('common.failed'))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="page">
      <div className="card summary">
        <ScreenTitle title={t('vms.title')} onBack={onBack}>
          {host && (
            <span className="badge">
              {t('common.outOf', { value: host.running_vms, total: host.total_vms })}
            </span>
          )}
        </ScreenTitle>
        {!host && !loaded && <SkeletonTiles />}
        {host && (
          <div className="tiles">
            <div className="tile">
              <span className="tile-label">{t('vms.freeRam')}</span>
              <span className="tile-value tnum">{ram(host.free_ram_mb)}</span>
            </div>
            <div className="tile">
              <span className="tile-label">{t('vms.usedVcpu')}</span>
              <span className="tile-value tnum">{host.used_vcpu}</span>
            </div>
          </div>
        )}
      </div>

      {error && <div className="card error-card">{error}</div>}

      {!loaded && guests.length === 0 && <SkeletonRows count={3} dot action />}

      <div className="list">
        {guests.map((g) => (
          <div key={g.id} className="card vm">
            <div className="vm-head">
              <span className={g.running ? 'dot on' : 'dot'} aria-hidden="true" />
              <div className="vm-text">
                <span className="vm-name">{g.name}</span>
                <span className="vm-specs tnum">
                  {t('vms.specs', { vcpu: g.vcpu, ram: ram(g.ram_mb), disk: ram(g.disk_mb) })}
                </span>
              </div>
              <button
                type="button"
                className={g.running ? 'icon-button danger' : 'icon-button ok'}
                disabled={busy === g.id}
                onClick={() => void act(g)}
                aria-label={g.running ? t('vms.stop') : t('vms.start')}
              >
                {g.running ? (
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                    <rect x="6" y="6" width="12" height="12" rx="2" />
                  </svg>
                ) : (
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                    <path d="M8 5.2v13.6a.8.8 0 0 0 1.23.67l10.4-6.8a.8.8 0 0 0 0-1.34L9.23 4.53A.8.8 0 0 0 8 5.2z" />
                  </svg>
                )}
              </button>
            </div>
            <div className="vm-foot">
              <span className={g.running ? 'vm-status on' : 'vm-status'}>
                {g.running ? t('vms.running') : t('vms.stopped')}
              </span>
              {g.autorun && <span className="muted tnum">{t('vms.autorun')}</span>}
            </div>
          </div>
        ))}

        {loaded && !error && guests.length === 0 && (
          <div className="card empty-text">{t('vms.empty')}</div>
        )}
      </div>
    </div>
  )
}
