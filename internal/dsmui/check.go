package dsmui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/dsm"
	"github.com/alexlnos/dsm-mini/internal/dsm/containers"
	"github.com/alexlnos/dsm-mini/internal/dsm/downloadstation"
	"github.com/alexlnos/dsm-mini/internal/dsm/filestation"
	"github.com/alexlnos/dsm-mini/internal/dsm/storage"
	"github.com/alexlnos/dsm-mini/internal/dsm/system"
	"github.com/alexlnos/dsm-mini/internal/dsm/vmm"
)

// The account check: the window signs in as the account typed into it and
// goes through everything the service would do, read-only, so that whoever
// sets the package up sees what the account lacks before the bot finds out.
// A SynoCommunity tester learnt it from the log instead: their ordinary account
// was refused the NAS details, the load and the storage.

// CheckItem is one thing the service needs from DSM, and how it went.
type CheckItem struct {
	// ID names the part: signin, admin, downloads, files, shares, details,
	// load, storage, vms, containers, log, notify.
	ID string `json:"id"`
	// State is "ok", "no" (DSM refused), "absent" (the package or its API is
	// not there), "error" (could not tell), or for admin "yes"/"no".
	State string `json:"state"`
	// Code is DSM's own code for a refusal.
	Code int `json:"code,omitempty"`
	// Count is how many shared folders the account sees.
	Count int `json:"count,omitempty"`
}

func (s *Server) handleCheckAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	// The DSM address the service itself would use, and the saved password
	// when the field is empty — the window never has it to send.
	cfg, _, _ := config.Load(SettingsFile(s.stateDir))
	user := strings.TrimSpace(req.User)
	password := req.Password
	if user == "" {
		user = cfg.DSMUser
	}
	if password == "" && user == cfg.DSMUser {
		password = cfg.DSMPassword
	}
	if user == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid", "code": config.Missing})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	// The client signs in exactly as the service's does, session name and
	// all: under a name of its own, an ordinary account was refused here
	// although the service signed in with it. Its sign-out closes its own
	// session by its SID, and should DSM end the service's session anyway,
	// the service signs in again by itself.
	client := dsm.New(dsm.Options{
		BaseURL:     cfg.DSMURL,
		User:        user,
		Password:    password,
		InsecureTLS: cfg.DSMInsecure,
		Timeout:     20 * time.Second,
		Logger:      s.log,
	})
	items := checkAccount(ctx, client)

	who, _ := SessionFrom(r.Context())
	// The account and the outcome, never the password.
	s.log.Info("dsm account checked from the dsm screen", "account", user, "by", who.User, "result", summary(items))
	writeJSON(w, http.StatusOK, map[string]any{"account": user, "items": items})
}

// checkAccount signs in and tries every part in turn. Nothing it calls
// changes anything on the NAS.
func checkAccount(ctx context.Context, c *dsm.Client) []CheckItem {
	if err := c.Login(ctx); err != nil {
		var refused *dsm.AuthError
		if errors.As(err, &refused) {
			return []CheckItem{{ID: "signin", State: "no", Code: refused.Code}}
		}
		return []CheckItem{{ID: "signin", State: "error"}}
	}
	defer func() {
		logout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Logout(logout)
	}()
	items := []CheckItem{{ID: "signin", State: "ok"}}

	// Whether it is an administrator — the call every DSM desktop makes, and
	// the one the window itself asks about the person opening it.
	var initdata struct {
		Session struct {
			IsAdmin bool `json:"is_admin"`
		} `json:"Session"`
	}
	if err := c.Call(ctx, "SYNO.Core.Desktop.Initdata", "get", 1, nil, &initdata); err != nil {
		items = append(items, CheckItem{ID: "admin", State: "error"})
	} else if initdata.Session.IsAdmin {
		items = append(items, CheckItem{ID: "admin", State: "yes"})
	} else {
		items = append(items, CheckItem{ID: "admin", State: "no"})
	}

	// Download Station: the station is only built when its API is there at
	// all, so a failure to build is "not installed", and a refusal comes
	// from the list.
	if ds, err := downloadstation.New(ctx, c); err != nil {
		items = append(items, CheckItem{ID: "downloads", State: "absent"})
	} else {
		_, err := ds.List(ctx)
		items = append(items, judge("downloads", err))
	}

	shares, err := filestation.New(c).Shares(ctx)
	items = append(items, judge("files", err))
	if err == nil {
		state := "ok"
		if len(shares) == 0 {
			state = "no"
		}
		items = append(items, CheckItem{ID: "shares", State: state, Count: len(shares)})
	}

	sys := system.New(c)
	_, err = sys.Info(ctx)
	items = append(items, judge("details", err))
	_, err = sys.Usage(ctx)
	items = append(items, judge("load", err))
	_, err = storage.New(c).Load(ctx)
	items = append(items, judge("storage", err))
	_, err = vmm.New(c).List(ctx)
	items = append(items, judge("vms", err))
	_, err = containers.New(c).List(ctx)
	items = append(items, judge("containers", err))
	_, err = sys.Log(ctx, 1, false)
	items = append(items, judge("log", err))

	// The notification webhook is registered by the service under this
	// account; reading the list is what the registration starts with.
	var providers any
	err = c.Call(ctx, "SYNO.Core.Notification.Push.Webhook.Provider", "list", 2, nil, &providers)
	items = append(items, judge("notify", err))
	return items
}

// judge turns the outcome of one call into an item.
func judge(id string, err error) CheckItem {
	if err == nil {
		return CheckItem{ID: id, State: "ok"}
	}
	if errors.Is(err, dsm.ErrNoAPI) {
		return CheckItem{ID: id, State: "absent"}
	}
	var refused *dsm.APIError
	if errors.As(err, &refused) {
		return CheckItem{ID: id, State: "no", Code: refused.Code}
	}
	return CheckItem{ID: id, State: "error"}
}

// summary is the one line the log gets: what was not ok.
func summary(items []CheckItem) string {
	var parts []string
	for _, it := range items {
		if it.State != "ok" && it.State != "yes" {
			parts = append(parts, it.ID+"="+it.State)
		}
	}
	if len(parts) == 0 {
		return "all ok"
	}
	return strings.Join(parts, " ")
}
