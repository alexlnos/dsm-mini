package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/i18n"
)

// maxUploadSize limits the file the Mini App will accept.
// A webview rarely sends more, and the limit protects memory.
const maxUploadSize = 256 << 20

// taskView is a task in the shape the interface likes: everything is already
// computed on the server so the Mini App does no arithmetic.
type taskView struct {
	downloadstation.Task
	Progress   float64 `json:"progress"`
	ETASeconds int64   `json:"eta_seconds,omitempty"`
	Active     bool    `json:"active"`
}

func view(t downloadstation.Task) taskView {
	v := taskView{Task: t, Progress: t.Progress(), Active: t.Status.Active()}
	if eta, ok := t.ETA(); ok {
		v.ETASeconds = int64(eta.Seconds())
	}
	return v
}

// handleOverview serves everything the first screen needs in one request:
// tasks, combined speeds, volumes and the default folder.
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tasks, err := s.tasksCache.GetStale(ctx, s.ds.List)
	if err != nil {
		s.fail(w, r, err, "api.tasks")
		return
	}
	views := make([]taskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, view(t))
	}

	// The rest is not critical: if the NAS kept quiet about something, the
	// screen must still open with the task list.
	stats, err := s.ds.Stats(ctx)
	if err != nil {
		s.log.Warn("cannot get the statistics", "err", err)
	}
	volumes, err := s.ds.Volumes(ctx)
	if err != nil {
		s.log.Warn("cannot get the volumes", "err", err)
	}
	dest, err := s.ds.DefaultDestination(ctx)
	if err != nil {
		s.log.Warn("cannot get the default folder", "err", err)
	}

	u, _ := userFrom(ctx)
	settings := s.userSettings(ctx, u.ID)
	if settings.LastUsed == "" {
		settings.LastUsed = dest
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tasks":               views,
		"stats":               stats,
		"volumes":             volumes,
		"default_destination": dest,
		"api_generation":      s.ds.Generation(),
		"folders":             s.folderChoices(r, u.ID),
		"settings":            settings,
	})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.tasksCache.GetStale(r.Context(), s.ds.List)
	if err != nil {
		s.fail(w, r, err, "api.tasks")
		return
	}
	views := make([]taskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, view(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": views})
}

type createRequest struct {
	URLs        []string `json:"urls"`
	Destination string   `json:"destination"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decode(w, r, &req, s) {
		return
	}

	urls := cleanStrings(req.URLs)
	if len(urls) == 0 {
		s.bad(w, r, "bad.noLinks")
		return
	}
	for _, u := range urls {
		if !looksLikeDownloadURL(u) {
			s.bad(w, r, "bad.badLink", i18n.P{"value": shorten(u)})
			return
		}
	}

	if err := s.ds.Create(r.Context(), downloadstation.CreateRequest{
		URLs:        urls,
		Destination: req.Destination,
	}); err != nil {
		s.fail(w, r, err, "api.create")
		return
	}
	// The list has just changed — the cache must notice.
	s.tasksCache.Invalidate()

	u, _ := userFrom(r.Context())
	if s.settings != nil && req.Destination != "" {
		if err := s.settings.RememberLastUsed(r.Context(), u.ID, req.Destination); err != nil {
			s.log.Warn("cannot remember the folder", "user", u.ID, "err", err)
		}
	}
	s.log.Info("task queued", "user", u.ID, "count", len(urls), "dest", req.Destination)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(urls)})
}

type actionRequest struct {
	Action        string   `json:"action"`
	IDs           []string `json:"ids"`
	ForceComplete bool     `json:"force_complete"`
}

func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
	var req actionRequest
	if !decode(w, r, &req, s) {
		return
	}
	ids := cleanStrings(req.IDs)
	if len(ids) == 0 {
		s.bad(w, r, "bad.noTasks")
		return
	}

	ctx := r.Context()
	var err error
	switch req.Action {
	case "pause":
		err = s.ds.Pause(ctx, ids)
	case "resume":
		err = s.ds.Resume(ctx, ids)
	case "delete":
		err = s.ds.Delete(ctx, ids, req.ForceComplete)
	default:
		s.bad(w, r, "bad.badAction", i18n.P{"value": shorten(req.Action)})
		return
	}
	if err != nil {
		s.fail(w, r, err, "api.action")
		return
	}
	s.tasksCache.Invalidate()

	u, _ := userFrom(ctx)
	s.log.Info("action on tasks", "user", u.ID, "action", req.Action, "count", len(ids))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	entries, err := s.fs.List(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.fail(w, r, err, "api.readFolder")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

type folderRequest struct {
	Parent string `json:"parent"`
	Name   string `json:"name"`
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var req folderRequest
	if !decode(w, r, &req, s) {
		return
	}
	path, err := s.fs.CreateFolder(r.Context(), req.Parent, strings.TrimSpace(req.Name))
	if err != nil {
		s.fail(w, r, err, "api.createFolder")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path})
}

type renameRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if !decode(w, r, &req, s) {
		return
	}
	path, err := s.fs.Rename(r.Context(), req.Path, strings.TrimSpace(req.Name))
	if err != nil {
		s.fail(w, r, err, "api.rename")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path})
}

type deleteRequest struct {
	Paths []string `json:"paths"`
}

func (s *Server) handleDeleteFiles(w http.ResponseWriter, r *http.Request) {
	var req deleteRequest
	if !decode(w, r, &req, s) {
		return
	}
	paths := cleanStrings(req.Paths)
	if len(paths) == 0 {
		s.bad(w, r, "bad.noFiles")
		return
	}
	if err := s.fs.Delete(r.Context(), paths); err != nil {
		s.fail(w, r, err, "api.delete")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("files deleted", "user", u.ID, "count", len(paths))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.bad(w, r, "bad.badForm")
		return
	}
	folder := r.FormValue("folder")
	if strings.TrimSpace(folder) == "" {
		s.bad(w, r, "bad.noFolder")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.bad(w, r, "bad.noAttachment")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		s.fail(w, r, err, "api.readFile")
		return
	}
	if len(data) > maxUploadSize {
		s.bad(w, r, "bad.tooBig")
		return
	}

	name := sanitizeFileName(header.Filename)
	if name == "" {
		s.bad(w, r, "bad.badName")
		return
	}

	if err := s.fs.Upload(r.Context(), folder, name, data, r.FormValue("overwrite") == "true"); err != nil {
		s.fail(w, r, err, "api.upload")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("file uploaded", "user", u.ID, "folder", folder, "name", name, "bytes", len(data))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

// sanitizeFileName keeps only the base part of the name it was sent:
// a client may send anything, including a path with "..".
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if name == "." || name == ".." {
		return ""
	}
	return name
}

func looksLikeDownloadURL(u string) bool {
	lower := strings.ToLower(u)
	for _, p := range []string{"magnet:", "http://", "https://", "ftp://", "ftps://", "ed2k://"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func shorten(s string) string {
	const max = 60
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func decode(w http.ResponseWriter, r *http.Request, dst any, s *Server) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		s.bad(w, r, "bad.badRequest")
		return false
	}
	return true
}

// bad answers a bad request in the language of whoever sent it.
func (s *Server) bad(w http.ResponseWriter, r *http.Request, key string, params ...i18n.P) {
	u, _ := userFrom(r.Context())
	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": i18n.T(i18n.Match(u.Language), key, params...),
	})
}

// failKey picks the message by action: copying and moving fail the same way
// but must read differently.
func failKey(move bool) string {
	if move {
		return "api.moveFailed"
	}
	return "api.copyFailed"
}

// fail answers with an error and writes it to the log.
//
// The person gets the message in their own language; the log stays English,
// like the rest of the code. The DSM error code goes as a separate field — a
// number reads the same in any language and helps when digging.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error, key string) {
	u, _ := userFrom(r.Context())
	s.log.Error(i18n.T(i18n.Fallback, key), "user", u.ID, "path", r.URL.Path, "err", err)

	status := http.StatusBadGateway
	body := map[string]any{
		"error":  i18n.T(i18n.Match(u.Language), key),
		"detail": err.Error(),
	}
	var apiErr *dsm.APIError
	if errors.As(err, &apiErr) {
		status = http.StatusConflict
		body["code"] = apiErr.Code
	}
	writeJSON(w, status, body)
}
