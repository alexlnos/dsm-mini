package dsmui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexlnos/dsm-mini/internal/config"
	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// checkNAS is a DSM that treats the signed-in account the way a real one
// treated a tester's ordinary account: Download Station and File Station
// answer, the NAS details come back with 1006 and the rest of the overview
// with 105. Virtual Machine Manager is not installed at all.
type checkNAS struct {
	password string
	admin    bool
	shares   int
	refuse   map[string]int
	absent   map[string]bool
}

func (n *checkNAS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	api, method := r.Form.Get("api"), r.Form.Get("method")
	w.Header().Set("Content-Type", "application/json")
	answer := func(v any) { _ = json.NewEncoder(w).Encode(v) }

	switch api {
	case "SYNO.API.Auth":
		if method == "login" && r.Form.Get("passwd") != n.password {
			answer(map[string]any{"success": false, "error": map[string]any{"code": 400}})
			return
		}
		answer(map[string]any{"success": true, "data": map[string]any{"sid": "s1d"}})
	case "SYNO.API.Info":
		q := r.Form.Get("query")
		if n.absent[q] {
			answer(map[string]any{"success": true, "data": map[string]any{}})
			return
		}
		answer(map[string]any{"success": true, "data": map[string]any{
			q: map[string]any{"path": "entry.cgi", "minVersion": 1, "maxVersion": 2, "requestFormat": "JSON"},
		}})
	default:
		if code, ok := n.refuse[api]; ok {
			answer(map[string]any{"success": false, "error": map[string]any{"code": code}})
			return
		}
		switch api {
		case "SYNO.Core.Desktop.Initdata":
			answer(map[string]any{"success": true, "data": map[string]any{"Session": map[string]any{"is_admin": n.admin}}})
		case "SYNO.FileStation.List":
			shares := make([]map[string]any, n.shares)
			for i := range shares {
				shares[i] = map[string]any{"name": "Download", "path": "/Download", "isdir": true}
			}
			answer(map[string]any{"success": true, "data": map[string]any{"shares": shares}})
		default:
			answer(map[string]any{"success": true, "data": map[string]any{}})
		}
	}
}

func runCheck(t *testing.T, nas *checkNAS, password string) map[string]CheckItem {
	t.Helper()
	srv := httptest.NewServer(nas)
	t.Cleanup(srv.Close)
	c := dsm.New(dsm.Options{BaseURL: srv.URL, User: "dsm-mini", Password: password,
		Timeout: 5 * time.Second, Session: checkSession, Logger: quietLog()})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := map[string]CheckItem{}
	for _, it := range checkAccount(ctx, c) {
		out[it.ID] = it
	}
	return out
}

func TestCheckAnOrdinaryAccount(t *testing.T) {
	nas := &checkNAS{
		password: "p", shares: 2,
		refuse: map[string]int{
			"SYNO.Core.System":                             1006,
			"SYNO.Core.System.Utilization":                 105,
			"SYNO.Storage.CGI.Storage":                     105,
			"SYNO.Docker.Container":                        105,
			"SYNO.Core.SyslogClient.Log":                   105,
			"SYNO.Core.Notification.Push.Webhook.Provider": 105,
		},
		absent: map[string]bool{
			"SYNO.Virtualization.API.Guest": true, "SYNO.Virtualization.API.Host": true,
		},
	}
	got := runCheck(t, nas, "p")

	want := map[string]CheckItem{
		"signin":     {ID: "signin", State: "ok"},
		"admin":      {ID: "admin", State: "no"},
		"downloads":  {ID: "downloads", State: "ok"},
		"files":      {ID: "files", State: "ok"},
		"shares":     {ID: "shares", State: "ok", Count: 2},
		"details":    {ID: "details", State: "no", Code: 1006},
		"load":       {ID: "load", State: "no", Code: 105},
		"storage":    {ID: "storage", State: "no", Code: 105},
		"vms":        {ID: "vms", State: "absent"},
		"containers": {ID: "containers", State: "no", Code: 105},
		"log":        {ID: "log", State: "no", Code: 105},
		"notify":     {ID: "notify", State: "no", Code: 105},
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: got %+v, want %+v", id, got[id], w)
		}
	}
}

// A wrong password stops the check at the sign-in, with DSM's code.
func TestCheckStopsAtAWrongPassword(t *testing.T) {
	got := runCheck(t, &checkNAS{password: "right"}, "wrong")
	if len(got) != 1 || got["signin"] != (CheckItem{ID: "signin", State: "no", Code: 400}) {
		t.Fatalf("got %+v", got)
	}
}

// An account that sees no shared folder cannot save a single download.
func TestCheckNoticesNoSharedFolder(t *testing.T) {
	got := runCheck(t, &checkNAS{password: "p", shares: 0}, "p")
	if got["files"].State != "ok" || got["shares"] != (CheckItem{ID: "shares", State: "no"}) {
		t.Fatalf("files %+v, shares %+v", got["files"], got["shares"])
	}
}

// The endpoint takes the saved password when the field is left empty, and
// never needs one typed for the account the service already uses.
func TestCheckUsesTheSavedPassword(t *testing.T) {
	nas := &checkNAS{password: "saved", shares: 1}
	srv := httptest.NewServer(nas)
	defer srv.Close()

	sc := newScreen(t)
	if err := config.WriteFile(SettingsFile(sc.dir), map[string]string{
		"DSM_URL": srv.URL, "DSM_USER": "dsm-mini", "DSM_PASSWORD": "saved",
	}); err != nil {
		t.Fatal(err)
	}
	code, got := sc.do(t, http.MethodPost, "/dsm/admin/check-account", `{"user":"dsm-mini","password":""}`)
	if code != http.StatusOK {
		t.Fatalf("got %d %v", code, got)
	}
	items, _ := got["items"].([]any)
	if len(items) == 0 || items[0].(map[string]any)["state"] != "ok" {
		t.Fatalf("items %v: the saved password should have been used", items)
	}
}
