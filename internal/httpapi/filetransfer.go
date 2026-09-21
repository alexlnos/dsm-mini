package httpapi

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm/filestation"
)

// maxPreviewSize limits the preview: people look at notes and pictures
// through a messenger, not at gigabyte archives.
const maxPreviewSize = 8 << 20

type transferRequest struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination"`
	Overwrite   bool     `json:"overwrite"`
}

func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	s.startTransfer(w, r, false)
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	s.startTransfer(w, r, true)
}

func (s *Server) startTransfer(w http.ResponseWriter, r *http.Request, move bool) {
	var req transferRequest
	if !decode(w, r, &req, s) {
		return
	}
	paths := cleanStrings(req.Paths)
	if len(paths) == 0 {
		s.bad(w, r, "bad.noFiles")
		return
	}

	var (
		taskID string
		err    error
	)
	if move {
		taskID, err = s.fs.Move(r.Context(), paths, req.Destination, req.Overwrite)
	} else {
		taskID, err = s.fs.Copy(r.Context(), paths, req.Destination, req.Overwrite)
	}
	if err != nil {
		s.fail(w, r, err, failKey(move))
		return
	}

	u, _ := userFrom(r.Context())
	s.log.Info("file transfer started", "user", u.ID, "move", move,
		"count", len(paths), "dest", req.Destination)
	writeJSON(w, http.StatusOK, map[string]any{"task_id": taskID})
}

func (s *Server) handleTransferStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		s.bad(w, r, "bad.noTask")
		return
	}
	status, err := s.fs.Status(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "api.transferStatus")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleTransferStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID string `json:"task_id"`
	}
	if !decode(w, r, &req, s) {
		return
	}
	if err := s.fs.Stop(r.Context(), req.TaskID); err != nil {
		s.fail(w, r, err, "api.transferStop")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleThumb serves an image thumbnail.
//
// A missing thumbnail is routine: text and archives have none. We answer 404
// so that the interface simply shows no picture.
func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.bad(w, r, "bad.noFile")
		return
	}
	size := filestation.ThumbSize(r.URL.Query().Get("size"))

	content, err := s.fs.Thumbnail(r.Context(), path, size)
	if err != nil {
		s.log.Debug("thumbnail unavailable", "path", path, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no preview available"})
		return
	}
	defer content.Body.Close()

	w.Header().Set("Content-Type", content.ContentType)
	// A thumbnail does not change while the file is in place, but the app
	// rarely asks for it again anyway — we keep it briefly.
	w.Header().Set("Cache-Control", "private, max-age=600")
	if _, err := io.Copy(w, io.LimitReader(content.Body, maxPreviewSize)); err != nil {
		s.log.Debug("the thumbnail was not fully sent", "err", err)
	}
}

// handlePreview serves file contents for viewing inside the app.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.bad(w, r, "bad.noFile")
		return
	}

	content, err := s.fs.Download(r.Context(), path)
	if err != nil {
		s.fail(w, r, err, "api.openFile")
		return
	}
	defer content.Body.Close()

	if content.Length > maxPreviewSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error": "the file is too large to preview",
			"size":  content.Length,
		})
		return
	}

	w.Header().Set("Content-Type", content.ContentType)
	if content.Length >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(content.Length, 10))
	}
	// Someone else's file contents must not execute as a page.
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, max-age=60")

	if _, err := io.Copy(w, io.LimitReader(content.Body, maxPreviewSize+1)); err != nil {
		s.log.Debug("the file was not fully sent", "err", err)
	}
}
