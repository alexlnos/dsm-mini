// Package httpapi — REST для Mini App и раздача самого приложения.
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

// Server обслуживает Mini App: отдаёт статику и REST поверх NAS.
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

	// Кэши с разным сроком жизни: сведения об устройстве почти не меняются,
	// загрузка — постоянно. Без них каждое обновление экрана превращалось
	// в несколько запросов к NAS и занимало больше двух секунд.
	infoCache     *cache.Cache[system.Info]
	usageCache    *cache.Cache[system.Usage]
	packagesCache *cache.Cache[[]system.Package]
	storageCache  *cache.Cache[storage.Overview]
	tasksCache    *cache.Cache[[]downloadstation.Task]
	vmsCache      *cache.Cache[[]vmm.Guest]
	vmHostCache   *cache.Cache[vmm.Resources]
}

// Options — зависимости сервера.
type Options struct {
	BotToken       string
	AllowedUserIDs []int64
	Downloads      downloadstation.Station
	Files          *filestation.Station
	// Settings хранит пользовательские настройки. Может быть nil: тогда
	// приложение работает с настройками по умолчанию и ничего не сохраняет.
	Settings   *store.Store
	System     *system.Service
	Storage    *storage.Service
	VMs        *vmm.Service
	Containers *containers.Service
	// Static — собранное Mini App. Может быть nil: тогда отдаётся заглушка,
	// что удобно при разработке бэкенда отдельно от фронтенда.
	Static fs.FS
	Logger *slog.Logger
}

// New собирает сервер.
func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &Server{
		// Сроки подобраны по тому, как часто меняются данные: модель и
		// прошивка — почти никогда, загрузка — ежесекундно.
		infoCache:     cache.New[system.Info](10 * time.Minute),
		usageCache:    cache.New[system.Usage](2 * time.Second),
		packagesCache: cache.New[[]system.Package](time.Minute),
		storageCache:  cache.New[storage.Overview](30 * time.Second),
		// Задачи и машины меняются на глазах, поэтому сроки короткие: кэш
		// здесь гасит шквал одинаковых запросов, а не хранит данные.
		tasksCache: cache.New[[]downloadstation.Task](2 * time.Second),
		vmsCache:   cache.New[[]vmm.Guest](5 * time.Second),
		// Свободная память хоста меняется медленно, а запрос за ней долгий.
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

// Handler возвращает маршрутизатор со всеми обработчиками.
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

// handleHealth отвечает без авторизации: по нему следят, жив ли контейнер.
// Ничего о NAS он не сообщает.
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
			_, _ = w.Write([]byte("Mini App не собрано: соберите web/ и пересоберите бинарник\n"))
		})
	}
	files := http.FileServer(http.FS(s.static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Одностраничное приложение: неизвестные пути отдают index.html,
		// чтобы работала навигация внутри Mini App.
		if r.URL.Path != "/" && !fileExists(s.static, r.URL.Path) {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		setCacheHeaders(w, r.URL.Path)
		files.ServeHTTP(w, r)
	})
}

// setCacheHeaders разделяет статику на две части.
//
// Сборщик даёт файлам в assets имена с хешем содержимого, поэтому их можно
// кешировать навсегда: изменится содержимое — изменится и имя. А вот
// index.html ссылается на эти имена, и его нужно перепроверять каждый раз,
// иначе клиент продолжит открывать старую сборку. Telegram кеширует Mini App
// особенно цепко: без этого заголовка обновление доходит до пользователя
// только после перезапуска приложения.
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

// securityHeaders выставляет заголовки, уместные для страницы, живущей внутри
// webview Telegram.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		// Mini App открывается только внутри Telegram, встраивать его
		// куда-либо ещё незачем.
		h.Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	})
}

// recoverMiddleware не даёт панике в обработчике уронить весь сервис.
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("паника в обработчике", "path", r.URL.Path, "panic", v)
				writeJSON(w, http.StatusInternalServerError,
					map[string]string{"error": "внутренняя ошибка"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
