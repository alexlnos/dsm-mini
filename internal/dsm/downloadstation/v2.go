package downloadstation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// stationV2 is the implementation on top of SYNO.DownloadStation2.* (DSM 7).
type stationV2 struct{ c apiClient }

func (s *stationV2) Generation() string { return "v2" }

// taskV2 mirrors the shape of a DownloadStation2 response.
//
// Field names here differ from the legacy API in small details that are easy
// to get wrong: created_time vs create_time, seed_elapsed vs seedelapsed.
type taskV2 struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Status     int    `json:"status"`
	Username   string `json:"username"`
	Additional struct {
		Detail struct {
			Destination       string `json:"destination"`
			URI               string `json:"uri"`
			CreatedTime       int64  `json:"created_time"`
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

func (t taskV2) toTask() Task {
	d, tr := t.Additional.Detail, t.Additional.Transfer
	return Task{
		ID:          t.ID,
		Title:       t.Title,
		Type:        t.Type,
		Status:      StatusFromCode(t.Status),
		FailReason:  ReasonFromCode(t.Status),
		Size:        t.Size,
		Downloaded:  tr.SizeDownloaded,
		Uploaded:    tr.SizeUploaded,
		SpeedDown:   tr.SpeedDownload,
		SpeedUp:     tr.SpeedUpload,
		Destination: d.Destination,
		Username:    t.Username,
		Seeders:     d.ConnectedSeeders,
		Leechers:    d.ConnectedLeechers,
		CreatedAt:   unixTime(d.CreatedTime),
		CompletedAt: unixTime(d.CompletedTime),
	}
}

func (s *stationV2) List(ctx context.Context) ([]Task, error) {
	var out struct {
		Total int      `json:"total"`
		Task  []taskV2 `json:"task"`
	}
	// limit -1 means give me every task; paging only gets in the way here, as
	// a Download Station list on a home NAS is measured in tens.
	err := s.c.Call(ctx, apiTaskV2, "list", 2, map[string]any{
		"additional": []string{"detail", "transfer"},
		"limit":      -1,
		"offset":     0,
	}, &out)
	if err != nil {
		return nil, err
	}

	tasks := make([]Task, 0, len(out.Task))
	for _, t := range out.Task {
		tasks = append(tasks, t.toTask())
	}
	return tasks, nil
}

func (s *stationV2) Pause(ctx context.Context, ids []string) error {
	return s.action(ctx, "pause", ids, nil)
}

func (s *stationV2) Resume(ctx context.Context, ids []string) error {
	return s.action(ctx, "resume", ids, nil)
}

func (s *stationV2) Delete(ctx context.Context, ids []string, forceComplete bool) error {
	return s.action(ctx, "delete", ids, map[string]any{"force_complete": forceComplete})
}

func (s *stationV2) action(ctx context.Context, method string, ids []string, extra map[string]any) error {
	if len(ids) == 0 {
		return fmt.Errorf("no tasks given")
	}
	params := map[string]any{"id": ids}
	for k, v := range extra {
		params[k] = v
	}

	// The answer is an array of per-task results. Checking it is mandatory:
	// DSM answers success even when some tasks refused the action.
	var out actionResult
	if err := s.c.Call(ctx, apiTaskV2, method, 2, params, &out); err != nil {
		return err
	}
	return failuresToError(apiTaskV2, method, out)
}

func (s *stationV2) Create(ctx context.Context, req CreateRequest) error {
	if len(req.TorrentFile) == 0 && len(req.URLs) == 0 {
		return fmt.Errorf("neither a link nor a file was given")
	}

	// Unlike the legacy API, destination is mandatory here: without it the NAS
	// answers with code 120 "destination required", even when a default folder
	// is configured. We fill it in ourselves.
	dest := req.Destination
	if dest == "" {
		var err error
		dest, err = s.DefaultDestination(ctx)
		if err != nil {
			return fmt.Errorf("cannot determine the destination folder: %w", err)
		}
		if dest == "" {
			return fmt.Errorf("no destination folder is set: give one in the request " +
				"or in the Download Station settings")
		}
	}

	// The path goes as a plain string, relative to the shared folder root.
	// A leading slash ("/Media/TV Shows") is rejected by the NAS with code 403.
	// create_list=false is mandatory: with true the task is not created but
	// goes into the pre-select files mode instead.
	dest = strings.TrimPrefix(dest, "/")

	if len(req.TorrentFile) > 0 {
		return s.createFromFile(ctx, req, dest)
	}

	params := map[string]any{
		"type":        "url",
		"url":         req.URLs,
		"create_list": false,
		"destination": dest,
	}
	return s.c.Call(ctx, apiTaskV2, "create", 2, params, nil)
}

// createFromFile queues a task from the contents of a .torrent.
//
// It has its own parameter encoding, matching neither ordinary calls nor a
// File Station upload: form field values go AS JSON — strings quoted, lists
// in brackets — while File Station in the same multipart expects them without
// quotes. The file part name must match the "file" element.
func (s *stationV2) createFromFile(ctx context.Context, req CreateRequest, dest string) error {
	name := req.FileName
	if name == "" {
		name = "upload.torrent"
	}
	const part = "torrent"

	return s.c.CallUpload(ctx, apiTaskV2, "create", 2, map[string]string{
		"type":        strconv.Quote("file"),
		"file":        `["` + part + `"]`,
		"create_list": "false",
		"destination": strconv.Quote(dest),
	}, dsm.UploadFile{Field: part, Name: name, Data: req.TorrentFile}, nil)
}

func (s *stationV2) Stats(ctx context.Context) (Stats, error) {
	var out struct {
		SpeedDownload int64 `json:"download_rate"`
		SpeedUpload   int64 `json:"upload_rate"`
	}
	if err := s.c.Call(ctx, apiStatisticV2, "get", 1, nil, &out); err != nil {
		return Stats{}, err
	}
	return Stats{SpeedDown: out.SpeedDownload, SpeedUp: out.SpeedUpload}, nil
}

func (s *stationV2) Volumes(ctx context.Context) ([]Volume, error) {
	// Sizes arrive as strings here, not numbers.
	var out struct {
		VolumeList []struct {
			MountPoint string `json:"mount_point"`
			SizeFree   string `json:"size_free"`
			SizeTotal  string `json:"size_total"`
		} `json:"volume_list"`
	}
	if err := s.c.Call(ctx, apiSettingsV2, "get", 2, nil, &out); err != nil {
		return nil, err
	}

	vols := make([]Volume, 0, len(out.VolumeList))
	for _, v := range out.VolumeList {
		free, _ := strconv.ParseInt(v.SizeFree, 10, 64)
		total, _ := strconv.ParseInt(v.SizeTotal, 10, 64)
		vols = append(vols, Volume{MountPoint: v.MountPoint, SizeFree: free, SizeTotal: total})
	}
	return vols, nil
}

func (s *stationV2) DefaultDestination(ctx context.Context) (string, error) {
	var out struct {
		DefaultDestination string `json:"default_destination"`
	}
	if err := s.c.Call(ctx, apiLocationV2, "get", 1, nil, &out); err != nil {
		return "", err
	}
	return out.DefaultDestination, nil
}

func unixTime(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}
