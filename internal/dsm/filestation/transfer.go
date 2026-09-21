package filestation

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

const apiCopyMove = "SYNO.FileStation.CopyMove"

// TransferStatus is the progress of a copy or a move.
type TransferStatus struct {
	TaskID   string  `json:"task_id"`
	Finished bool    `json:"finished"`
	Progress float64 `json:"progress"`
	// Processing is the file being handled right now.
	Processing string `json:"processing,omitempty"`
	// Skipped tells whether files with clashing names were skipped.
	Skipped bool `json:"skipped,omitempty"`
}

// Copy copies files and folders. Returns a task id: the operation is
// asynchronous and is followed through Status.
func (s *Station) Copy(ctx context.Context, paths []string, destination string, overwrite bool) (string, error) {
	return s.transfer(ctx, paths, destination, overwrite, false)
}

// Move moves files and folders.
func (s *Station) Move(ctx context.Context, paths []string, destination string, overwrite bool) (string, error) {
	return s.transfer(ctx, paths, destination, overwrite, true)
}

func (s *Station) transfer(ctx context.Context, paths []string, destination string,
	overwrite, removeSource bool) (string, error) {

	if len(paths) == 0 {
		return "", fmt.Errorf("no files selected")
	}
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		p = normalize(p)
		if p == "/" {
			return "", fmt.Errorf("a whole shared folder cannot be moved")
		}
		clean = append(clean, p)
	}

	// A trailing slash in the destination is treated by DSM as an invalid name
	// and answered with error 418 — normalize strips it.
	dest := normalize(destination)
	if dest == "/" {
		return "", fmt.Errorf("name a destination folder")
	}
	for _, p := range clean {
		if p == dest {
			return "", fmt.Errorf("the destination folder is the same as the source")
		}
		if strings.HasPrefix(dest+"/", p+"/") {
			return "", fmt.Errorf("a folder cannot be moved inside itself")
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
		return "", fmt.Errorf("the NAS returned no task id")
	}
	return out.TaskID, nil
}

// Status reports how the copy or move is going.
func (s *Station) Status(ctx context.Context, taskID string) (TransferStatus, error) {
	if strings.TrimSpace(taskID) == "" {
		return TransferStatus{}, fmt.Errorf("no task given")
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

// Stop interrupts a copy or a move.
func (s *Station) Stop(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("no task given")
	}
	return s.c.CallVersioned(ctx, apiCopyMove, "stop", []int{3, 2, 1},
		map[string]any{"taskid": taskID}, nil)
}

// ThumbSize is a thumbnail size.
type ThumbSize string

const (
	ThumbSmall  ThumbSize = "small"
	ThumbMedium ThumbSize = "medium"
	ThumbLarge  ThumbSize = "large"
)

// Thumbnail returns an image thumbnail.
//
// Works only for pictures; for video only when it lives in the photo shared
// folder. For everything else the NAS answers with an error, and that is
// fine: those files have no preview.
func (s *Station) Thumbnail(ctx context.Context, path string, size ThumbSize) (*dsm.Content, error) {
	path = normalize(path)
	if path == "/" {
		return nil, fmt.Errorf("no file given")
	}
	if size == "" {
		size = ThumbSmall
	}
	return s.c.Fetch(ctx, apiThumb, "get", 2, map[string]any{
		"path": path,
		"size": string(size),
	})
}

// Download hands back the contents of a file.
func (s *Station) Download(ctx context.Context, path string) (*dsm.Content, error) {
	path = normalize(path)
	if path == "/" {
		return nil, fmt.Errorf("no file given")
	}
	return s.c.Fetch(ctx, apiDownload, "download", 2, map[string]any{
		"path": []string{path},
		// open hands back the contents; download asks the browser to save it.
		"mode": "open",
	})
}
