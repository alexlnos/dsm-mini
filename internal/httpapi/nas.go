package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm/system"
	"github.com/alexlnos/dsm-mini/internal/i18n"
)

// handleSystem отдаёт всё для главного экрана одним запросом: устройство,
// загрузка и состояние пакетов.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Три независимых запроса к NAS идут разом: последовательно они
	// складывались в две с лишним секунды ожидания на главном экране.
	var (
		info     system.Info
		infoErr  error
		usage    system.Usage
		packages []system.Package
	)

	tasks := []struct {
		what string
		run  func()
	}{
		{"сведения о NAS", func() {
			info, infoErr = s.infoCache.GetStale(ctx, s.system.Info)
		}},
		{"загрузка NAS", func() {
			value, err := s.usageCache.GetStale(ctx, s.system.Usage)
			if err != nil {
				s.log.Warn("не получить загрузку", "err", err)
				return
			}
			usage = value
		}},
		{"список пакетов", func() {
			value, err := s.packagesCache.GetStale(ctx, s.system.Packages)
			if err != nil {
				s.log.Warn("не получить список пакетов", "err", err)
				return
			}
			packages = value
		}},
	}

	// Счётчик берётся из самого списка: разойдись он с числом запущенных
	// горутин — и обработчик завис бы навсегда, что уже случалось.
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go s.inBackground(&wg, task.what, task.run)
	}
	wg.Wait()

	if infoErr != nil {
		s.fail(w, r, infoErr, "api.info")
		return
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
		s.fail(w, r, err, "api.log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {
	overview, err := s.storageCache.GetStale(r.Context(), s.storage.Load)
	if err != nil {
		s.fail(w, r, err, "api.storage")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleVMs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	guests, err := s.vmsCache.GetStale(ctx, s.vms.List)
	if err != nil {
		s.fail(w, r, err, "api.vms")
		return
	}
	// Сводка считается по готовым данным: иначе список и сведения о хосте
	// запрашивались бы у NAS заново, и экран снова ждал бы секунду.
	res, err := s.vmHostCache.GetStale(ctx, s.vms.Resources)
	if err != nil {
		s.log.Warn("не получить сведения о хосте виртуализации", "err", err)
	}
	host := s.vms.HostFor(guests, res)
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
		s.bad(w, r, "bad.badAction", i18n.P{"value": shorten(req.Action)})
		return
	}
	if err != nil {
		s.fail(w, r, err, "api.vmAction")
		return
	}

	// Машина меняет состояние не мгновенно, но список надо перечитать.
	s.vmsCache.Invalidate()

	u, _ := userFrom(ctx)
	s.log.Info("действие над машиной", "user", u.ID, "vm", req.ID, "action", req.Action)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	list, err := s.containers.List(r.Context())
	if err != nil {
		s.fail(w, r, err, "api.containers")
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
		s.bad(w, r, "bad.badAction", i18n.P{"value": shorten(req.Action)})
		return
	}
	if err != nil {
		s.fail(w, r, err, "api.containerAction")
		return
	}

	u, _ := userFrom(ctx)
	s.log.Info("действие над контейнером", "user", u.ID, "name", req.Name, "action", req.Action)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// inBackground запускает часть работы запроса в отдельной горутине.
//
// Восстановление здесь обязательно: recover в middleware ловит панику только
// в своей горутине, а паника в соседней роняет процесс целиком — так уже
// падал весь сервис из-за одного неинициализированного кэша.
func (s *Server) inBackground(wg *sync.WaitGroup, what string, run func()) {
	defer wg.Done()
	defer func() {
		if v := recover(); v != nil {
			s.log.Error("паника в фоновой части запроса", "что", what, "panic", v)
		}
	}()
	run()
}

// Warmup заранее наполняет кэши состояния NAS.
//
// Без него первый, кто откроет приложение, ждёт полный обход DSM. Прогрев в
// фоне превращает это ожидание в чтение готовых значений.
func (s *Server) Warmup(ctx context.Context) {
	if s.system == nil {
		return
	}

	tasks := []struct {
		what string
		run  func()
	}{
		{"сведения о NAS", func() { _, _ = s.infoCache.Get(ctx, s.system.Info) }},
		{"загрузка NAS", func() { _, _ = s.usageCache.Get(ctx, s.system.Usage) }},
		{"список пакетов", func() { _, _ = s.packagesCache.Get(ctx, s.system.Packages) }},
	}
	if s.ds != nil {
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"задачи", func() { _, _ = s.tasksCache.Get(ctx, s.ds.List) }})
	}
	if s.vms != nil {
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"машины", func() { _, _ = s.vmsCache.Get(ctx, s.vms.List) }})
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"ресурсы виртуализации", func() { _, _ = s.vmHostCache.Get(ctx, s.vms.Resources) }})
	}
	if s.storage != nil {
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"хранилище", func() { _, _ = s.storageCache.Get(ctx, s.storage.Load) }})
	}

	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go s.inBackground(&wg, task.what, task.run)
	}
	wg.Wait()
}

// KeepWarm обновляет кэши, пока жив контекст.
//
// Интервал меньше самого короткого срока жизни не нужен: чаще, чем меняются
// данные, их всё равно не прочитать, а NAS не стоит дёргать зря.
func (s *Server) KeepWarm(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 20 * time.Second
	}
	s.Warmup(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Warmup(ctx)
		}
	}
}
