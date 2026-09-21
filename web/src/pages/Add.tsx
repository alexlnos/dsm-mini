import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { alertMessage, backButton, haptic } from '../telegram'
import type { Overview } from '../types'

interface Props {
  data: Overview | null
  onDone: () => void
  onBack: () => void
}

export function Add({ data, onDone, onBack }: Props) {
  const [link, setLink] = useState('')
  const [destination, setDestination] = useState(data?.default_destination ?? '')
  const [sending, setSending] = useState(false)

  useEffect(() => backButton(onBack), [onBack])

  // Папки: та, что по умолчанию, плюс использованные недавно.
  const folders = Array.from(
    new Set(
      [data?.default_destination, ...(data?.tasks ?? []).map((t) => t.destination)]
        .filter((f): f is string => Boolean(f)),
    ),
  ).slice(0, 6)

  async function submit() {
    const urls = link.split('\n').map((s) => s.trim()).filter(Boolean)
    if (urls.length === 0) {
      alertMessage('Вставьте ссылку')
      return
    }
    setSending(true)
    try {
      await api.create(urls, destination)
      haptic('success')
      setLink('')
      onDone()
    } catch (e) {
      haptic('error')
      alertMessage(e instanceof ApiError ? (e.detail ?? e.message) : 'Не получилось поставить задачу')
    } finally {
      setSending(false)
    }
  }

  async function paste() {
    try {
      const text = await navigator.clipboard.readText()
      if (text) setLink(text.trim())
    } catch {
      alertMessage('Буфер обмена недоступен — вставьте ссылку вручную')
    }
  }

  return (
    <div className="page">
      <div className="card">
        <h1 className="card-title">Новая загрузка</h1>

        <label className="field-label" htmlFor="link">Ссылка</label>
        <textarea
          id="link"
          rows={3}
          className="input"
          placeholder="magnet:?xt=urn:btih:… или https://…"
          value={link}
          onChange={(e) => setLink(e.target.value)}
        />
        <div className="row">
          <button type="button" className="button secondary" onClick={paste}>
            Из буфера
          </button>
        </div>
      </div>

      <div className="section-head">
        <span className="section-title">Куда положить</span>
      </div>

      <div className="list">
        {folders.map((folder) => (
          <button
            key={folder}
            type="button"
            className={folder === destination ? 'card folder selected' : 'card folder'}
            onClick={() => setDestination(folder)}
          >
            <span className="radio" aria-hidden="true">
              {folder === destination && <span className="radio-dot" />}
            </span>
            <span className="folder-name">{folder}</span>
          </button>
        ))}
        {folders.length === 0 && (
          <div className="card empty-text">
            Папки появятся, когда Download Station сообщит папку по умолчанию.
          </div>
        )}
      </div>

      <button
        type="button"
        className="main-button"
        disabled={sending || !link.trim()}
        onClick={submit}
      >
        {sending ? 'Ставлю…' : 'Поставить в очередь'}
      </button>
    </div>
  )
}
