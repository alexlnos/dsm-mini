package dsmui

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DSMVerifier asks DSM who a session cookie belongs to.
//
// The call is SYNO.Core.Desktop.Initdata, which every DSM desktop makes on
// load. Its Session block carries `user` and `is_admin` — checked against a
// live NAS; there is no documented "who am I" call, and SYNO.Core.CurrentUser
// answers 102, "no such API", on DSM 7.4.
type DSMVerifier struct {
	baseURL string
	client  *http.Client
}

// NewVerifier builds one against the same NAS the service talks to.
func NewVerifier(baseURL string, insecureTLS bool) *DSMVerifier {
	transport := &http.Transport{}
	if insecureTLS {
		// The NAS answers on its own self-signed certificate, and the service
		// reaches it over loopback: there is nothing in between to be fooled.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &DSMVerifier{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Transport: transport, Timeout: 15 * time.Second},
	}
}

func (v *DSMVerifier) Identify(ctx context.Context, cookie string) (Session, error) {
	q := url.Values{
		"api":     {"SYNO.Core.Desktop.Initdata"},
		"method":  {"get"},
		"version": {"1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		v.baseURL+"/webapi/entry.cgi?"+q.Encode(), nil)
	if err != nil {
		return Session{}, err
	}
	req.AddCookie(&http.Cookie{Name: "id", Value: cookie})

	resp, err := v.client.Do(req)
	if err != nil {
		return Session{}, fmt.Errorf("cannot reach dsm: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Session struct {
				User    string `json:"user"`
				IsAdmin bool   `json:"is_admin"`
			} `json:"Session"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Session{}, fmt.Errorf("cannot read the dsm answer: %w", err)
	}
	// DSM answers success:false for a cookie it does not know, rather than an
	// HTTP status — so the status alone would let an expired session through.
	if !body.Success || body.Data.Session.User == "" {
		return Session{}, ErrNotLoggedIn
	}
	return Session{User: body.Data.Session.User, IsAdmin: body.Data.Session.IsAdmin}, nil
}
