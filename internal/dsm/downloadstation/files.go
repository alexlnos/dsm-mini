package downloadstation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// FilePriority is the priority of a file inside a torrent.
//
// Download Station passes it as a string, not a number.
type FilePriority string

const (
	PriorityLow    FilePriority = "low"
	PriorityNormal FilePriority = "normal"
	PriorityHigh   FilePriority = "high"
)

// Valid reports whether the priority is known. Sending an unknown value to
// the NAS is a bad idea: it answers with an unhelpful error.
func (p FilePriority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh:
		return true
	}
	return false
}

// File is a file inside a torrent.
type File struct {
	Index      int          `json:"index"`
	Name       string       `json:"name"`
	Size       int64        `json:"size"`
	Downloaded int64        `json:"downloaded"`
	Priority   FilePriority `json:"priority"`
	// Wanted tells whether to download the file at all.
	Wanted bool `json:"wanted"`
}

// Progress is the downloaded share, from 0 to 1.
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

// Tracker is a torrent tracker.
type Tracker struct {
	URL    string `json:"url"`
	Status string `json:"status"`
	Seeds  int    `json:"seeds"`
	Peers  int    `json:"peers"`
}

// ErrNotActive means the details are only available for a running task.
//
// Download Station closes the BT session of a finished or stopped task and
// answers code 1913 to any request for files, trackers or peers.
var ErrNotActive = fmt.Errorf("details are available only while the task is downloading or seeding")

const codeNotActive = 1913

// Files returns the files inside a torrent.
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
		// Torrents with thousands of files do happen; we would not show more
		// at once anyway, and the list loads in a single request.
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

// SetFile changes a file's priority and whether to download it.
func (s *stationV2) SetFile(ctx context.Context, taskID string, indexes []int,
	priority FilePriority, wanted *bool) error {

	if len(indexes) == 0 {
		return fmt.Errorf("no files selected")
	}
	params := map[string]any{"task_id": taskID, "index": indexes}
	if priority != "" {
		if !priority.Valid() {
			return fmt.Errorf("unknown priority %q", priority)
		}
		params["priority"] = string(priority)
	}
	if wanted != nil {
		params["wanted"] = *wanted
	}
	if len(params) == 2 {
		return fmt.Errorf("nothing to change")
	}

	err := s.c.Call(ctx, apiFileV2, "set", 2, params, nil)
	if isNotActive(err) {
		return ErrNotActive
	}
	return err
}

// Trackers returns the torrent's trackers.
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

// SetDestination moves a task to another folder.
//
// The path goes from the shared folder root and without a leading slash: a
// missing folder gives code 1203.
func (s *stationV2) SetDestination(ctx context.Context, taskIDs []string, destination string) error {
	if len(taskIDs) == 0 {
		return fmt.Errorf("no tasks given")
	}
	destination = strings.Trim(strings.TrimSpace(destination), "/")
	if destination == "" {
		return fmt.Errorf("no folder given")
	}
	// Here the parameter is called id, not task_id as for files of the same API.
	var res actionResult
	if err := s.c.Call(ctx, apiTaskV2, "edit", 2, map[string]any{
		"id": taskIDs, "destination": destination,
	}, &res); err != nil {
		return err
	}
	return failuresToError(apiTaskV2, "edit", res)
}

// SetPriority changes a task's priority in the queue.
//
// Careful: in Download Station this property belongs to eMule tasks, not BT.
// For torrents the NAS accepts the call and answers success, but the priority
// is not stored and never comes back in any response — the interface shows no
// such switch. The method is kept for those who have eMule enabled.
func (s *stationV2) SetPriority(ctx context.Context, taskIDs []string, priority FilePriority) error {
	if len(taskIDs) == 0 {
		return fmt.Errorf("no tasks given")
	}
	if !priority.Valid() {
		return fmt.Errorf("unknown priority %q", priority)
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
