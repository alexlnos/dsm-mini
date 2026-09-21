import { useCallback, useEffect, useState } from 'react'
import { ScreenTitle } from '../components/ScreenTitle'
import { api, ApiError } from '../api'
import type { Guest, VMHost } from '../types'
import { alertMessage, backButton, confirmAction, haptic } from '../telegram'

interface Props {
  onBack: () => void
}

function ram(mb: number): string {
  if (mb >= 1024) {
    const gb = mb / 1024
    return `${gb % 1 === 0 ? gb : gb.toFixed(1).replace('.', ',')} ГБ`
  }
  return `${mb} МБ`
}

export function VMs({ onBack }: Props) {
  const [guests, setGuests] = useState<Guest[]>([])
  const [host, setHost] = useState<VMHost | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  useEffect(() => backButton(onBack), [onBack])

  const load = useCallback(async () => {
    try {
      const data = await api.vms()
      setGuests(data.guests ?? [])
      setHost(data.host)
      setError(null)
    } catch (e) {
      setError(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получить список машин')
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
        `Выключить «${guest.name}»? Гостевой системе будет отправлена команда завершения работы.`,
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
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получилось')
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="page">
      <div className="card summary">
        <ScreenTitle title="Машины" onBack={onBack}>
          {host && <span className="badge">{host.running_vms} из {host.total_vms}</span>}
        </ScreenTitle>
        {host && (
          <div className="tiles">
            <div className="tile">
              <span className="tile-label">Свободно RAM</span>
              <span className="tile-value tnum">{ram(host.free_ram_mb)}</span>
            </div>
            <div className="tile">
              <span className="tile-label">Занято vCPU</span>
              <span className="tile-value tnum">{host.used_vcpu}</span>
            </div>
          </div>
        )}
      </div>

      {error && <div className="card error-card">{error}</div>}

      <div className="list">
        {guests.map((g) => (
          <div key={g.id} className="card vm">
            <div className="vm-head">
              <span className={g.running ? 'dot on' : 'dot'} aria-hidden="true" />
              <div className="vm-text">
                <span className="vm-name">{g.name}</span>
                <span className="vm-specs tnum">
                  {g.vcpu} vCPU · {ram(g.ram_mb)} · диск {ram(g.disk_mb)}
                </span>
              </div>
              <button
                type="button"
                className={g.running ? 'icon-button danger' : 'icon-button ok'}
                disabled={busy === g.id}
                onClick={() => void act(g)}
                aria-label={g.running ? 'Выключить' : 'Запустить'}
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
                {g.running ? 'Работает' : 'Выключена'}
              </span>
              {g.autorun && <span className="muted tnum">автозапуск</span>}
            </div>
          </div>
        ))}

        {!error && guests.length === 0 && (
          <div className="card empty-text">Машин нет.</div>
        )}
      </div>
    </div>
  )
}
