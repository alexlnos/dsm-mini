package filestation

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

const apiCopyMove = "SYNO.FileStation.CopyMove"

// TransferStatus — ход копирования или переноса.
type TransferStatus struct {
	TaskID   string  `json:"task_id"`
	Finished bool    `json:"finished"`
	Progress float64 `json:"progress"`
	// Processing — файл, который обрабатывается сейчас.
	Processing string `json:"processing,omitempty"`
	// Skipped — были ли пропущены файлы с совпадающими именами.
	Skipped bool `json:"skipped,omitempty"`
}

// Copy копирует файлы и папки. Возвращает идентификатор задачи: операция
// асинхронная, за ходом следят через Status.
func (s *Station) Copy(ctx context.Context, paths []string, destination string, overwrite bool) (string, error) {
	return s.transfer(ctx, paths, destination, overwrite, false)
}

// Move переносит файлы и папки.
func (s *Station) Move(ctx context.Context, paths []string, destination string, overwrite bool) (string, error) {
	return s.transfer(ctx, paths, destination, overwrite, true)
}

func (s *Station) transfer(ctx context.Context, paths []string, destination string,
	overwrite, removeSource bool) (string, error) {

	if len(paths) == 0 {
		return "", fmt.Errorf("не выбрано ни одного файла")
	}
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		p = normalize(p)
		if p == "/" {
			return "", fmt.Errorf("нельзя переносить общую папку целиком")
		}
		clean = append(clean, p)
	}

	// Завершающий слеш в пути назначения DSM считает недопустимым именем и
	// отвечает ошибкой 418 — normalize его убирает.
	dest := normalize(destination)
	if dest == "/" {
		return "", fmt.Errorf("укажите папку назначения")
	}
	for _, p := range clean {
		if p == dest {
			return "", fmt.Errorf("папка назначения совпадает с источником")
		}
		if strings.HasPrefix(dest+"/", p+"/") {
			return "", fmt.Errorf("нельзя перенести папку внутрь самой себя")
		}
	}

	var out struct {
		TaskID string `json:"taskid"`
	}
	err := s.c.CallVersioned(ctx, apiCopyMove, "start", []int{3, 2, 1}, map[string]any{
		"path":              clean,
		"dest_folder_path":  dest,
		"overwrite":         overwrite,
		"remove_src":        removeSource,
		"accurate_progress": true,
	}, &out)
	if err != nil {
		return "", err
	}
	if out.TaskID == "" {
		return "", fmt.Errorf("NAS не вернул идентификатор задачи")
	}
	return out.TaskID, nil
}

// Status сообщает, как идёт копирование или перенос.
func (s *Station) Status(ctx context.Context, taskID string) (TransferStatus, error) {
	if strings.TrimSpace(taskID) == "" {
		return TransferStatus{}, fmt.Errorf("не указана задача")
	}
	var out struct {
		Finished       bool    `json:"finished"`
		Progress       float64 `json:"progress"`
		ProcessingPath string  `json:"processing_path"`
		SkipStatus     struct {
			Status string `json:"status"`
		} `json:"skipstatus"`
	}
	if err := s.c.CallVersioned(ctx, apiCopyMove, "status", []int{3, 2, 1},
		map[string]any{"taskid": taskID}, &out); err != nil {
		return TransferStatus{}, err
	}
	return TransferStatus{
		TaskID:     taskID,
		Finished:   out.Finished,
		Progress:   out.Progress,
		Processing: out.ProcessingPath,
		Skipped:    out.SkipStatus.Status != "" && out.SkipStatus.Status != "none",
	}, nil
}

// Stop прерывает копирование или перенос.
func (s *Station) Stop(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("не указана задача")
	}
	return s.c.CallVersioned(ctx, apiCopyMove, "stop", []int{3, 2, 1},
		map[string]any{"taskid": taskID}, nil)
}

// ThumbSize — размер миниатюры.
type ThumbSize string

const (
	ThumbSmall  ThumbSize = "small"
	ThumbMedium ThumbSize = "medium"
	ThumbLarge  ThumbSize = "large"
)

// Thumbnail возвращает миниатюру изображения.
//
// Работает только для картинок; для видео — лишь когда они лежат в общей
// папке photo. Для остальных файлов NAS отвечает ошибкой, и это нормально:
// предпросмотра у них нет.
func (s *Station) Thumbnail(ctx context.Context, path string, size ThumbSize) (*dsm.Content, error) {
	path = normalize(path)
	if path == "/" {
		return nil, fmt.Errorf("не указан файл")
	}
	if size == "" {
		size = ThumbSmall
	}
	return s.c.Fetch(ctx, apiThumb, "get", 2, map[string]any{
		"path": path,
		"size": string(size),
	})
}

// Download отдаёт содержимое файла.
func (s *Station) Download(ctx context.Context, path string) (*dsm.Content, error) {
	path = normalize(path)
	if path == "/" {
		return nil, fmt.Errorf("не указан файл")
	}
	return s.c.Fetch(ctx, apiDownload, "download", 2, map[string]any{
		"path": []string{path},
		// open — отдать содержимое; download просит браузер сохранить файл.
		"mode": "open",
	})
}
