import { useCallback, useEffect, useRef, useState } from 'react'
import { api, ApiError } from '../api'
import { size } from '../format'
import { alertMessage, confirmAction, haptic } from '../telegram'
import type { Entry } from '../types'

const ICONS: Record<string, { glyph: string; className: string }> = {
  dir: { glyph: '▸', className: 'icon dir' },
  audio: { glyph: '♪', className: 'icon audio' },
  image: { glyph: '▣', className: 'icon image' },
  video: { glyph: '▶', className: 'icon video' },
  doc: { glyph: '≡', className: 'icon doc' },
}

function kindOf(entry: Entry): keyof typeof ICONS {
  if (entry.is_dir) return 'dir'
  const ext = entry.name.split('.').pop()?.toLowerCase() ?? ''
  if (['mp3', 'flac', 'wav', 'm4a', 'ogg'].includes(ext)) return 'audio'
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'heic'].includes(ext)) return 'image'
  if (['mp4', 'mkv', 'avi', 'mov', 'webm'].includes(ext)) return 'video'
  return 'doc'
}

export function Files() {
  const [path, setPath] = useState('/')
  const [entries, setEntries] = useState<Entry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const fileInput = useRef<HTMLInputElement>(null)

  const load = useCallback(async (target: string) => {
    setLoading(true)
    setError(null)
    try {
      const { entries } = await api.files(target)
      setEntries(entries ?? [])
      setPath(target)
    } catch (e) {
      setError(e instanceof ApiError ? (e.detail ?? e.message) : 'Не прочитать папку')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load('/') }, [load])

  const segments = path.split('/').filter(Boolean)

  async function remove(entry: Entry) {
    const what = entry.is_dir ? 'папку' : 'файл'
    if (!(await confirmAction(`Удалить ${what} «${entry.name}»? Это необратимо.`))) return
    try {
      await api.deleteFiles([entry.path])
      haptic('success')
      await load(path)
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не удалить')
    }
  }

  async function rename(entry: Entry) {
    const name = window.prompt('Новое имя', entry.name)
    if (!name || name === entry.name) return
    try {
      await api.rename(entry.path, name)
      haptic('success')
      await load(path)
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не переименовать')
    }
  }

  async function upload(file: File) {
    if (path === '/') {
      alertMessage('Выберите папку: в корень загружать нельзя')
      return
    }
    try {
      await api.upload(path, file, false)
      haptic('success')
      await load(path)
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не загрузить файл')
    }
  }

  async function createFolder() {
    const name = window.prompt('Имя новой папки')
    if (!name) return
    try {
      await api.createFolder(path, name)
      haptic('success')
      await load(path)
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не создать папку')
    }
  }

  return (
    <div className="page">
      <div className="card summary">
        <div className="crumbs">
          <button type="button" className="crumb" onClick={() => void load('/')}>NAS</button>
          {segments.map((part, i) => {
            const target = '/' + segments.slice(0, i + 1).join('/')
            const last = i === segments.length - 1
            return (
              <span key={target} className="crumb-wrap">
                <span className="crumb-sep">›</span>
                {last ? (
                  <span className="crumb current">{part}</span>
                ) : (
                  <button type="button" className="crumb" onClick={() => void load(target)}>
                    {part}
                  </button>
                )}
              </span>
            )
          })}
        </div>
        <h1>{segments.at(-1) ?? 'Общие папки'}</h1>
        <div className="muted tnum">{entries.length} объектов</div>

        {path !== '/' && (
          <div className="row">
            <button type="button" className="button secondary"
                    onClick={() => fileInput.current?.click()}>
              Загрузить
            </button>
            <button type="button" className="button secondary" onClick={createFolder}>
              Новая папка
            </button>
          </div>
        )}
        <input
          ref={fileInput}
          type="file"
          hidden
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) void upload(file)
            e.target.value = ''
          }}
        />
      </div>

      {error && <div className="card error-card">{error}</div>}
      {loading && <div className="card empty-text">Загружаю…</div>}

      <div className="list">
        {entries.map((entry) => {
          const icon = ICONS[kindOf(entry)]
          return (
            <div key={entry.path} className="card entry">
              <button
                type="button"
                className="entry-main"
                onClick={() => entry.is_dir && void load(entry.path)}
              >
                <span className={icon.className} aria-hidden="true">{icon.glyph}</span>
                <span className="entry-text">
                  <span className="entry-name">{entry.name}</span>
                  <span className="entry-meta tnum">
                    {entry.is_dir ? 'папка' : size(entry.size)}
                  </span>
                </span>
              </button>
              <button type="button" className="icon-button small" onClick={() => void rename(entry)}
                      aria-label="Переименовать">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                     strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z" />
                </svg>
              </button>
              <button type="button" className="icon-button small danger" onClick={() => void remove(entry)}
                      aria-label="Удалить">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                     strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M4 7h16" /><path d="M6.5 7l.9 12.1A1.5 1.5 0 0 0 8.9 20.5h6.2a1.5 1.5 0 0 0 1.5-1.4L17.5 7" />
                </svg>
              </button>
            </div>
          )
        })}
      </div>
    </div>
  )
}
