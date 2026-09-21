package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

// handleSystem отдаёт всё для главного экрана одним запросом: устройство,
// загрузка и состояние пакетов.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	info, err := s.system.Info(ctx)
	if err != nil {
		s.fail(w, r, err, "не получить сведения о NAS")
		return
	}

	// Загрузка и пакеты не критичны: экран полезен и без них.
	usage, err := s.system.Usage(ctx)
	if err != nil {
		s.log.Warn("не получить загрузку", "err", err)
	}
	packages, err := s.system.Packages(ctx)
	if err != nil {
		s.log.Warn("не получить список пакетов", "err", err)
	}

	// Интерфейсу важно только то, доступен ли раздел приложения.
	state := make(map[string]any, 4)
	for _, id := range []string{"DownloadStation", "FileStation", "Virtualization", "ContainerManager"} {
		found := false
		for _, p := range packages {
			if strings.EqualFold(p.ID, id) {
				state[id] = map[string]any{
					"installed": true, "running": p.Running,
					"version": p.Version, "name": p.Name,
				}
				found = true
				break
			}
		}
		if !found {
			state[id] = map[string]any{"installed": false, "running": false}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"info":     info,
		"usage":    usage,
		"packages": state,
	})
}

func (s *Server) handleSystemLog(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	onlyProblems := r.URL.Query().Get("problems") == "true"

	entries, err := s.system.Log(r.Context(), limit, onlyProblems)
	if err != nil {
		s.fail(w, r, err, "не получить журнал")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {
	overview, err := s.storage.Load(r.Context())
	if err != nil {
		s.fail(w, r, err, "не получить состояние хранилища")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleVMs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	guests, err := s.vms.List(ctx)
	if err != nil {
		s.fail(w, r, err, "не получить список машин")
		return
	}
	host, err := s.vms.Host(ctx)
	if err != nil {
		s.log.Warn("не получить сводку виртуализации", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"guests": guests, "host": host})
}

type vmActionRequest struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

func (s *Server) handleVMAction(w http.ResponseWriter, r *http.Request) {
	var req vmActionRequest
	if !decode(w, r, &req, s) {
		return
	}

	ctx := r.Context()
	var err error
	switch req.Action {
	case "start":
		err = s.vms.PowerOn(ctx, req.ID)
	case "shutdown":
		err = s.vms.Shutdown(ctx, req.ID)
	default:
		s.bad(w, "неизвестное действие: "+shorten(req.Action))
		return
	}
	if err != nil {
		s.fail(w, r, err, "не выполнить действие над машиной")
		return
	}

	u, _ := userFrom(ctx)
	s.log.Info("действие над машиной", "user", u.ID, "vm", req.ID, "action", req.Action)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	list, err := s.containers.List(r.Context())
	if err != nil {
		s.fail(w, r, err, "не получить список контейнеров")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": list})
}

type containerActionRequest struct {
	Name   string `json:"name"`
	Action string `json:"action"`
}

func (s *Server) handleContainerAction(w http.ResponseWriter, r *http.Request) {
	var req containerActionRequest
	if !decode(w, r, &req, s) {
		return
	}

	ctx := r.Context()
	var err error
	switch req.Action {
	case "start":
		err = s.containers.Start(ctx, req.Name)
	case "stop":
		err = s.containers.Stop(ctx, req.Name)
	case "restart":
		err = s.containers.Restart(ctx, req.Name)
	default:
		s.bad(w, "неизвестное действие: "+shorten(req.Action))
		return
	}
	if err != nil {
		s.fail(w, r, err, "не выполнить действие над контейнером")
		return
	}

	u, _ := userFrom(ctx)
	s.log.Info("действие над контейнером", "user", u.ID, "name", req.Name, "action", req.Action)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
