import { useCallback, useEffect, useRef, useState } from 'react'
import { Preview } from '../components/Preview'
import { Bar, SkeletonEntries } from '../components/Skeleton'
import { api, ApiError } from '../api'
import { size } from '../format'
import { alertMessage, backButton, confirmAction, haptic } from '../telegram'
import type { Entry } from '../types'

const ICONS: Record<string, { glyph: string; className: string }> = {
  dir: { glyph: '▸', className: 'icon dir' },
  audio: { glyph: '♪', className: 'icon audio' },
  image: { glyph: '▣', className: 'icon image' },
  video: { glyph: '▶', className: 'icon video' },
  doc: { glyph: '≡', className: 'icon doc' },
}

/** Родительская папка; у корня родитель — он сам. */
function parentOf(path: string): string {
  const parent = path.replace(/\/+$/, '').split('/').slice(0, -1).join('/')
  return parent || '/'
}

function kindOf(entry: Entry): keyof typeof ICONS {
  if (entry.is_dir) return 'dir'
  const ext = entry.name.split('.').pop()?.toLowerCase() ?? ''
  if (['mp3', 'flac', 'wav', 'm4a', 'ogg'].includes(ext)) return 'audio'
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'heic'].includes(ext)) return 'image'
  if (['mp4', 'mkv', 'avi', 'mov', 'webm'].includes(ext)) return 'video'
  return 'doc'
}

interface FilesProps {
  /** Режим выбора папки: вместо обычного обзора показываем кнопку подтверждения. */
  pickMode?: boolean
  onPick?: (path: string) => void
  onCancelPick?: () => void
  /** С какой папки начать обзор. Пусто — с корня. */
  initialPath?: string
  /**
   * Что перенести или скопировать сюда. Задаётся, когда обзор открыт ради
   * выбора папки назначения для уже отмеченных файлов.
   */
  pending?: { paths: string[]; move: boolean } | null
  /** Отмеченные файлы отправлены в другую папку: открыть выбор назначения. */
  onTransfer?: (paths: string[], move: boolean) => void
  /**
   * Куда пользователь перешёл.
   *
   * Вкладка «Файлы» размонтируется при переходе на другую вкладку, и без
   * этого возврат всегда приводил бы в корень, а не туда, где человек был.
   */
  onPathChange?: (path: string) => void
}

export function Files({
  pickMode, onPick, onCancelPick, initialPath, pending, onTransfer, onPathChange,
}: FilesProps = {}) {
  const [path, setPath] = useState('/')
  const [entries, setEntries] = useState<Entry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [preview, setPreview] = useState<Entry | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const fileInput = useRef<HTMLInputElement>(null)

  // Начальная папка берётся один раз, при открытии экрана: дальше человек
  // ходит сам, и возврат к ней сбивал бы его с пути. Обработчик держим в
  // ссылке, чтобы его смена не перезапускала обзор с начала.
  const start = useRef(initialPath)
  const report = useRef(onPathChange)
  report.current = onPathChange

  // stay = обновление той же папки после действия: список не сбрасываем,
  // иначе он мигнул бы заглушками на каждое переименование.
  const load = useCallback(async (target: string, stay = false) => {
    setLoading(true)
    setError(null)
    if (!stay) setEntries([])
    try {
      const { entries } = await api.files(target)
      setEntries(entries ?? [])
      setPath(target)
      setSelected(new Set())
      report.current?.(target)
    } catch (e) {
      setError(e instanceof ApiError ? (e.detail ?? e.message) : 'Не прочитать папку')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const from = start.current
    void load(from ? '/' + from.replace(/^\/+/, '') : '/')
  }, [load])

  useEffect(() => {
    if (!pickMode || !onCancelPick) return
    return backButton(onCancelPick)
  }, [pickMode, onCancelPick])

  const segments = path.split('/').filter(Boolean)
  // На верхнем уровне перечислены общие папки NAS: переименовать или удалить
  // их через File Station нельзя, поэтому и кнопок быть не должно.
  const canEdit = !pickMode && path !== '/'
  const chosen = entries.filter((e) => selected.has(e.path))

  function toggle(entry: Entry) {
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(entry.path)) next.delete(entry.path)
      else next.add(entry.path)
      return next
    })
    haptic('light')
  }

  async function act(what: string, run: () => Promise<unknown>) {
    setBusy(what)
    try {
      await run()
      haptic('success')
      await load(path, true)
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получилось')
    } finally {
      setBusy(null)
    }
  }

  async function removeSelected() {
    const names = chosen.map((e) => e.name).join(', ')
    if (!(await confirmAction(`Удалить безвозвратно: ${names}?`))) return
    await act('delete', () => api.deleteFiles(chosen.map((e) => e.path)))
  }

  async function renameEntry(entry: Entry) {
    const name = window.prompt('Новое имя', entry.name)
    if (!name || name === entry.name) return
    await act('rename', () => api.rename(entry.path, name))
  }

  async function createFolder() {
    const name = window.prompt('Имя новой папки')
    if (!name) return
    await act('mkdir', () => api.createFolder(path, name))
  }

  async function upload(file: File) {
    if (path === '/') {
      alertMessage('Выберите папку: в корень загружать нельзя')
      return
    }
    await act('upload', () => api.upload(path, file, false))
  }

  // Копирование и перенос идут на NAS фоновой задачей — дожидаемся её.
  async function finishTransfer(taskId: string) {
    for (let i = 0; i < 120; i++) {
      const status = await api.transferStatus(taskId)
      if (status.finished) {
        if (status.skipped) {
          alertMessage('Часть файлов пропущена: в папке назначения уже есть файлы с такими именами.')
        }
        return
      }
      await new Promise((r) => setTimeout(r, 1000))
    }
    alertMessage('Операция ещё идёт на NAS — обновите папку позже.')
  }

  async function applyPending() {
    if (!pending || path === '/') return
    const verb = pending.move ? 'move' : 'copy'
    await act(verb, async () => {
      const { task_id } = pending.move
        ? await api.move(pending.paths, path, false)
        : await api.copy(pending.paths, path, false)
      await finishTransfer(task_id)
    })
    onPick?.(path.replace(/^\/+/, ''))
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
        <div className="home-head">
          {/*
            Стрелка ведёт в родительскую папку, а не из раздела: системная
            кнопка Telegram этого не умеет, поэтому показываем свою везде,
            где есть куда подняться.
          */}
          {path !== '/' && (
            <button
              type="button"
              className="back-button"
              onClick={() => void load(parentOf(path))}
              aria-label="На уровень выше"
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                   strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M15 18l-6-6 6-6" />
              </svg>
            </button>
          )}
          <h1>{segments.at(-1) ?? 'Общие папки'}</h1>
        </div>
        {loading && entries.length === 0 ? (
          <Bar width="38%" height={13} />
        ) : (
          <div className="muted tnum">{entries.length} объектов</div>
        )}

        {pending && (
          <div className="row">
            <button
              type="button"
              className="button"
              disabled={path === '/' || busy !== null}
              onClick={() => void applyPending()}
            >
              {busy ? 'Переношу…' : (
                path === '/' ? 'Откройте папку'
                  : `${pending.move ? 'Перенести' : 'Скопировать'} сюда · ${pending.paths.length}`
              )}
            </button>
          </div>
        )}

        {pickMode && !pending && (
          <div className="row">
            <button
              type="button"
              className="button"
              disabled={path === '/'}
              onClick={() => onPick?.(path.replace(/^\/+/, ''))}
            >
              {path === '/' ? 'Откройте папку' : `Выбрать ${segments.at(-1)}`}
            </button>
          </div>
        )}

        {!pickMode && path !== '/' && (
          <div className="row">
            <button type="button" className="button secondary"
                    onClick={() => fileInput.current?.click()}>
              Загрузить
            </button>
            <button type="button" className="button secondary" onClick={() => void createFolder()}>
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
      {loading && entries.length === 0 && <SkeletonEntries count={6} />}

      <div className="list">
        {entries.map((entry) => {
          const icon = ICONS[kindOf(entry)]
          const checked = selected.has(entry.path)
          return (
            <div key={entry.path} className={checked ? 'card entry checked' : 'card entry'}>
              {canEdit && (
                <label className="entry-check">
                  <input type="checkbox" checked={checked} onChange={() => toggle(entry)} />
                </label>
              )}
              <button
                type="button"
                className="entry-main"
                onClick={() => (entry.is_dir ? void load(entry.path) : setPreview(entry))}
              >
                <span className={icon.className} aria-hidden="true">{icon.glyph}</span>
                <span className="entry-text">
                  <span className="entry-name">{entry.name}</span>
                  <span className="entry-meta tnum">
                    {entry.is_dir ? 'папка' : size(entry.size)}
                  </span>
                </span>
              </button>
              {canEdit && (
                <button type="button" className="icon-button small" onClick={() => void renameEntry(entry)}
                        aria-label="Переименовать">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                       strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z" />
                  </svg>
                </button>
              )}
            </div>
          )
        })}
      </div>

      {preview && <Preview entry={preview} onClose={() => setPreview(null)} />}

      {chosen.length > 0 && (
        <div className="action-bar">
          <span className="action-count tnum">{chosen.length}</span>
          <button type="button" className="action" disabled={busy !== null}
                  onClick={() => onTransfer?.(chosen.map((e) => e.path), false)}>
            Копировать
          </button>
          <button type="button" className="action" disabled={busy !== null}
                  onClick={() => onTransfer?.(chosen.map((e) => e.path), true)}>
            Перенести
          </button>
          <button type="button" className="action danger" disabled={busy !== null}
                  onClick={() => void removeSelected()}>
            Удалить
          </button>
        </div>
      )}
    </div>
  )
}
