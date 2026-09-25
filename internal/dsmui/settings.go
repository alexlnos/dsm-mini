package dsmui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/config"
)

// Keys of config.env that the screen edits, in the order it asks for them.
// Everything else in the file is left alone: the service reads more variables
// than are worth showing, and a screen that rewrites a file it does not fully
// understand loses what it did not know about.
var editable = []string{
	"DSM_USER", "DSM_PASSWORD",
	"TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "PUBLIC_URL",
	"DSM_URL", "LISTEN_ADDR",
}

// secret marks the values that are never sent back to the browser. The screen
// shows whether they are set and offers an empty field to replace them.
var secret = map[string]bool{"DSM_PASSWORD": true, "TELEGRAM_BOT_TOKEN": true}

// SettingsFile is where the settings live: the package var, which survives an
// upgrade.
func SettingsFile(stateDir string) string {
	return filepath.Join(stateDir, "config.env")
}

type settingsBody struct {
	Values map[string]string `json:"values"`
	// Set lists the secrets that have a value, without saying what it is.
	Set map[string]bool `json:"set"`
	// Defaults are what an empty field stands for, shown as placeholders.
	Defaults map[string]string `json:"defaults"`
	// Notifications is the installation-wide choice. It is kept in the
	// database and applies at once. Empty when the database is out of reach.
	Notifications string `json:"notifications,omitempty"`
	// Applying is true when the save changed something the service reads
	// when it starts: it is starting again with it right now, and the screen
	// waits for the status to settle.
	Applying bool `json:"applying,omitempty"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	values, ok := s.readSettings(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.view(values, false))
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Values        map[string]string `json:"values"`
		Notifications *string           `json:"notifications"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	current, ok := s.readSettings(w, r)
	if !ok {
		return
	}

	changed := false
	for _, key := range editable {
		v, ok := req.Values[key]
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		// An empty secret means "leave it alone": the field is empty on every
		// load, because the value is never sent to the browser in the first
		// place, and treating that as "erase it" would lock the service out of
		// the NAS on the first save of an unrelated setting.
		if v == "" && secret[key] {
			continue
		}
		if code := config.Check(key, v); code != "" {
			// A code, not a sentence: the screen speaks ten languages.
			writeJSON(w, http.StatusBadRequest,
				map[string]string{"error": "invalid", "field": key, "code": code})
			return
		}
		if current[key] != v {
			current[key] = v
			changed = true
		}
	}

	if changed {
		if err := config.WriteFile(SettingsFile(s.stateDir), current); err != nil {
			s.fail(w, r, err, "cannot save the settings file")
			return
		}
		who, _ := SessionFrom(r.Context())
		// Who, never what: the values include a password and a token.
		s.log.Info("settings changed from the dsm screen", "by", who.User)
	}

	if req.Notifications != nil && s.store != nil {
		mode, err := s.store.SetNotifyMode(r.Context(), *req.Notifications)
		if err != nil {
			s.fail(w, r, err, "cannot save the notification mode")
			return
		}
		who, _ := SessionFrom(r.Context())
		s.log.Info("notification mode set from the dsm screen", "mode", mode, "by", who.User)
	}

	writeJSON(w, http.StatusOK, s.view(current, changed))
	if changed {
		// After the answer: the service is about to close this very server,
		// and it waits for requests in flight — this one included — to end.
		s.restart()
	}
}

// readSettings reads config.env for the screen. A file the service cannot
// read is not a failure here: it is one left by an earlier installation that
// ran as another user, the status explains it, and the fields start empty so
// the settings can be entered again — saving replaces the file.
func (s *Server) readSettings(w http.ResponseWriter, r *http.Request) (map[string]string, bool) {
	values, err := config.ReadFile(SettingsFile(s.stateDir))
	if os.IsPermission(err) {
		return map[string]string{}, true
	}
	if err != nil {
		s.fail(w, r, err, "cannot read the settings file")
		return nil, false
	}
	return values, true
}

func (s *Server) view(values map[string]string, applying bool) settingsBody {
	out := settingsBody{
		Values:   map[string]string{},
		Set:      map[string]bool{},
		Defaults: config.Defaults(),
		Applying: applying,
	}
	for _, key := range editable {
		if secret[key] {
			out.Set[key] = values[key] != ""
			continue
		}
		out.Values[key] = values[key]
	}
	out.Notifications = s.notifyMode()
	return out
}
