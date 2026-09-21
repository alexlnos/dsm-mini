// Package downloadstation — работа с пакетом Download Station на Synology NAS.
//
// У Synology сосуществуют два поколения API:
//
//	SYNO.DownloadStation2.*  — DSM 7, статусы числами, requestFormat JSON
//	SYNO.DownloadStation.*   — легаси, есть и в DSM 6, статусы строками
//
// Обе реализации спрятаны за интерфейсом Station; нужная выбирается в рантайме
// по тому, что реально отдаёт SYNO.API.Info на конкретном NAS.
package downloadstation

import (
	"strings"
	"time"
)

// Status — состояние задачи.
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

// statusByCode — числовые коды DownloadStation2.
//
// Легаси-API отдаёт те же состояния строками, поэтому числа нужны только
// современному пути. Код 5 подтверждён на живом DSM 7.2.2 сверкой обоих
// API на одной задаче; остальные — из схемы Synology.
var statusByCode = map[int]Status{
	1:  StatusWaiting,
	2:  StatusDownloading,
	3:  StatusPaused,
	4:  StatusFinishing,
	5:  StatusFinished,
	6:  StatusHashChecking,
	7:  StatusSeeding,
	8:  StatusWaiting, // filehosting_waiting — для пользователя это то же ожидание
	9:  StatusExtracting,
	10: StatusError,
}

// StatusFromCode переводит числовой код DownloadStation2 в Status.
func StatusFromCode(code int) Status {
	if s, ok := statusByCode[code]; ok {
		return s
	}
	return StatusUnknown
}

// StatusFromString переводит строку легаси-API в Status.
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

// Active сообщает, что задача ещё в работе и её состояние будет меняться.
// По этому признаку решается, нужно ли часто опрашивать NAS.
func (s Status) Active() bool {
	switch s {
	case StatusWaiting, StatusDownloading, StatusFinishing, StatusHashChecking, StatusExtracting:
		return true
	}
	return false
}

// Terminal сообщает, что задача досчиталась или упала и больше не изменится
// сама по себе. По нему watcher решает, о чём слать уведомление.
func (s Status) Terminal() bool {
	return s == StatusFinished || s == StatusError
}

// Task — задача загрузки в том виде, в каком её показывает Mini App.
// Структура одинакова для обоих поколений API.
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Type        string    `json:"type"` // bt, http, ftp, nzb, emule
	Status      Status    `json:"status"`
	StatusExtra string    `json:"status_extra,omitempty"` // причина ошибки, если есть
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

// Progress — доля загруженного от 0 до 1.
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

// ETA — оценка оставшегося времени. Второе значение false, если посчитать
// нельзя: задача стоит, досчиталась или скорость нулевая.
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

// Stats — сводная статистика по всем задачам.
type Stats struct {
	SpeedDown int64 `json:"speed_down"`
	SpeedUp   int64 `json:"speed_up"`
}

// Volume — том NAS и место на нём.
type Volume struct {
	MountPoint string `json:"mount_point"`
	SizeFree   int64  `json:"size_free"`
	SizeTotal  int64  `json:"size_total"`
}

// CreateRequest — запрос на постановку задачи.
type CreateRequest struct {
	// URLs — magnet-ссылки или прямые ссылки. Взаимоисключимо с TorrentFile.
	URLs []string
	// TorrentFile — содержимое .torrent, если задача ставится файлом.
	TorrentFile []byte
	FileName    string
	// Destination — путь от корня общей папки, например "Media/TV Shows".
	// Пусто — папка по умолчанию из настроек Download Station.
	Destination string
}
