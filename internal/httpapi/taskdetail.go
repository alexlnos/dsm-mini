package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
)

// fileView is a torrent file with its downloaded share already computed.
type fileView struct {
	downloadstation.File
	Progress float64 `json:"progress"`
}

// handleTaskFiles serves the files and trackers of a torrent.
//
// Download Station closes the BT session of a finished or stopped task, so a
// failure here is a normal state rather than a fault: the interface has to
// say so plainly.
func (s *Server) handleTaskFiles(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		s.bad(w, r, "bad.noTask")
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
		s.fail(w, r, err, "api.taskFiles")
		return
	}

	views := make([]fileView, 0, len(files))
	for _, f := range files {
		views = append(views, fileView{File: f, Progress: f.Progress()})
	}

	// Trackers are not critical: the screen is useful without them.
	trackers, err := s.ds.Trackers(ctx, id)
	if err != nil && !errors.Is(err, downloadstation.ErrNotActive) {
		s.log.Warn("cannot get the trackers", "task", id, "err", err)
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
	// Wanted is a pointer: it tells "do not change" from "untick the box".
	Wanted *bool `json:"wanted"`
}

func (s *Server) handleSetFile(w http.ResponseWriter, r *http.Request) {
	var req fileUpdateRequest
	if !decode(w, r, &req, s) {
		return
	}
	if strings.TrimSpace(req.TaskID) == "" {
		s.bad(w, r, "bad.noTask")
		return
	}
	if len(req.Indexes) == 0 {
		s.bad(w, r, "bad.noFiles")
		return
	}

	err := s.ds.SetFile(r.Context(), req.TaskID, req.Indexes,
		downloadstation.FilePriority(req.Priority), req.Wanted)
	if err != nil {
		if errors.Is(err, downloadstation.ErrNotActive) {
			s.bad(w, r, "bad.taskStopped")
			return
		}
		s.fail(w, r, err, "api.setFile")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("task files changed", "user", u.ID, "task", req.TaskID, "count", len(req.Indexes))
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
		s.bad(w, r, "bad.noTasks")
		return
	}
	if err := s.ds.SetDestination(r.Context(), ids, req.Destination); err != nil {
		s.fail(w, r, err, "api.setDestination")
		return
	}

	u, _ := userFrom(r.Context())
	if s.settings != nil {
		if err := s.settings.RememberLastUsed(r.Context(), u.ID, req.Destination); err != nil {
			s.log.Warn("cannot remember the folder", "err", err)
		}
	}
	s.log.Info("task folder changed", "user", u.ID, "dest", req.Destination)
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
		s.bad(w, r, "bad.noTasks")
		return
	}
	err := s.ds.SetPriority(r.Context(), ids, downloadstation.FilePriority(req.Priority))
	if err != nil {
		s.fail(w, r, err, "api.setPriority")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("task priority changed", "user", u.ID, "priority", req.Priority)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
