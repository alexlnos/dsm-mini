import type {
  Entry, FilePriority, Overview, Settings, SettingsView, Task, TaskDetails,
} from './types'

/**
 * initData — подписанная Telegram строка, доказывающая, кто открыл приложение.
 * Сервер проверяет её подпись на каждом запросе, поэтому без неё нет смысла
 * даже пытаться.
 */
let authToken = ''

export function setAuthToken(raw: string) {
  authToken = raw
}

export class ApiError extends Error {
  readonly status: number
  readonly detail?: string

  constructor(status: number, message: string, detail?: string) {
    super(message)
    this.status = status
    this.detail = detail
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  if (authToken) headers.set('Authorization', `tma ${authToken}`)
  if (init?.body && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(path, { ...init, headers })
  const text = await response.text()
  let parsed: unknown = null
  if (text) {
    try {
      parsed = JSON.parse(text)
    } catch {
      // Ответ не JSON — ниже отдадим как есть.
    }
  }

  if (!response.ok) {
    const body = parsed as { error?: string; detail?: string } | null
    throw new ApiError(
      response.status,
      body?.error ?? `Ошибка ${response.status}`,
      body?.detail,
    )
  }
  return parsed as T
}

export const api = {
  overview: () => request<Overview>('/api/overview'),

  tasks: () => request<{ tasks: Task[] }>('/api/downloads'),

  create: (urls: string[], destination: string) =>
    request<{ ok: boolean }>('/api/downloads', {
      method: 'POST',
      body: JSON.stringify({ urls, destination }),
    }),

  action: (action: 'pause' | 'resume' | 'delete', ids: string[], forceComplete = false) =>
    request<{ ok: boolean }>('/api/downloads/action', {
      method: 'POST',
      body: JSON.stringify({ action, ids, force_complete: forceComplete }),
    }),

  taskDetails: (id: string) =>
    request<TaskDetails>(`/api/downloads/files?id=${encodeURIComponent(id)}`),

  setFile: (taskId: string, indexes: number[], change: { priority?: FilePriority; wanted?: boolean }) =>
    request<{ ok: boolean }>('/api/downloads/files', {
      method: 'POST',
      body: JSON.stringify({ task_id: taskId, indexes, ...change }),
    }),

  setDestination: (ids: string[], destination: string) =>
    request<{ ok: boolean }>('/api/downloads/destination', {
      method: 'POST',
      body: JSON.stringify({ ids, destination }),
    }),

  setPriority: (ids: string[], priority: FilePriority) =>
    request<{ ok: boolean }>('/api/downloads/priority', {
      method: 'POST',
      body: JSON.stringify({ ids, priority }),
    }),

  settings: () => request<SettingsView>('/api/settings'),

  saveSettings: (settings: Settings) =>
    request<SettingsView>('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  files: (path: string) =>
    request<{ entries: Entry[] }>(`/api/files?path=${encodeURIComponent(path)}`),

  createFolder: (parent: string, name: string) =>
    request<{ path: string }>('/api/files/folder', {
      method: 'POST',
      body: JSON.stringify({ parent, name }),
    }),

  rename: (path: string, name: string) =>
    request<{ path: string }>('/api/files/rename', {
      method: 'POST',
      body: JSON.stringify({ path, name }),
    }),

  deleteFiles: (paths: string[]) =>
    request<{ ok: boolean }>('/api/files/delete', {
      method: 'POST',
      body: JSON.stringify({ paths }),
    }),

  upload: (folder: string, file: File, overwrite: boolean) => {
    const form = new FormData()
    form.set('folder', folder)
    form.set('overwrite', String(overwrite))
    form.set('file', file)
    return request<{ ok: boolean; name: string }>('/api/files/upload', {
      method: 'POST',
      body: form,
    })
  },
}
