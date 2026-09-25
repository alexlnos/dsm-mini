package dsmui

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm"
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
	// Store holds the installation-wide settings. Nil when the database
	// could not be opened: the screen still serves everything else, because
	// it is where the reason is shown.
	Store *store.Store
	// DSMURL and InsecureTLS are where DSM is. The screen calls it as the
	// administrator who opened it, never as the service's own account, so it
	// works before that account is even set up — and the account is not an
	// administrator and could not create a reverse proxy rule anyway.
	DSMURL      string
	InsecureTLS bool
	// ListenAddr is where the service listens, so the screen can tell which
	// reverse proxy rule, if any, points at it.
	ListenAddr string
	// Status is what the service is doing, shown at the top of the screen.
	Status *Status
	// Restart applies saved settings: the service starts its insides again
	// with them, within the same process. Called after the answer is written.
	Restart func()
	Auth    *Auth
	Logger  *slog.Logger
}

// Server serves the screen's endpoints.
type Server struct {
	stateDir   string
	store      *store.Store
	dsmURL     string
	insecure   bool
	listenAddr string
	status     *Status
	restart    func()
	auth       *Auth
	log        *slog.Logger

	// session builds the DSM client a request works through. A field so the
	// tests can hand in a fake NAS.
	session func(r *http.Request) Caller
}

// New builds the server.
func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Status == nil {
		o.Status = NewStatus()
	}
	if o.Restart == nil {
		o.Restart = func() {}
	}
	s := &Server{
		stateDir:   o.StateDir,
		store:      o.Store,
		dsmURL:     o.DSMURL,
		insecure:   o.InsecureTLS,
		listenAddr: o.ListenAddr,
		status:     o.Status,
		restart:    o.Restart,
		auth:       o.Auth,
		log:        o.Logger,
	}
	s.session = s.borrowSession
	return s
}

// Handler returns the routes, already behind the administrator check.
//
// They sit under /dsm/ rather than /api/ because the caller is a browser
// logged into DSM, not a person in Telegram: there is no initData here, and
// the Telegram check would refuse every one of them.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dsm/admin/status", s.handleStatus)
	mux.HandleFunc("GET /dsm/admin/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /dsm/admin/settings", s.handleSaveSettings)
	mux.HandleFunc("GET /dsm/admin/address", s.handleAddress)
	mux.HandleFunc("POST /dsm/admin/address/proxy", s.handleCreateProxy)
	mux.HandleFunc("GET /dsm/admin/whoami", s.handleWhoami)
	return s.auth.Middleware(mux)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.status.View())
}

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	who, _ := SessionFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": who.User, "is_admin": who.IsAdmin})
}

// borrowSession is a DSM client working inside the browser session of the
// administrator who opened the screen: the cookie the browser sent and DSM's
// CSRF token the page read from the desktop around it. The Auth middleware
// has already checked with DSM that the session is an administrator's.
//
// DSM accepts such a session over loopback although it was made from another
// machine, with its IP checking on (skip_ip_checking false) — checked on DSM
// 7.4.
func (s *Server) borrowSession(r *http.Request) Caller {
	return dsm.New(dsm.Options{
		BaseURL:     s.dsmURL,
		InsecureTLS: s.insecure,
		Cookie:      dsmCookie(r),
		SynoToken:   r.Header.Get(TokenHeader),
		Timeout:     20 * time.Second,
		Logger:      s.log,
	})
}

func (s *Server) notifyMode() string {
	if s.store == nil {
		return ""
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
