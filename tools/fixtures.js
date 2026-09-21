// Демо-данные для скриншотов: ничего личного, только правдоподобное.
const now = Date.now();
const iso = (msAgo) => new Date(now - msAgo).toISOString();

const tasks = [
  { id: 'dbid_1', title: 'ubuntu-24.04.1-desktop-amd64.iso', type: 'bt', status: 'downloading',
    size: 6203355136, downloaded: 4160749568, uploaded: 524288000, speed_down: 11534336,
    speed_up: 1048576, destination: 'Downloads/ISO', seeders: 42, leechers: 7,
    created_at: iso(3600e3), progress: 0.67, eta_seconds: 178, active: true },
  { id: 'dbid_2', title: 'Blender Open Movie — Big Buck Bunny (1080p)', type: 'bt', status: 'seeding',
    size: 1073741824, downloaded: 1073741824, uploaded: 3221225472, speed_down: 0,
    speed_up: 2097152, destination: 'Media/Movies', seeders: 3, leechers: 18,
    created_at: iso(86400e3), completed_at: iso(80000e3), progress: 1, active: true },
  { id: 'dbid_3', title: 'debian-12.7.0-amd64-netinst.iso', type: 'bt', status: 'paused',
    size: 661651456, downloaded: 231211008, uploaded: 0, speed_down: 0, speed_up: 0,
    destination: 'Downloads/ISO', seeders: 0, leechers: 0, created_at: iso(7200e3),
    progress: 0.35, active: false },
  { id: 'dbid_4', title: 'LibreOffice_24.8.2_MacOS_aarch64.dmg', type: 'http', status: 'finished',
    size: 357564416, downloaded: 357564416, uploaded: 0, speed_down: 0, speed_up: 0,
    destination: 'Downloads', seeders: 0, leechers: 0, created_at: iso(172800e3),
    completed_at: iso(171000e3), progress: 1, active: false },
];

module.exports = {
  '/api/overview': {
    tasks,
    stats: { speed_download: 11534336, speed_upload: 3145728 },
    volumes: [{ id: 'volume_1', size_free: 5931641045811, size_total: 11446382665728 }],
    default_destination: 'Downloads',
    folders: ['Downloads', 'Downloads/ISO', 'Media/Movies', 'Media/Music'],
    settings: { pinned_folders: ['Downloads', 'Media/Movies'], show_recent: true,
      last_used: 'Downloads/ISO', suggested: ['Media/Music'] },
    api_generation: 2,
  },
  '/api/system': {
    info: { hostname: 'NAS', model: 'DS923+', firmware: '7.2.2-72806', cpu_vendor: 'AMD',
      cpu_series: 'R1600', cpu_cores: 4, ram_mb: 8192, uptime_seconds: 1310400 },
    usage: { cpu_percent: 12, memory_percent: 46, network_rx: 11534336, network_tx: 3145728 },
    packages: {
      DownloadStation: { installed: true, running: true },
      FileStation: { installed: true, running: true },
      Virtualization: { installed: true, running: true },
      ContainerManager: { installed: true, running: true },
    },
  },
  '/api/storage': {
    healthy: true,
    disks: [
      { id: 'sata1', model: 'WD40EFPX', vendor: 'WDC', size: 4000787030016, temp: 36,
        status: 'normal', smart: 'normal', slot: 1, is_ssd: false, role: 'pool', pool: 'reuse_1', healthy: true },
      { id: 'sata2', model: 'WD40EFPX', vendor: 'WDC', size: 4000787030016, temp: 38,
        status: 'normal', smart: 'normal', slot: 2, is_ssd: false, role: 'pool', pool: 'reuse_1', healthy: true },
      { id: 'sata3', model: 'ST8000VN004', vendor: 'Seagate', size: 8001563222016, temp: 41,
        status: 'normal', smart: 'normal', slot: 3, is_ssd: false, role: 'pool', pool: 'reuse_2', healthy: true },
      { id: 'nvme1', model: 'SNV3410-400G', vendor: 'Synology', size: 400088457216, temp: 44,
        status: 'normal', smart: 'normal', slot: 1, is_ssd: true, role: 'cache', healthy: true },
    ],
    pools: [
      { id: 'reuse_1', total: 4000787030016, raid: 'SHR', disks: ['sata1', 'sata2'] },
      { id: 'reuse_2', total: 8001563222016, raid: 'Basic', disks: ['sata3'] },
    ],
    volumes: [
      { id: 'volume_1', name: '', total: 3865470566400, used: 1932735283200, fs_type: 'btrfs', status: 'normal' },
      { id: 'volume_2', name: '', total: 7696581394432, used: 5497558138880, fs_type: 'btrfs', status: 'normal' },
    ],
  },
  '/api/vms': {
    host: { name: 'NAS', free_ram_mb: 3072, total_ram_mb: 8192, used_vcpu: 4, running_vms: 2, total_vms: 3 },
    guests: [
      { id: 'vm-1', name: 'home-assistant', status: 'running', running: true, vcpu: 2,
        ram_mb: 2048, disk_mb: 32768, autorun: true },
      { id: 'vm-2', name: 'ubuntu-server', status: 'running', running: true, vcpu: 2,
        ram_mb: 4096, disk_mb: 65536, autorun: false },
      { id: 'vm-3', name: 'test-bench', status: 'shutdown', running: false, vcpu: 1,
        ram_mb: 1024, disk_mb: 16384, autorun: false },
    ],
  },
  '/api/containers': {
    containers: [
      { id: 'c1', name: 'nginx', image: 'nginx:alpine', status: 'running', running: true, cpu: 0.4, memory: 12582912 },
      { id: 'c2', name: 'vaultwarden', image: 'vaultwarden/server:latest', status: 'running', running: true, cpu: 1.2, memory: 58720256 },
      { id: 'c3', name: 'postgres', image: 'postgres:16-alpine', status: 'exited', running: false, cpu: 0, memory: 0 },
    ],
  },
  '/api/system/log': {
    entries: [
      { level: 'info', message: 'System started up.', time: iso(1800e3) },
      { level: 'info', message: 'User [admin] logged in via [DSM].', time: iso(5400e3) },
      { level: 'warning', message: 'Volume 2 usage is above 70%.', time: iso(9000e3) },
      { level: 'info', message: 'Package [Download Station] was updated.', time: iso(90000e3) },
      { level: 'error', message: 'Failed to connect to external storage.', time: iso(176400e3) },
      { level: 'info', message: 'Scheduled S.M.A.R.T. test finished.', time: iso(180000e3) },
    ],
  },
  '/api/files': {
    entries: [
      { name: 'Downloads', path: '/Downloads', is_dir: true, size: 0, modified: iso(3600e3) },
      { name: 'Media', path: '/Media', is_dir: true, size: 0, modified: iso(86400e3) },
      { name: 'Documents', path: '/Documents', is_dir: true, size: 0, modified: iso(172800e3) },
      { name: 'Backups', path: '/Backups', is_dir: true, size: 0, modified: iso(259200e3) },
      { name: 'homes', path: '/homes', is_dir: true, size: 0, modified: iso(345600e3) },
      { name: 'photo', path: '/photo', is_dir: true, size: 0, modified: iso(432000e3) },
      { name: 'ubuntu-24.04.1-desktop-amd64.iso', path: '/ubuntu-24.04.1-desktop-amd64.iso',
        is_dir: false, size: 6203355136, modified: iso(3600e3) },
      { name: 'quarterly-report.pdf', path: '/quarterly-report.pdf', is_dir: false,
        size: 2411724, modified: iso(90000e3) },
    ],
  },
  '/api/settings': { pinned_folders: ['Downloads', 'Media/Movies'], show_recent: true,
    last_used: 'Downloads/ISO', suggested: ['Media/Music'] },
};
