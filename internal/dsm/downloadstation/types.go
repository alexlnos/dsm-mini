// Package downloadstation works with the Download Station package on a Synology NAS.
//
// Synology has two API generations living side by side:
//
//	SYNO.DownloadStation2.*  — DSM 7, numeric statuses, requestFormat JSON
//	SYNO.DownloadStation.*   — legacy, present in DSM 6 too, string statuses
//
// Both implementations hide behind the Station interface; the right one is
// chosen at runtime from what SYNO.API.Info actually reports on this NAS.
package downloadstation

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Status is the state of a task.
type Status string

const (
	StatusWaiting      Status = "waiting"
	StatusDownloading  Status = "downloading"
	StatusPaused       Status = "paused"
	StatusFinishing    Status = "finishing"
	StatusFinished     Status = "finished"
	StatusHashChecking Status = "hash_checking"
	StatusSeeding      Status = "seeding"
	StatusExtracting   Status = "extracting"
	StatusError        Status = "error"
	StatusUnknown      Status = "unknown"
)

// statusByCode holds the numeric codes of DownloadStation2.
//
// The legacy API reports the same states as strings, so the numbers are only
// needed on the modern path. This table is not guesswork: it is transcribed
// from getStatusString in Download Station's own interface,
// /webman/3rdparty/DownloadStation/download.js on the NAS. An earlier version
// of this map was shifted by one from code 7 on, which meant a task being
// unpacked (10) was shown to the user as a failure.
var statusByCode = map[int]Status{
	1:  StatusWaiting,
	11: StatusWaiting,
	12: StatusWaiting,
	2:  StatusDownloading,
	3:  StatusPaused,
	4:  StatusFinishing,
	13: StatusFinishing,
	14: StatusFinishing,
	5:  StatusFinished,
	6:  StatusHashChecking,
	// Pre-seeding is the moment between finishing and seeding. There is no
	// separate word for it here, and "seeding" is what it becomes.
	7: StatusSeeding,
	8: StatusSeeding,
	// filehosting_waiting — to the user this is the same waiting.
	9:  StatusWaiting,
	10: StatusExtracting,
	// captcha_needed. The task is stuck until a human types the captcha in
	// Download Station itself, so it is shown as a failure: red is the signal
	// that someone has to go and look. If file-hosting downloads ever become
	// a real scenario here, this deserves a state of its own.
	15: StatusError,
}

// StatusFromCode translates a numeric DownloadStation2 code into a Status.
//
// Everything outside the table is an error — that is what Download Station's
// own interface does, and it is why a failed task arrives as 101 with no
// string of its own. Codes 102 to 134 name the specific reason (no space on
// disk, destination denied, invalid torrent and so on); we do not show those
// yet, and the legacy API is the only one that reports them in words.
//
// Unmapped codes are still written to the log: the table above was wrong once
// and will go stale again when Synology adds a state.
func StatusFromCode(code int) Status {
	if s, ok := statusByCode[code]; ok {
		return s
	}
	if _, seen := unknownCodes.LoadOrStore(code, true); !seen {
		slog.Default().Warn("Download Station status code outside the known table, shown as an error",
			"code", code)
	}
	return StatusError
}

// unknownCodes remembers the codes already reported.
//
// The task list is polled every few seconds, so a code nobody mapped would
// otherwise repeat the same warning until the task goes away.
var unknownCodes sync.Map

// StatusFromString translates a legacy API string into a Status.
func StatusFromString(s string) Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "waiting", "filehosting_waiting":
		return StatusWaiting
	case "downloading":
		return StatusDownloading
	case "paused":
		return StatusPaused
	case "finishing":
		return StatusFinishing
	case "finished":
		return StatusFinished
	case "hash_checking":
		return StatusHashChecking
	case "seeding":
		return StatusSeeding
	case "extracting":
		return StatusExtracting
	case "error":
		return StatusError
	default:
		return StatusUnknown
	}
}

// Active reports that the task is still running and its state will keep
// changing. It decides whether the NAS has to be polled often.
func (s Status) Active() bool {
	switch s {
	case StatusWaiting, StatusDownloading, StatusFinishing, StatusHashChecking, StatusExtracting:
		return true
	}
	return false
}

// Terminal reports that the task finished or failed and will not change on
// its own any more. The watcher uses it to decide what to notify about.
func (s Status) Terminal() bool {
	return s == StatusFinished || s == StatusError
}

// Task is a download task in the shape the Mini App shows it.
// The struct is the same for both API generations.
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Type        string    `json:"type"` // bt, http, ftp, nzb, emule
	Status      Status    `json:"status"`
	StatusExtra string    `json:"status_extra,omitempty"` // failure reason, if any
	Size        int64     `json:"size"`
	Downloaded  int64     `json:"downloaded"`
	Uploaded    int64     `json:"uploaded"`
	SpeedDown   int64     `json:"speed_down"`
	SpeedUp     int64     `json:"speed_up"`
	Destination string    `json:"destination"`
	Username    string    `json:"username,omitempty"`
	Seeders     int       `json:"seeders"`
	Leechers    int       `json:"leechers"`
	CreatedAt   time.Time `json:"created_at,omitzero"`
	CompletedAt time.Time `json:"completed_at,omitzero"`
}

// Progress is the downloaded share, from 0 to 1.
func (t Task) Progress() float64 {
	if t.Size <= 0 {
		return 0
	}
	p := float64(t.Downloaded) / float64(t.Size)
	if p > 1 {
		return 1
	}
	return p
}

// ETA estimates the remaining time. The second value is false when it cannot
// be computed: the task is stopped, finished, or the speed is zero.
func (t Task) ETA() (time.Duration, bool) {
	if t.SpeedDown <= 0 || t.Size <= 0 {
		return 0, false
	}
	left := t.Size - t.Downloaded
	if left <= 0 {
		return 0, false
	}
	return time.Duration(left/t.SpeedDown) * time.Second, true
}

// Stats is the summary across all tasks.
type Stats struct {
	SpeedDown int64 `json:"speed_down"`
	SpeedUp   int64 `json:"speed_up"`
}

// Volume is a NAS volume and the space on it.
type Volume struct {
	MountPoint string `json:"mount_point"`
	SizeFree   int64  `json:"size_free"`
	SizeTotal  int64  `json:"size_total"`
}

// CreateRequest asks for a task to be queued.
type CreateRequest struct {
	// URLs are magnet or direct links. Mutually exclusive with TorrentFile.
	URLs []string
	// TorrentFile is the contents of a .torrent when the task comes as a file.
	TorrentFile []byte
	FileName    string
	// Destination is a path from the shared folder root, e.g. "Media/TV Shows".
	// Empty means the default folder from the Download Station settings.
	Destination string
}
