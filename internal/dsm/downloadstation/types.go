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
	"strings"
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
// needed on the modern path. Code 5 was confirmed on a live DSM 7.2.2 by
// comparing both APIs on one task; the rest come from Synology's schema.
var statusByCode = map[int]Status{
	1:  StatusWaiting,
	2:  StatusDownloading,
	3:  StatusPaused,
	4:  StatusFinishing,
	5:  StatusFinished,
	6:  StatusHashChecking,
	7:  StatusSeeding,
	8:  StatusWaiting, // filehosting_waiting — to the user this is the same waiting
	9:  StatusExtracting,
	10: StatusError,
}

// StatusFromCode translates a numeric DownloadStation2 code into a Status.
func StatusFromCode(code int) Status {
	if s, ok := statusByCode[code]; ok {
		return s
	}
	return StatusUnknown
}

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
