// Package httpapi — REST для Mini App и раздача самого приложения.
package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/dsm/filestation"
	"github.com/alexlnos/dsm-mini/internal/store"
)

// Server обслуживает Mini App: отдаёт статику и REST поверх NAS.
type Server struct {
	botToken string
	allowed  []int64
	ds       downloadstation.Station
	fs       *filestation.Station
	settings *store.Store
	static   fs.FS
	log      *slog.Logger
}

// Options — зависимости сервера.
type Options struct {
	BotToken       string
	AllowedUserIDs []int64
	Downloads      downloadstation.Station
	Files          *filestation.Station
	// Settings хранит пользовательские настройки. Может быть nil: тогда
	// приложение работает с настройками по умолчанию и ничего не сохраняет.
	Settings *store.Store
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
		botToken: o.BotToken,
		allowed:  o.AllowedUserIDs,
		ds:       o.Downloads,
		fs:       o.Files,
		settings: o.Settings,
		static:   o.Static,
		log:      o.Logger,
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
	api.HandleFunc("GET /api/files", s.handleListFiles)
	api.HandleFunc("POST /api/files/folder", s.handleCreateFolder)
	api.HandleFunc("POST /api/files/rename", s.handleRename)
	api.HandleFunc("POST /api/files/delete", s.handleDeleteFiles)
	api.HandleFunc("POST /api/files/upload", s.handleUpload)
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
		files.ServeHTTP(w, r)
	})
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
