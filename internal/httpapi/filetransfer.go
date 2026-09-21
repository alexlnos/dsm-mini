package httpapi

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/dsm/filestation"
)

// maxPreviewSize ограничивает предпросмотр: через мессенджер смотрят заметки
// и картинки, а не гигабайтные архивы.
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
		s.bad(w, "не выбрано ни одного файла")
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
		verb := "скопировать"
		if move {
			verb = "перенести"
		}
		s.fail(w, r, err, "не "+verb)
		return
	}

	u, _ := userFrom(r.Context())
	s.log.Info("перенос файлов начат", "user", u.ID, "move", move,
		"count", len(paths), "dest", req.Destination)
	writeJSON(w, http.StatusOK, map[string]any{"task_id": taskID})
}

func (s *Server) handleTransferStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		s.bad(w, "не указана задача")
		return
	}
	status, err := s.fs.Status(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "не получить ход операции")
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
		s.fail(w, r, err, "не остановить операцию")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleThumb отдаёт миниатюру изображения.
//
// Отсутствие миниатюры — обычное дело: у текста и архивов её нет. Отвечаем
// 404, чтобы интерфейс просто не показывал картинку.
func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.bad(w, "не указан файл")
		return
	}
	size := filestation.ThumbSize(r.URL.Query().Get("size"))

	content, err := s.fs.Thumbnail(r.Context(), path, size)
	if err != nil {
		s.log.Debug("миниатюра недоступна", "path", path, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "предпросмотр недоступен"})
		return
	}
	defer content.Body.Close()

	w.Header().Set("Content-Type", content.ContentType)
	// Миниатюра неизменна, пока файл на месте, но в приложении её всё равно
	// перезапрашивают редко — держим недолго.
	w.Header().Set("Cache-Control", "private, max-age=600")
	if _, err := io.Copy(w, io.LimitReader(content.Body, maxPreviewSize)); err != nil {
		s.log.Debug("миниатюра не доотдалась", "err", err)
	}
}

// handlePreview отдаёт содержимое файла для просмотра в приложении.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.bad(w, "не указан файл")
		return
	}

	content, err := s.fs.Download(r.Context(), path)
	if err != nil {
		s.fail(w, r, err, "не открыть файл")
		return
	}
	defer content.Body.Close()

	if content.Length > maxPreviewSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error": "файл слишком большой для просмотра",
			"size":  content.Length,
		})
		return
	}

	w.Header().Set("Content-Type", content.ContentType)
	if content.Length >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(content.Length, 10))
	}
	// Содержимое чужих файлов не должно исполняться как страница.
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, max-age=60")

	if _, err := io.Copy(w, io.LimitReader(content.Body, maxPreviewSize+1)); err != nil {
		s.log.Debug("файл не доотдался", "err", err)
	}
}
