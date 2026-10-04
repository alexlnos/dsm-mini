//go:build integration

package dsm

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestSessionNames signs in as DSM_USER under the session name every client
// uses and under the one the settings window's check used to have, and says
// what DSM answered to each. It changes nothing on the NAS: each session is
// signed out by its own SID. A refused sign-in is one failed sign-in in DSM's
// log.
//
// A SynoCommunity tester's ordinary account got in as DownloadStation and was
// refused with 402 as DsmMiniCheck. Run it with an ordinary account — allowed
// Download Station and File Station, as the documentation says — to see what
// this NAS does, and with an administrator for comparison.
func TestSessionNames(t *testing.T) {
	url, user, pass := os.Getenv("DSM_URL"), os.Getenv("DSM_USER"), os.Getenv("DSM_PASSWORD")
	if url == "" || user == "" || pass == "" {
		t.Skip("DSM_URL / DSM_USER / DSM_PASSWORD are not set")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("DSM_INSECURE_TLS"))
	c := New(Options{BaseURL: url, User: user, Password: pass,
		InsecureTLS: insecure, Timeout: 30 * time.Second})
	ctx := context.Background()

	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in as %s: %v — the service could not sign in with this account either", sessionName, err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	var initdata struct {
		Session struct {
			IsAdmin bool `json:"is_admin"`
		} `json:"Session"`
	}
	admin := "unknown"
	if err := c.Call(ctx, "SYNO.Core.Desktop.Initdata", "get", 1, nil, &initdata); err != nil {
		t.Logf("whether %s is an administrator: %v", user, err)
	} else {
		admin = strconv.FormatBool(initdata.Session.IsAdmin)
	}
	t.Logf("%s: signed in; administrator: %s", sessionName, admin)

	const other = "DsmMiniCheck"
	raw, err := c.post(ctx, "entry.cgi", buildForm("SYNO.API.Auth", "login", 7, "", map[string]any{
		"account": user, "passwd": pass, "session": other, "format": "sid",
	}, false))
	if err != nil {
		t.Fatalf("sign in as %s: %v", other, err)
	}
	if !raw.Success {
		code := 0
		if raw.Error != nil {
			code = raw.Error.Code
		}
		t.Logf("%s: refused with code %d", other, code)
		return
	}
	var out struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(raw.Data, &out); err != nil {
		t.Fatalf("cannot parse the login response: %v", err)
	}
	_, _ = c.post(ctx, "entry.cgi", buildForm("SYNO.API.Auth", "logout", 7,
		out.SID, map[string]any{"session": other}, false))
	t.Logf("%s: signed in", other)
}
