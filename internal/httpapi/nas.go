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

// handleSystem serves everything for the home screen in one request: device,
// load and package states.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Three independent requests to the NAS go at once: one after another they
	// added up to over two seconds of waiting on the home screen.
	var (
		info        system.Info
		infoErr     error
		usage       system.Usage
		usageErr    error
		packages    []system.Package
		packagesErr error
	)

	tasks := []struct {
		what string
		run  func()
	}{
		{"nas details", func() {
			info, infoErr = s.infoCache.GetStale(ctx, s.system.Info)
		}},
		{"nas load", func() {
			usage, usageErr = s.usageCache.GetStale(ctx, s.system.Usage)
		}},
		{"package list", func() {
			packages, packagesErr = s.packagesCache.GetStale(ctx, s.system.Packages)
		}},
	}

	// The count comes from the slice itself: let it drift from the number of
	// started goroutines and the handler would hang forever, as it already did.
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go s.inBackground(&wg, task.what, task.run)
	}
	wg.Wait()

	// Each part stands on its own. With an ordinary DSM account the details
	// and the load are refused — checked by a tester, 1006 and 105 — and
	// failing the whole answer over them showed "no connection" on the home
	// screen of a service that was connected and working; zeros in place of
	// a refused load would have been a plain lie.
	u, _ := userFrom(ctx)
	lang := i18n.Match(u.Language)
	body := map[string]any{"info": nil, "usage": nil}
	if infoErr != nil {
		s.log.Warn("cannot get the nas details", "err", infoErr)
		body["info_error"] = failureText(lang, "api.info", infoErr)
	} else {
		body["info"] = info
	}
	if usageErr != nil {
		s.log.Warn("cannot get the load", "err", usageErr)
		body["usage_error"] = failureText(lang, "api.load", usageErr)
	} else {
		body["usage"] = usage
	}

	// The interface only cares whether a section of the app is available.
	// Without the package list it knows nothing either way, and says nothing:
	// marking Download Station "not installed" because the list was refused
	// would lock people out of the one section that does work for them.
	state := make(map[string]any, 4)
	if packagesErr != nil {
		s.log.Warn("cannot get the package list", "err", packagesErr)
		packages = nil
	}
	for _, id := range []string{"DownloadStation", "FileStation", "Virtualization", "ContainerManager"} {
		if packagesErr != nil {
			break
		}
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

	body["packages"] = state
	writeJSON(w, http.StatusOK, body)
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
	// The summary is built from data already at hand: otherwise the list and
	// the host details would be fetched again and the screen would wait a second.
	res, err := s.vmHostCache.GetStale(ctx, s.vms.Resources)
	if err != nil {
		s.log.Warn("cannot get the virtualisation host details", "err", err)
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

	// A machine does not change state instantly, but the list must be re-read.
	s.vmsCache.Invalidate()

	u, _ := userFrom(ctx)
	s.log.Info("action on a machine", "user", u.ID, "vm", req.ID, "action", req.Action)
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
	s.log.Info("action on a container", "user", u.ID, "name", req.Name, "action", req.Action)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// inBackground runs part of a request's work in a separate goroutine.
//
// Recovering here is mandatory: recover in middleware catches a panic only in
// its own goroutine, while a panic in a neighbouring one takes the whole
// process down — that is how the service already died over one uninitialised cache.
func (s *Server) inBackground(wg *sync.WaitGroup, what string, run func()) {
	defer wg.Done()
	defer func() {
		if v := recover(); v != nil {
			s.log.Error("panic in the background part of a request", "what", what, "panic", v)
		}
	}()
	run()
}

// Warmup fills the NAS state caches in advance.
//
// Without it the first person to open the app waits out a full DSM round
// trip. Warming up in the background turns that wait into reading ready values.
func (s *Server) Warmup(ctx context.Context) {
	if s.system == nil {
		return
	}

	tasks := []struct {
		what string
		run  func()
	}{
		{"nas details", func() { _, _ = s.infoCache.Get(ctx, s.system.Info) }},
		{"nas load", func() { _, _ = s.usageCache.Get(ctx, s.system.Usage) }},
		{"package list", func() { _, _ = s.packagesCache.Get(ctx, s.system.Packages) }},
	}
	if s.ds != nil {
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"tasks", func() { _, _ = s.tasksCache.Get(ctx, s.ds.List) }})
	}
	if s.vms != nil {
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"machines", func() { _, _ = s.vmsCache.Get(ctx, s.vms.List) }})
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"virtualisation resources", func() { _, _ = s.vmHostCache.Get(ctx, s.vms.Resources) }})
	}
	if s.storage != nil {
		tasks = append(tasks, struct {
			what string
			run  func()
		}{"storage", func() { _, _ = s.storageCache.Get(ctx, s.storage.Load) }})
	}

	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go s.inBackground(&wg, task.what, task.run)
	}
	wg.Wait()
}

// KeepWarm refreshes the caches for as long as the context lives.
//
// An interval shorter than the shortest lifetime is pointless: the data
// cannot be read fresher than it changes, and the NAS should not be poked for nothing.
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
