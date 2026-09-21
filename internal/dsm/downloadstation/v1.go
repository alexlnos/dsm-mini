package downloadstation

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// stationLegacy is the implementation on top of SYNO.DownloadStation.* .
//
// This path is for DSM 6 and older NAS units without SYNO.DownloadStation2.
// Differences from v2 that are easy to get wrong:
//   - the task list is in the "tasks" field, not "task";
//   - the status arrives as a string, not a number;
//   - additional is passed as a comma-separated string, not an array;
//   - task ids in actions are a comma-separated string too;
//   - the creation time is called create_time, not created_time.
type stationLegacy struct{ c apiClient }

func (s *stationLegacy) Generation() string { return "legacy" }

type taskLegacy struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Status     string `json:"status"`
	Username   string `json:"username"`
	Additional struct {
		Detail struct {
			Destination       string `json:"destination"`
			URI               string `json:"uri"`
			CreateTime        int64  `json:"create_time"`
			CompletedTime     int64  `json:"completed_time"`
			ConnectedSeeders  int    `json:"connected_seeders"`
			ConnectedLeechers int    `json:"connected_leechers"`
		} `json:"detail"`
		Transfer struct {
			SizeDownloaded int64 `json:"size_downloaded"`
			SizeUploaded   int64 `json:"size_uploaded"`
			SpeedDownload  int64 `json:"speed_download"`
			SpeedUpload    int64 `json:"speed_upload"`
		} `json:"transfer"`
	} `json:"additional"`
}

func (t taskLegacy) toTask() Task {
	d, tr := t.Additional.Detail, t.Additional.Transfer
	return Task{
		ID:          t.ID,
		Title:       t.Title,
		Type:        t.Type,
		Status:      StatusFromString(t.Status),
		Size:        t.Size,
		Downloaded:  tr.SizeDownloaded,
		Uploaded:    tr.SizeUploaded,
		SpeedDown:   tr.SpeedDownload,
		SpeedUp:     tr.SpeedUpload,
		Destination: d.Destination,
		Username:    t.Username,
		Seeders:     d.ConnectedSeeders,
		Leechers:    d.ConnectedLeechers,
		CreatedAt:   unixTime(d.CreateTime),
		CompletedAt: unixTime(d.CompletedTime),
	}
}

func (s *stationLegacy) List(ctx context.Context) ([]Task, error) {
	var out struct {
		Total int          `json:"total"`
		Tasks []taskLegacy `json:"tasks"`
	}
	err := s.c.Call(ctx, apiTaskLegacy, "list", 1, map[string]any{
		"additional": "detail,transfer",
		"limit":      -1,
		"offset":     0,
	}, &out)
	if err != nil {
		return nil, err
	}

	tasks := make([]Task, 0, len(out.Tasks))
	for _, t := range out.Tasks {
		tasks = append(tasks, t.toTask())
	}
	return tasks, nil
}

func (s *stationLegacy) Pause(ctx context.Context, ids []string) error {
	return s.action(ctx, "pause", ids, nil)
}

func (s *stationLegacy) Resume(ctx context.Context, ids []string) error {
	return s.action(ctx, "resume", ids, nil)
}

func (s *stationLegacy) Delete(ctx context.Context, ids []string, forceComplete bool) error {
	return s.action(ctx, "delete", ids, map[string]any{"force_complete": forceComplete})
}

func (s *stationLegacy) action(ctx context.Context, method string, ids []string, extra map[string]any) error {
	if len(ids) == 0 {
		return fmt.Errorf("no tasks given")
	}
	params := map[string]any{"id": strings.Join(ids, ",")}
	for k, v := range extra {
		params[k] = v
	}

	// The legacy API puts each task's result into an array while still
	// reporting success: without parsing the array a failure looks like one.
	var out actionResult
	if err := s.c.Call(ctx, apiTaskLegacy, method, 1, params, &out); err != nil {
		return err
	}
	return failuresToError(apiTaskLegacy, method, out)
}

func (s *stationLegacy) Create(ctx context.Context, req CreateRequest) error {
	if len(req.TorrentFile) == 0 && len(req.URLs) == 0 {
		return fmt.Errorf("neither a link nor a file was given")
	}

	if len(req.TorrentFile) > 0 {
		name := req.FileName
		if name == "" {
			name = "upload.torrent"
		}
		fields := map[string]string{}
		if req.Destination != "" {
			fields["destination"] = req.Destination
		}
		// The legacy API takes the file in a part named "file" — that is what
		// Synology's documentation calls it.
		return s.c.CallUpload(ctx, apiTaskLegacy, "create", 1, fields,
			dsm.UploadFile{Field: "file", Name: name, Data: req.TorrentFile}, nil)
	}

	params := map[string]any{"uri": strings.Join(req.URLs, ",")}
	if req.Destination != "" {
		// Unlike v2, the path is passed as-is here, without extra quotes.
		params["destination"] = req.Destination
	}
	return s.c.Call(ctx, apiTaskLegacy, "create", 1, params, nil)
}

func (s *stationLegacy) Stats(ctx context.Context) (Stats, error) {
	var out struct {
		SpeedDownload int64 `json:"speed_download"`
		SpeedUpload   int64 `json:"speed_upload"`
	}
	if err := s.c.Call(ctx, apiStatsLegacy, "getinfo", 1, nil, &out); err != nil {
		return Stats{}, err
	}
	return Stats{SpeedDown: out.SpeedDownload, SpeedUp: out.SpeedUpload}, nil
}

// The legacy API has no Volumes: volume details appeared only in
// DownloadStation2.Settings.Global. We return nothing — the Mini App simply
// will not show the free space block.
func (s *stationLegacy) Volumes(ctx context.Context) ([]Volume, error) {
	return nil, nil
}

func (s *stationLegacy) DefaultDestination(ctx context.Context) (string, error) {
	var out struct {
		DefaultDestination string `json:"default_destination"`
	}
	if err := s.c.Call(ctx, apiInfoLegacy, "getconfig", 1, nil, &out); err != nil {
		return "", err
	}
	return out.DefaultDestination, nil
}
