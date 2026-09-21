// Package httpapi is the REST layer for the Mini App and serves the app itself.
package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alexlnos/dsm-mini/internal/cache"
	"github.com/alexlnos/dsm-mini/internal/dsm/containers"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/dsm/filestation"
	"github.com/alexlnos/dsm-mini/internal/dsm/storage"
	"github.com/alexlnos/dsm-mini/internal/dsm/system"
	"github.com/alexlnos/dsm-mini/internal/dsm/vmm"
	"github.com/alexlnos/dsm-mini/internal/store"
)

// Server serves the Mini App: static files and REST on top of the NAS.
type Server struct {
	botToken   string
	allowed    []int64
	ds         downloadstation.Station
	fs         *filestation.Station
	system     *system.Service
	storage    *storage.Service
	vms        *vmm.Service
	containers *containers.Service
	settings   *store.Store
	static     fs.FS
	log        *slog.Logger

	// Caches with different lifetimes: device details barely change, the load
	// changes constantly. Without them every screen refresh turned into
	// several requests to the NAS and took over two seconds.
	infoCache     *cache.Cache[system.Info]
	usageCache    *cache.Cache[system.Usage]
	packagesCache *cache.Cache[[]system.Package]
	storageCache  *cache.Cache[storage.Overview]
	tasksCache    *cache.Cache[[]downloadstation.Task]
	vmsCache      *cache.Cache[[]vmm.Guest]
	vmHostCache   *cache.Cache[vmm.Resources]
}

// Options are the server dependencies.
type Options struct {
	BotToken       string
	AllowedUserIDs []int64
	Downloads      downloadstation.Station
	Files          *filestation.Station
	// Settings stores user settings. May be nil: then the app works with the
	// defaults and saves nothing.
	Settings   *store.Store
	System     *system.Service
	Storage    *storage.Service
	VMs        *vmm.Service
	Containers *containers.Service
	// Static is the built Mini App. May be nil: then a stub is served, which
	// is handy when developing the backend apart from the frontend.
	Static fs.FS
	Logger *slog.Logger
}

// New assembles the server.
func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &Server{
		// The lifetimes follow how often the data changes: model and firmware
		// almost never, the load every second.
		infoCache:     cache.New[system.Info](10 * time.Minute),
		usageCache:    cache.New[system.Usage](2 * time.Second),
		packagesCache: cache.New[[]system.Package](time.Minute),
		storageCache:  cache.New[storage.Overview](30 * time.Second),
		// Tasks and machines change before your eyes, so the lifetimes are
		// short: the cache here damps a burst of identical requests, not stores data.
		tasksCache: cache.New[[]downloadstation.Task](2 * time.Second),
		vmsCache:   cache.New[[]vmm.Guest](5 * time.Second),
		// The host free memory changes slowly, while the request for it is slow.
		vmHostCache: cache.New[vmm.Resources](15 * time.Second),

		botToken:   o.BotToken,
		allowed:    o.AllowedUserIDs,
		ds:         o.Downloads,
		fs:         o.Files,
		system:     o.System,
		storage:    o.Storage,
		vms:        o.VMs,
		containers: o.Containers,
		settings:   o.Settings,
		static:     o.Static,
		log:        o.Logger,
	}
}

func (s *Server) isAllowed(id int64) bool {
	for _, a := range s.allowed {
		if a == id {
			return true
		}
	}
	return false
}

// Handler returns the router with every handler wired up.
func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/overview", s.handleOverview)
	api.HandleFunc("GET /api/downloads", s.handleListTasks)
	api.HandleFunc("POST /api/downloads", s.handleCreateTask)
	api.HandleFunc("POST /api/downloads/action", s.handleTaskAction)
	api.HandleFunc("GET /api/downloads/files", s.handleTaskFiles)
	api.HandleFunc("POST /api/downloads/files", s.handleSetFile)
	api.HandleFunc("POST /api/downloads/destination", s.handleSetDestination)
	api.HandleFunc("POST /api/downloads/priority", s.handleSetPriority)
	api.HandleFunc("GET /api/files", s.handleListFiles)
	api.HandleFunc("POST /api/files/folder", s.handleCreateFolder)
	api.HandleFunc("POST /api/files/rename", s.handleRename)
	api.HandleFunc("POST /api/files/delete", s.handleDeleteFiles)
	api.HandleFunc("POST /api/files/upload", s.handleUpload)
	api.HandleFunc("POST /api/files/copy", s.handleCopy)
	api.HandleFunc("POST /api/files/move", s.handleMove)
	api.HandleFunc("GET /api/files/transfer", s.handleTransferStatus)
	api.HandleFunc("POST /api/files/transfer/stop", s.handleTransferStop)
	api.HandleFunc("GET /api/files/thumb", s.handleThumb)
	api.HandleFunc("GET /api/files/preview", s.handlePreview)
	api.HandleFunc("GET /api/system", s.handleSystem)
	api.HandleFunc("GET /api/system/log", s.handleSystemLog)
	api.HandleFunc("GET /api/storage", s.handleStorage)
	api.HandleFunc("GET /api/vms", s.handleVMs)
	api.HandleFunc("POST /api/vms/action", s.handleVMAction)
	api.HandleFunc("GET /api/containers", s.handleContainers)
	api.HandleFunc("POST /api/containers/action", s.handleContainerAction)
	api.HandleFunc("GET /api/settings", s.handleGetSettings)
	api.HandleFunc("PUT /api/settings", s.handleSaveSettings)

	root := http.NewServeMux()
	root.Handle("/api/", s.authMiddleware(api))
	root.HandleFunc("GET /healthz", s.handleHealth)
	root.Handle("/", s.staticHandler())

	return s.recoverMiddleware(securityHeaders(root))
}

// handleHealth answers without authorisation: it is how the container is
// watched. It tells nothing about the NAS.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) staticHandler() http.Handler {
	if s.static == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("the Mini App is not built: build web/ and rebuild the binary\n"))
		})
	}
	files := http.FileServer(http.FS(s.static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A single-page app: unknown paths serve index.html so that navigation
		// inside the Mini App works.
		if r.URL.Path != "/" && !fileExists(s.static, r.URL.Path) {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		setCacheHeaders(w, r.URL.Path)
		files.ServeHTTP(w, r)
	})
}

// setCacheHeaders splits the static files in two.
//
// The bundler gives files in assets names carrying a content hash, so they
// can be cached forever: change the content and the name changes too. But
// index.html refers to those names and has to be revalidated every time,
// otherwise the client keeps opening the old build. Telegram caches a Mini
// App especially stubbornly: without this header an update reaches the user
// only after the app is restarted.
func setCacheHeaders(w http.ResponseWriter, path string) {
	if strings.HasPrefix(path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

func fileExists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(trimLeadingSlash(name))
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}

// securityHeaders sets headers suited to a page living inside
// webview Telegram.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		// The Mini App is opened only inside Telegram, there is no reason to
		// embed it anywhere else.
		h.Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware keeps a panic in a handler from taking the service down.
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic in a handler", "path", r.URL.Path, "panic", v)
				writeJSON(w, http.StatusInternalServerError,
					map[string]string{"error": "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
