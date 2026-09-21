package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
)

// maxUploadSize ограничивает файл, который примет Mini App.
// Через webview больше обычно и не отправляют, а лимит защищает память.
const maxUploadSize = 256 << 20

// taskView — задача в виде, удобном интерфейсу: всё уже посчитано на сервере,
// чтобы Mini App не занимался арифметикой.
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

// handleOverview отдаёт всё, что нужно для первого экрана, одним запросом:
// задачи, суммарные скорости, тома и папку по умолчанию.
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tasks, err := s.tasksCache.GetStale(ctx, s.ds.List)
	if err != nil {
		s.fail(w, r, err, "не получить список задач")
		return
	}
	views := make([]taskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, view(t))
	}

	// Остальное не критично: если NAS о чём-то умолчал, экран всё равно
	// должен открыться со списком задач.
	stats, err := s.ds.Stats(ctx)
	if err != nil {
		s.log.Warn("не получить статистику", "err", err)
	}
	volumes, err := s.ds.Volumes(ctx)
	if err != nil {
		s.log.Warn("не получить тома", "err", err)
	}
	dest, err := s.ds.DefaultDestination(ctx)
	if err != nil {
		s.log.Warn("не получить папку по умолчанию", "err", err)
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
		s.fail(w, r, err, "не получить список задач")
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
		s.bad(w, "не указано ни одной ссылки")
		return
	}
	for _, u := range urls {
		if !looksLikeDownloadURL(u) {
			s.bad(w, "ссылка не похожа на magnet или http-адрес: "+shorten(u))
			return
		}
	}

	if err := s.ds.Create(r.Context(), downloadstation.CreateRequest{
		URLs:        urls,
		Destination: req.Destination,
	}); err != nil {
		s.fail(w, r, err, "не поставить задачу")
		return
	}
	// Список только что изменился — кэш обязан это заметить.
	s.tasksCache.Invalidate()

	u, _ := userFrom(r.Context())
	if s.settings != nil && req.Destination != "" {
		if err := s.settings.RememberLastUsed(r.Context(), u.ID, req.Destination); err != nil {
			s.log.Warn("не запомнить папку", "user", u.ID, "err", err)
		}
	}
	s.log.Info("задача поставлена", "user", u.ID, "count", len(urls), "dest", req.Destination)
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
		s.bad(w, "не выбрано ни одной задачи")
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
		s.bad(w, "неизвестное действие: "+shorten(req.Action))
		return
	}
	if err != nil {
		s.fail(w, r, err, "действие не выполнено")
		return
	}
	s.tasksCache.Invalidate()

	u, _ := userFrom(ctx)
	s.log.Info("действие над задачами", "user", u.ID, "action", req.Action, "count", len(ids))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	entries, err := s.fs.List(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.fail(w, r, err, "не прочитать папку")
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
		s.fail(w, r, err, "не создать папку")
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
		s.fail(w, r, err, "не переименовать")
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
		s.bad(w, "не выбрано ни одного файла")
		return
	}
	if err := s.fs.Delete(r.Context(), paths); err != nil {
		s.fail(w, r, err, "не удалить")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("файлы удалены", "user", u.ID, "count", len(paths))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.bad(w, "не разобрать форму загрузки")
		return
	}
	folder := r.FormValue("folder")
	if strings.TrimSpace(folder) == "" {
		s.bad(w, "не указана папка")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.bad(w, "файл не приложен")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		s.fail(w, r, err, "не прочитать файл")
		return
	}
	if len(data) > maxUploadSize {
		s.bad(w, "файл больше допустимых 256 МБ")
		return
	}

	name := sanitizeFileName(header.Filename)
	if name == "" {
		s.bad(w, "недопустимое имя файла")
		return
	}

	if err := s.fs.Upload(r.Context(), folder, name, data, r.FormValue("overwrite") == "true"); err != nil {
		s.fail(w, r, err, "не загрузить файл")
		return
	}
	u, _ := userFrom(r.Context())
	s.log.Info("файл загружен", "user", u.ID, "folder", folder, "name", name, "bytes", len(data))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

// sanitizeFileName оставляет от присланного имени только базовую часть:
// клиент может прислать что угодно, включая путь с "..".
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
		s.bad(w, "не разобрать запрос")
		return false
	}
	return true
}

func (s *Server) bad(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

// fail отвечает на ошибку от NAS.
//
// Сообщения DSM понятны человеку и не содержат секретов, поэтому их видно в
// интерфейсе: «папка не существует» полезнее, чем «внутренняя ошибка».
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error, context string) {
	u, _ := userFrom(r.Context())
	s.log.Error(context, "user", u.ID, "path", r.URL.Path, "err", err)

	status := http.StatusBadGateway
	var apiErr *dsm.APIError
	if errors.As(err, &apiErr) {
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{
		"error":  context,
		"detail": err.Error(),
	})
}
