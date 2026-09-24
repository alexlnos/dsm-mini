package dsmui

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/alexlnos/dsm-mini/internal/store"
)

// Caller is the part of the DSM client this package needs.
type Caller interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
}

// Options are what the settings screen needs to work.
type Options struct {
	// StateDir is the package var: config.env and the database live there.
	StateDir string
	// Store holds the installation-wide settings.
	Store *store.Store
	// DSM is the logged-in client, used to read the NAS's own network setup.
	DSM Caller
	// ListenAddr is where the service listens, so the screen can tell which
	// reverse proxy rule, if any, points at it.
	ListenAddr string
	Auth       *Auth
	Logger     *slog.Logger
}

// Server serves the screen's endpoints.
type Server struct {
	stateDir   string
	store      *store.Store
	dsm        Caller
	listenAddr string
	auth       *Auth
	log        *slog.Logger
}

// New builds the server.
func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &Server{
		stateDir:   o.StateDir,
		store:      o.Store,
		dsm:        o.DSM,
		listenAddr: o.ListenAddr,
		auth:       o.Auth,
		log:        o.Logger,
	}
}

// Handler returns the routes, already behind the administrator check.
//
// They sit under /dsm/ rather than /api/ because the caller is a browser
// logged into DSM, not a person in Telegram: there is no initData here, and
// the Telegram check would refuse every one of them.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dsm/admin/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /dsm/admin/settings", s.handleSaveSettings)
	mux.HandleFunc("GET /dsm/admin/address", s.handleAddress)
	mux.HandleFunc("POST /dsm/admin/address/proxy", s.handleCreateProxy)
	mux.HandleFunc("GET /dsm/admin/whoami", s.handleWhoami)
	return s.auth.Middleware(mux)
}

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	who, _ := SessionFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": who.User, "is_admin": who.IsAdmin})
}

func (s *Server) notifyMode() string {
	if s.store == nil {
		return store.NotifyDownloads
	}
	mode, err := s.store.NotifyMode(context.Background())
	if err != nil {
		s.log.Warn("cannot read the notification mode", "err", err)
		return store.NotifyDownloads
	}
	return mode
}

// fail logs the detail and tells the browser only that it did not work: the
// screen is behind an administrator check, but the answer still travels over a
// path DSM serves to anyone.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error, what string) {
	s.log.Error("dsm settings screen", "what", what, "path", r.URL.Path, "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": what})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
