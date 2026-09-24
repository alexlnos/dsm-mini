package dsmui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Keys of config.env that the screen edits. Everything else in the file is
// left alone: the service reads more variables than are worth showing, and a
// screen that rewrites a file it does not fully understand loses what it did
// not know about.
var editable = []string{
	"DSM_URL", "DSM_USER", "DSM_PASSWORD",
	"TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "PUBLIC_URL", "LISTEN_ADDR",
}

// secret marks the values that are never sent back to the browser. The screen
// shows whether they are set and offers an empty field to replace them.
var secret = map[string]bool{"DSM_PASSWORD": true, "TELEGRAM_BOT_TOKEN": true}

// settingsFile is where the installer wrote the answers and where the service
// reads them from. It sits in the package var, which survives an upgrade.
func settingsFile(stateDir string) string {
	return filepath.Join(stateDir, "config.env")
}

type settingsBody struct {
	Values map[string]string `json:"values"`
	// Set lists the secrets that have a value, without saying what it is.
	Set map[string]bool `json:"set"`
	// Notifications is the installation-wide choice; it applies at once,
	// unlike everything else here.
	Notifications string `json:"notifications"`
	// RestartNeeded is true when something was saved that the running service
	// read at startup and will not notice on its own.
	RestartNeeded bool `json:"restart_needed,omitempty"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	values, err := readEnv(settingsFile(s.stateDir))
	if err != nil {
		s.fail(w, r, err, "cannot read the settings file")
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

	current, err := readEnv(settingsFile(s.stateDir))
	if err != nil {
		s.fail(w, r, err, "cannot read the settings file")
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
		if problem := validate(key, v); problem != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": problem, "field": key})
			return
		}
		if current[key] != v {
			current[key] = v
			changed = true
		}
	}

	if changed {
		if err := writeEnv(settingsFile(s.stateDir), current); err != nil {
			s.fail(w, r, err, "cannot save the settings file")
			return
		}
		who, _ := SessionFrom(r.Context())
		// The keys are logged, never the values.
		s.log.Info("settings changed from the dsm screen", "by", who.User)
	}

	if req.Notifications != nil {
		if _, err := s.store.SetNotifyMode(r.Context(), *req.Notifications); err != nil {
			s.fail(w, r, err, "cannot save the notification mode")
			return
		}
	}

	writeJSON(w, http.StatusOK, s.view(current, changed))
}

func (s *Server) view(values map[string]string, restart bool) settingsBody {
	out := settingsBody{
		Values:        map[string]string{},
		Set:           map[string]bool{},
		RestartNeeded: restart,
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

// validate keeps an obviously broken value out of the file. It is deliberately
// shallow: whether a token works is not something a regular expression knows,
// and the screen says so after a restart instead of guessing here.
func validate(key, v string) string {
	switch key {
	case "DSM_URL", "PUBLIC_URL":
		if v != "" && !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			return "the address has to start with http:// or https://"
		}
	case "ALLOWED_USER_IDS":
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, err := strconv.ParseInt(part, 10, 64); err != nil {
				return "the list of ids may only hold numbers separated by commas"
			}
		}
	case "LISTEN_ADDR":
		_, port, found := strings.Cut(v, ":")
		if !found {
			return "the address has to look like 127.0.0.1:8080"
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "the port has to be a number from 1 to 65535"
		}
	}
	return ""
}

// envLine matches KEY='value' as postinst writes it, and KEY=value as a person
// editing the file by hand is likely to.
var envLine = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(.*)$`)

func readEnv(path string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		// Not an error: an installation that never ran the wizard has nothing
		// to show, and the screen is how it gets filled in.
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := envLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out[m[1]] = unquote(m[2])
	}
	return out, scan.Err()
}

func unquote(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], `'\''`, `'`)
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// writeEnv rewrites the file through a temporary one: a half-written
// config.env is a service that will not start after the next restart.
//
// Values go in single quotes, the same way postinst writes them — the file is
// read with `.` by the start script, so a password with a space or a dollar
// sign would otherwise break the start.
func writeEnv(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s='%s'\n", k, strings.ReplaceAll(values[k], `'`, `'\''`))
	}

	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
