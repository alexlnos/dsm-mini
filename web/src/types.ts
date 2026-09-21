export type TaskStatus =
  | 'waiting' | 'downloading' | 'paused' | 'finishing' | 'finished'
  | 'hash_checking' | 'seeding' | 'extracting' | 'error' | 'unknown'

export interface Task {
  id: string
  title: string
  type: string
  status: TaskStatus
  size: number
  downloaded: number
  uploaded: number
  speed_down: number
  speed_up: number
  destination: string
  seeders: number
  leechers: number
  created_at?: string
  completed_at?: string
  progress: number
  eta_seconds?: number
  active: boolean
}

export interface Volume {
  mount_point: string
  size_free: number
  size_total: number
}

export interface Settings {
  pinned_folders: string[] | null
  show_recent: boolean
  last_used: string
}

export interface SettingsView extends Settings {
  suggested: string[] | null
}

export interface Overview {
  tasks: Task[]
  stats: { speed_down: number; speed_up: number }
  volumes: Volume[] | null
  default_destination: string
  api_generation: string
  /** Папки для экрана добавления: закреплённые, затем недавние. */
  folders: string[] | null
  settings: Settings
}

export interface Entry {
  name: string
  path: string
  is_dir: boolean
  size: number
  modified?: string
}
