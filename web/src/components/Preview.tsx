import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { t } from '../i18n'
import { size as humanSize } from '../format'
import type { Entry } from '../types'

interface Props {
  entry: Entry
  onClose: () => void
}

const IMAGE_EXT = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'heic', 'tif', 'tiff', 'svg']
const TEXT_EXT = [
  'txt', 'md', 'log', 'json', 'xml', 'yml', 'yaml', 'ini', 'conf', 'cfg',
  'csv', 'sh', 'js', 'ts', 'css', 'html', 'nfo', 'srt', 'sub',
]

function extensionOf(name: string): string {
  const i = name.lastIndexOf('.')
  return i < 0 ? '' : name.slice(i + 1).toLowerCase()
}

/**
 * A file preview.
 *
 * We show what makes sense to look at from a phone: images and text. For
 * everything else we say plainly that there is no preview — opening gigabytes
 * of video through a messenger is pointless.
 */
export function Preview({ entry, onClose }: Props) {
  const [url, setUrl] = useState<string | null>(null)
  const [text, setText] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const ext = extensionOf(entry.name)
  const isImage = IMAGE_EXT.includes(ext)
  const isText = TEXT_EXT.includes(ext)

  useEffect(() => {
    if (!isImage && !isText) {
      setLoading(false)
      return
    }
    let revoke: string | null = null
    let cancelled = false

    void (async () => {
      try {
        const blob = await api.preview(entry.path)
        if (cancelled) return
        if (isText) {
          setText(await blob.text())
        } else {
          revoke = URL.createObjectURL(blob)
          setUrl(revoke)
        }
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof ApiError ? e.message : t('preview.failed'))
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()

    return () => {
      cancelled = true
      // Release the blob URL, otherwise the file stays in the tab's memory.
      if (revoke) URL.revokeObjectURL(revoke)
    }
  }, [entry.path, isImage, isText])

  return (
    <div className="preview-backdrop" onClick={onClose}>
      <div className="preview" onClick={(e) => e.stopPropagation()}>
        <div className="preview-head">
          <span className="preview-name">{entry.name}</span>
          <button type="button" className="icon-button small" onClick={onClose} aria-label={t('common.close')}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2.4" strokeLinecap="round" aria-hidden="true">
              <path d="M6 6l12 12" /><path d="M18 6L6 18" />
            </svg>
          </button>
        </div>

        <div className="preview-body">
          {loading && <div className="muted">{t('common.loading')}</div>}
          {error && <div className="preview-error">{error}</div>}

          {!loading && !error && url && (
            <img src={url} alt={entry.name} className="preview-image" />
          )}
          {!loading && !error && text !== null && (
            <pre className="preview-text">{text}</pre>
          )}
          {!loading && !error && !isImage && !isText && (
            <div className="muted preview-none">
              {t('preview.unsupported')}
              <br />
              {humanSize(entry.size)}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
