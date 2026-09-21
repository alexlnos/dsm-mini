package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
)

// fileView — файл раздачи с уже посчитанной долей загруженного.
type fileView struct {
	downloadstation.File
	Progress float64 `json:"progress"`
}

// handleTaskFiles отдаёт файлы и трекеры раздачи.
//
// Download Station закрывает BT-сессию у завершённой и остановленной задачи,
// поэтому «не получилось» здесь — обычное состояние, а не сбой: интерфейс
// должен сказать об этом прямо.
func (s *Server) handleTaskFiles(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		s.bad(w, "не указана задача")
		return
	}
	ctx := r.Context()

	files, err := s.ds.Files(ctx, id)
	if err != nil {
		if errors.Is(err, downloadstation.ErrNotActive) {
			writeJSON(w, http.StatusOK, map[string]any{
				"files":      nil,
				"trackers":   nil,
				"not_active": true,
			})
			return
		}
		s.fail(w, r, err, "не получить файлы задачи")
		return
	}

	views := make([]fileView, 0, len(files))
	for _, f := range files {
		views = append(views, fileView{File: f, Progress: f.Progress()})
	}

	// Трекеры не критичны: без них экран всё равно полезен.
	trackers, err := s.ds.Trackers(ctx, id)
	if err != nil && !errors.Is(err, downloadstation.ErrNotActive) {
		s.log.Warn("не получить трекеры", "task", id, "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"files":    views,
		"trackers": trackers,
	})
}

type fileUpdateRequest struct {
	TaskID   string `json:"task_id"`
	Indexes  []int  `json:"indexes"`
	Priority string `json:"priority"`
	// Wanted указателем: отличаем «не менять» от «снять галочку».
	Wanted *bool `json:"wanted"`
}

func (s *Server) handleSetFile(w http.ResponseWriter, r *http.Request) {
	var req fileUpdateRequest
	if !decode(w, r, &req, s) {
		return
	}
	if strings.TrimSpace(req.TaskID) == "" {
		s.bad(w, "не указана задача")
		return
	}
	if len(req.Indexes) == 0 {
		s.bad(w, "не выбран ни один файл")
		return
	}

	err := s.ds.SetFile(r.Context(), req.TaskID, req.Indexes,
		downloadstation.FilePriority(req.Priority), req.Wanted)
	if err != nil {
		if errors.Is(err, downloadstation.ErrNotActive) {
			s.bad(w, "файлы можно менять, только пока задача качается")
			return
		}
		s.fail(w, r, err, "не изменить файл")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("файлы задачи изменены", "user", u.ID, "task", req.TaskID, "count", len(req.Indexes))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type destinationRequest struct {
	IDs         []string `json:"ids"`
	Destination string   `json:"destination"`
}

func (s *Server) handleSetDestination(w http.ResponseWriter, r *http.Request) {
	var req destinationRequest
	if !decode(w, r, &req, s) {
		return
	}
	ids := cleanStrings(req.IDs)
	if len(ids) == 0 {
		s.bad(w, "не выбрано ни одной задачи")
		return
	}
	if err := s.ds.SetDestination(r.Context(), ids, req.Destination); err != nil {
		s.fail(w, r, err, "не сменить папку")
		return
	}

	u, _ := userFrom(r.Context())
	if s.settings != nil {
		if err := s.settings.RememberLastUsed(r.Context(), u.ID, req.Destination); err != nil {
			s.log.Warn("не запомнить папку", "err", err)
		}
	}
	s.log.Info("папка задачи изменена", "user", u.ID, "dest", req.Destination)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type priorityRequest struct {
	IDs      []string `json:"ids"`
	Priority string   `json:"priority"`
}

func (s *Server) handleSetPriority(w http.ResponseWriter, r *http.Request) {
	var req priorityRequest
	if !decode(w, r, &req, s) {
		return
	}
	ids := cleanStrings(req.IDs)
	if len(ids) == 0 {
		s.bad(w, "не выбрано ни одной задачи")
		return
	}
	err := s.ds.SetPriority(r.Context(), ids, downloadstation.FilePriority(req.Priority))
	if err != nil {
		s.fail(w, r, err, "не изменить приоритет")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("приоритет задач изменён", "user", u.ID, "priority", req.Priority)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
