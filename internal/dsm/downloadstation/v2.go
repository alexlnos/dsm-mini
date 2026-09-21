package downloadstation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// stationV2 — реализация поверх SYNO.DownloadStation2.* (DSM 7).
type stationV2 struct{ c apiClient }

func (s *stationV2) Generation() string { return "v2" }

// taskV2 повторяет форму ответа DownloadStation2.
//
// Имена полей здесь отличаются от легаси-API мелочами, на которых легко
// ошибиться: created_time против create_time, seed_elapsed против seedelapsed.
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
	// limit -1 — отдать все задачи; постраничность тут только мешает,
	// список задач Download Station на домашнем NAS измеряется десятками.
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
		return fmt.Errorf("не указано ни одной задачи")
	}
	params := map[string]any{"id": ids}
	for k, v := range extra {
		params[k] = v
	}

	// Ответ — массив результатов по каждой задаче. Проверять его обязательно:
	// DSM отвечает success даже тогда, когда часть задач действие не приняла.
	var out actionResult
	if err := s.c.Call(ctx, apiTaskV2, method, 2, params, &out); err != nil {
		return err
	}
	return failuresToError(apiTaskV2, method, out)
}

func (s *stationV2) Create(ctx context.Context, req CreateRequest) error {
	if len(req.TorrentFile) > 0 {
		return fmt.Errorf("постановка задачи файлом .torrent пока не реализована для API v2")
	}
	if len(req.URLs) == 0 {
		return fmt.Errorf("не указано ни одной ссылки")
	}

	// В отличие от легаси-API, здесь destination обязателен: без него NAS
	// отвечает кодом 120 «destination required», даже когда папка по
	// умолчанию задана в настройках. Подставляем её сами.
	dest := req.Destination
	if dest == "" {
		var err error
		dest, err = s.DefaultDestination(ctx)
		if err != nil {
			return fmt.Errorf("не определить папку назначения: %w", err)
		}
		if dest == "" {
			return fmt.Errorf("не задана папка назначения: укажите её в запросе " +
				"или в настройках Download Station")
		}
	}

	// Путь идёт как обычная строка, относительно корня общей папки.
	// Ведущий слеш ("/Media/TV Shows") NAS отвергает кодом 403.
	// create_list=false обязателен: с true задача не создаётся, а уходит
	// в режим предварительного выбора файлов.
	params := map[string]any{
		"type":        "url",
		"url":         req.URLs,
		"create_list": false,
		"destination": strings.TrimPrefix(dest, "/"),
	}
	return s.c.Call(ctx, apiTaskV2, "create", 2, params, nil)
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
	// Размеры тут приходят строками, а не числами.
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
