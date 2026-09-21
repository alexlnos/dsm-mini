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

export type FilePriority = 'low' | 'normal' | 'high'

export interface TaskFile {
  index: number
  name: string
  size: number
  downloaded: number
  priority: FilePriority
  wanted: boolean
  progress: number
}

export interface Tracker {
  url: string
  status: string
  seeds: number
  peers: number
}

export interface TaskDetails {
  files: TaskFile[] | null
  trackers: Tracker[] | null
  /** NAS отдаёт файлы только у работающей задачи. */
  not_active?: boolean
}

export interface Entry {
  name: string
  path: string
  is_dir: boolean
  size: number
  modified?: string
}

export interface SystemInfo {
  hostname: string
  model: string
  firmware: string
  cpu_vendor?: string
  cpu_series?: string
  cpu_cores: number
  ram_mb: number
  uptime_seconds: number
}

export interface SystemUsage {
  cpu_percent: number
  memory_percent: number
  memory_total_mb: number
  network_rx: number
  network_tx: number
}

export interface PackageState {
  installed: boolean
  running: boolean
  version?: string
  name?: string
}

export interface SystemOverview {
  info: SystemInfo
  usage: SystemUsage
  packages: Record<string, PackageState>
}

export interface LogEntry {
  time: string
  level: string
  type: string
  message: string
  who?: string
}

export interface Disk {
  id: string
  model: string
  vendor?: string
  size: number
  temp: number
  status: string
  smart: string
  slot: number
  is_ssd: boolean
  /** Назначение диска: 'pool' | 'cache' | 'free'. Подпись собирает интерфейс. */
  role?: string
  /** Имя пула, если диск в пуле. */
  pool?: string
  healthy: boolean
}

export interface Pool {
  id: string
  raid?: string
  status: string
  total: number
  used: number
  disks: string[]
}

export interface StorageVolume {
  id: string
  name?: string
  fs_type?: string
  status: string
  total: number
  used: number
}

export interface StorageOverview {
  disks: Disk[] | null
  pools: Pool[] | null
  volumes: StorageVolume[] | null
  healthy: boolean
}

export interface Guest {
  id: string
  name: string
  status: string
  running: boolean
  vcpu: number
  ram_mb: number
  disk_mb: number
  autorun: boolean
  storage?: string
  macs?: string[]
}

export interface VMHost {
  name: string
  free_ram_mb: number
  total_ram_mb?: number
  used_vcpu: number
  running_vms: number
  total_vms: number
}

export interface Container {
  id: string
  name: string
  image: string
  status: string
  running: boolean
  cpu: number
  memory: number
}
