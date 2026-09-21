package downloadstation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// FilePriority — приоритет файла внутри раздачи.
//
// Download Station передаёт его строкой, а не числом.
type FilePriority string

const (
	PriorityLow    FilePriority = "low"
	PriorityNormal FilePriority = "normal"
	PriorityHigh   FilePriority = "high"
)

// Valid сообщает, знаком ли приоритет. Неизвестное значение отправлять на NAS
// не стоит: он ответит невнятной ошибкой.
func (p FilePriority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh:
		return true
	}
	return false
}

// File — файл внутри раздачи.
type File struct {
	Index      int          `json:"index"`
	Name       string       `json:"name"`
	Size       int64        `json:"size"`
	Downloaded int64        `json:"downloaded"`
	Priority   FilePriority `json:"priority"`
	// Wanted — качать ли файл вообще.
	Wanted bool `json:"wanted"`
}

// Progress — доля загруженного от 0 до 1.
func (f File) Progress() float64 {
	if f.Size <= 0 {
		return 0
	}
	p := float64(f.Downloaded) / float64(f.Size)
	if p > 1 {
		return 1
	}
	return p
}

// Tracker — трекер раздачи.
type Tracker struct {
	URL    string `json:"url"`
	Status string `json:"status"`
	Seeds  int    `json:"seeds"`
	Peers  int    `json:"peers"`
}

// ErrNotActive означает, что сведения доступны только у работающей задачи.
//
// Download Station закрывает BT-сессию завершённой или остановленной задачи и
// отвечает кодом 1913 на любой запрос файлов, трекеров и пиров.
var ErrNotActive = fmt.Errorf("сведения доступны, только пока задача качается или раздаётся")

const codeNotActive = 1913

// Files возвращает файлы внутри раздачи.
func (s *stationV2) Files(ctx context.Context, taskID string) ([]File, error) {
	var out struct {
		Items []struct {
			Index      int    `json:"index"`
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			Downloaded int64  `json:"size_downloaded"`
			Priority   string `json:"priority"`
			Wanted     bool   `json:"wanted"`
		} `json:"items"`
		Total int `json:"total"`
	}
	err := s.c.Call(ctx, apiFileV2, "list", 2, map[string]any{
		"task_id": taskID,
		"offset":  0,
		// Раздачи с тысячами файлов встречаются; больше за раз всё равно не
		// покажем, а список грузится в один запрос.
		"limit": 1000,
	}, &out)
	if err != nil {
		if isNotActive(err) {
			return nil, ErrNotActive
		}
		return nil, err
	}

	files := make([]File, 0, len(out.Items))
	for _, it := range out.Items {
		files = append(files, File{
			Index:      it.Index,
			Name:       it.Name,
			Size:       it.Size,
			Downloaded: it.Downloaded,
			Priority:   FilePriority(strings.ToLower(it.Priority)),
			Wanted:     it.Wanted,
		})
	}
	return files, nil
}

// SetFile меняет приоритет файла и признак «качать ли его».
func (s *stationV2) SetFile(ctx context.Context, taskID string, indexes []int,
	priority FilePriority, wanted *bool) error {

	if len(indexes) == 0 {
		return fmt.Errorf("не выбран ни один файл")
	}
	params := map[string]any{"task_id": taskID, "index": indexes}
	if priority != "" {
		if !priority.Valid() {
			return fmt.Errorf("неизвестный приоритет %q", priority)
		}
		params["priority"] = string(priority)
	}
	if wanted != nil {
		params["wanted"] = *wanted
	}
	if len(params) == 2 {
		return fmt.Errorf("нечего менять")
	}

	err := s.c.Call(ctx, apiFileV2, "set", 2, params, nil)
	if isNotActive(err) {
		return ErrNotActive
	}
	return err
}

// Trackers возвращает трекеры раздачи.
func (s *stationV2) Trackers(ctx context.Context, taskID string) ([]Tracker, error) {
	var out struct {
		Items []Tracker `json:"items"`
	}
	err := s.c.Call(ctx, apiTrackerV2, "list", 2, map[string]any{
		"task_id": taskID, "offset": 0, "limit": 100,
	}, &out)
	if err != nil {
		if isNotActive(err) {
			return nil, ErrNotActive
		}
		return nil, err
	}
	return out.Items, nil
}

// SetDestination переносит задачу в другую папку.
//
// Путь идёт от корня общей папки и без ведущего слеша: несуществующая папка
// даёт код 1203.
func (s *stationV2) SetDestination(ctx context.Context, taskIDs []string, destination string) error {
	if len(taskIDs) == 0 {
		return fmt.Errorf("не указано ни одной задачи")
	}
	destination = strings.Trim(strings.TrimSpace(destination), "/")
	if destination == "" {
		return fmt.Errorf("не указана папка")
	}
	// Здесь параметр называется id, а не task_id, как у файлов того же API.
	var res actionResult
	if err := s.c.Call(ctx, apiTaskV2, "edit", 2, map[string]any{
		"id": taskIDs, "destination": destination,
	}, &res); err != nil {
		return err
	}
	return failuresToError(apiTaskV2, "edit", res)
}

// SetPriority меняет приоритет задачи в очереди.
func (s *stationV2) SetPriority(ctx context.Context, taskIDs []string, priority FilePriority) error {
	if len(taskIDs) == 0 {
		return fmt.Errorf("не указано ни одной задачи")
	}
	if !priority.Valid() {
		return fmt.Errorf("неизвестный приоритет %q", priority)
	}
	var res actionResult
	if err := s.c.Call(ctx, apiTaskV2, "edit", 2, map[string]any{
		"id": taskIDs, "priority": string(priority),
	}, &res); err != nil {
		return err
	}
	return failuresToError(apiTaskV2, "edit", res)
}

func isNotActive(err error) bool {
	var apiErr *dsm.APIError
	return errors.As(err, &apiErr) && apiErr.Code == codeNotActive
}
