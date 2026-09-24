export type TaskStatus =
  | 'waiting' | 'downloading' | 'paused' | 'finishing' | 'finished'
  | 'hash_checking' | 'seeding' | 'extracting' | 'error' | 'unknown'

/**
 * Why a task failed. The server sends a key, not a sentence: the person reads
 * it in their own language, the log keeps the Download Station code.
 *
 * Listed here rather than typed as a plain string so that a reason without a
 * translation is caught by the compiler instead of by a user.
 */
export const FAIL_REASONS = [
  'diskFull', 'destination', 'link', 'timeout', 'duplicate', 'torrent', 'extract',
] as const

export type FailReason = (typeof FAIL_REASONS)[number]

export interface Task {
  id: string
  title: string
  type: string
  status: TaskStatus
  /** Why it failed — absent when Download Station did not say. */
  fail_reason?: FailReason
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

/** What the service may send unprompted. Belongs to the installation, not
 * to the person reading it: the settings screen inside DSM edits the same
 * value, and there it has no Telegram user to attach it to. */
export type NotifyMode = 'off' | 'downloads' | 'all'

export interface SettingsView extends Settings {
  suggested: string[] | null
  notifications: NotifyMode
}

export interface Overview {
  tasks: Task[]
  stats: { speed_down: number; speed_up: number }
  volumes: Volume[] | null
  default_destination: string
  api_generation: string
  /** Folders for the add screen: pinned first, then recent. */
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
  /** The NAS hands back files only for a running task. */
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
  /** Disk purpose: 'pool' | 'cache' | 'free'. The caption is built by the UI. */
  role?: string
  /** The pool name when the disk belongs to one. */
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
