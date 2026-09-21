import { t } from './i18n'
import type {
  Container, Entry, FilePriority, Guest, LogEntry, Overview, Settings, SettingsView,
  StorageOverview, SystemOverview, Task, TaskDetails, VMHost,
} from './types'

/**
 * initData is the string signed by Telegram proving who opened the app.
 * The server checks its signature on every request, so without it there is
 * no point even trying.
 */
let authToken = ''

export function setAuthToken(raw: string) {
  authToken = raw
}

export class ApiError extends Error {
  readonly status: number
  /** Technical details for the log: we do not show them to the person. */
  readonly detail?: string
  /** The DSM error code, when there was one: digits read the same anywhere. */
  readonly code?: number

  constructor(status: number, message: string, detail?: string, code?: number) {
    super(message)
    this.status = status
    this.detail = detail
    this.code = code
  }
}

/**
 * What to show the person.
 *
 * The server sends the message already in their language, and the details in
 * English for the log. So the message goes out with the DSM code attached: it
 * shows what exactly the NAS is unhappy about and can be quoted to support.
 */
export function errorText(e: unknown, fallback: string): string {
  if (!(e instanceof ApiError)) return fallback
  return e.code ? `${e.message} (${e.code})` : e.message
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
      // The answer is not JSON — below we hand it over as it is.
    }
  }

  if (!response.ok) {
    const body = parsed as { error?: string; detail?: string; code?: number } | null
    throw new ApiError(
      response.status,
      body?.error ?? t('error.http', { status: response.status }),
      body?.detail,
      body?.code,
    )
  }
  return parsed as T
}

/**
 * Fetches a file as a blob.
 *
 * Through fetch rather than <img src>: the Telegram signature travels in a
 * header and lands neither in the address bar nor in proxy logs.
 */
async function fetchBlob(path: string): Promise<Blob> {
  const headers = new Headers()
  if (authToken) headers.set('Authorization', `tma ${authToken}`)
  const response = await fetch(path, { headers })
  if (!response.ok) {
    let message = t('error.http', { status: response.status })
    try {
      const body = await response.json() as { error?: string }
      if (body?.error) message = body.error
    } catch {
      // The answer is not JSON — keep the generic text.
    }
    throw new ApiError(response.status, message)
  }
  return response.blob()
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

  system: () => request<SystemOverview>('/api/system'),

  systemLog: (limit = 60, problems = false) =>
    request<{ entries: LogEntry[] | null }>(
      `/api/system/log?limit=${limit}${problems ? '&problems=true' : ''}`,
    ),

  storage: () => request<StorageOverview>('/api/storage'),

  vms: () => request<{ guests: Guest[] | null; host: VMHost }>('/api/vms'),

  vmAction: (id: string, action: 'start' | 'shutdown') =>
    request<{ ok: boolean }>('/api/vms/action', {
      method: 'POST',
      body: JSON.stringify({ id, action }),
    }),

  containers: () => request<{ containers: Container[] | null }>('/api/containers'),

  containerAction: (name: string, action: 'start' | 'stop' | 'restart') =>
    request<{ ok: boolean }>('/api/containers/action', {
      method: 'POST',
      body: JSON.stringify({ name, action }),
    }),

  settings: () => request<SettingsView>('/api/settings'),

  saveSettings: (settings: Settings) =>
    request<SettingsView>('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),

  copy: (paths: string[], destination: string, overwrite: boolean) =>
    request<{ task_id: string }>('/api/files/copy', {
      method: 'POST',
      body: JSON.stringify({ paths, destination, overwrite }),
    }),

  move: (paths: string[], destination: string, overwrite: boolean) =>
    request<{ task_id: string }>('/api/files/move', {
      method: 'POST',
      body: JSON.stringify({ paths, destination, overwrite }),
    }),

  transferStatus: (taskId: string) =>
    request<{ task_id: string; finished: boolean; progress: number; processing?: string; skipped?: boolean }>(
      `/api/files/transfer?id=${encodeURIComponent(taskId)}`,
    ),

  thumb: (path: string, size = 'small') =>
    fetchBlob(`/api/files/thumb?path=${encodeURIComponent(path)}&size=${size}`),

  preview: (path: string) =>
    fetchBlob(`/api/files/preview?path=${encodeURIComponent(path)}`),

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
