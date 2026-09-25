// Package dsm is a client for the Synology DSM Web API.
//
// Three quirks of DSM make plain form requests impossible:
//
//  1. The path and allowed versions of every API are discovered at runtime
//     through SYNO.API.Info — they differ between DSM 6 and 7 and between models.
//  2. Some APIs have requestFormat == "JSON": then EVERY parameter value must
//     be encoded as JSON (strings quoted). Otherwise DSM answers 101/105
//     without explanation.
//  3. Errors arrive in the body of an HTTP 200 response, and the session goes
//     stale silently — a transparent re-login is required.
package dsm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is a thread-safe DSM Web API client.
type Client struct {
	baseURL string
	user    string
	pass    string
	otp     string
	http    *http.Client
	log     *slog.Logger

	// cookie and synoToken are set for a client borrowed from a browser
	// session; see Options.
	cookie    string
	synoToken string

	mu   sync.RWMutex
	sid  string
	apis map[string]apiInfo
	// goodVersion remembers the version a call already succeeded with: on some
	// DSM builds the newer API version is broken, and probing it every time
	// means paying for an extra request.
	goodVersion map[string]int
}

type apiInfo struct {
	Path          string `json:"path"`
	MinVersion    int    `json:"minVersion"`
	MaxVersion    int    `json:"maxVersion"`
	RequestFormat string `json:"requestFormat"`
}

// Options are the DSM connection parameters.
type Options struct {
	BaseURL     string
	User        string
	Password    string
	OTP         string
	InsecureTLS bool
	Timeout     time.Duration
	Logger      *slog.Logger

	// Cookie and SynoToken make the client act inside somebody else's browser
	// session instead of signing in itself: the settings window inside DSM
	// works as the administrator who opened it. A browser session is refused
	// without the token whenever DSM's CSRF protection is on, which it is by
	// default. Such a client never signs in, and cannot sign in again when
	// the session ends.
	Cookie    string
	SynoToken string
}

// New creates a client. It makes no network requests — the login happens
// lazily on the first call or explicitly through Login.
func New(o Options) *Client {
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if o.InsecureTLS {
		// DSM ships with a self-signed certificate. Verification is disabled
		// only with explicit consent — see DSM_INSECURE_TLS.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		baseURL:     strings.TrimRight(o.BaseURL, "/"),
		user:        o.User,
		pass:        o.Password,
		otp:         o.OTP,
		cookie:      o.Cookie,
		synoToken:   o.SynoToken,
		http:        &http.Client{Timeout: o.Timeout, Transport: transport},
		log:         o.Logger,
		apis:        make(map[string]apiInfo),
		goodVersion: make(map[string]int),
	}
}

type response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code   int `json:"code"`
		Errors struct {
			// DownloadStation2 puts tasks that refused an action here.
			FailedTask []ItemFailure `json:"failed_task"`
		} `json:"errors"`
	} `json:"error"`
}

// ItemFailure is a refusal for a single item of a batch operation.
type ItemFailure struct {
	ID   string `json:"id"`
	Code int    `json:"error"`
}

// Login signs in and remembers the SID.
func (c *Client) Login(ctx context.Context) error {
	params := map[string]any{
		"account": c.user,
		"passwd":  c.pass,
		"session": "DownloadStation",
		"format":  "sid",
	}
	if c.otp != "" {
		params["otp_code"] = c.otp
	}

	// The login happens before requestFormat is known — parameters go as they are.
	raw, err := c.post(ctx, "entry.cgi", buildForm("SYNO.API.Auth", "login", 7, "", params, false))
	if err != nil {
		return err
	}
	if !raw.Success {
		code := 0
		if raw.Error != nil {
			code = raw.Error.Code
		}
		return &AuthError{Code: code}
	}

	var out struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(raw.Data, &out); err != nil {
		return fmt.Errorf("cannot parse the login response: %w", err)
	}

	c.mu.Lock()
	c.sid = out.SID
	c.mu.Unlock()
	c.log.Info("signed in to DSM", "user", c.user)
	return nil
}

// Logout closes the session. The error can be ignored — the session goes
// stale on its own anyway.
func (c *Client) Logout(ctx context.Context) error {
	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()
	if sid == "" {
		return nil
	}
	_, err := c.post(ctx, "entry.cgi", buildForm("SYNO.API.Auth", "logout", 7,
		sid, map[string]any{"session": "DownloadStation"}, false))
	c.mu.Lock()
	c.sid = ""
	c.mu.Unlock()
	return err
}

// Call invokes a DSM API method and unpacks the data field into out.
//
// It signs in by itself when there is no session yet, and repeats the request
// once if DSM reported a stale session.
func (c *Client) Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error {
	if err := c.ensureSession(ctx); err != nil {
		return err
	}

	data, err := c.call(ctx, api, method, version, params)
	var apiErr *APIError
	if err != nil && c.cookie == "" && asAPIError(err, &apiErr) && apiErr.needsRelogin() {
		c.log.Debug("the DSM session is invalid, signing in again", "api", api)
		if err := c.Login(ctx); err != nil {
			return err
		}
		data, err = c.call(ctx, api, method, version, params)
	}
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s.%s: cannot parse the response: %w", api, method, err)
	}
	return nil
}

func (c *Client) call(ctx context.Context, api, method string, version int, params map[string]any) (json.RawMessage, error) {
	info, err := c.apiInfo(ctx, api)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()

	form := buildForm(api, method, version, sid, params, info.RequestFormat == "JSON")
	raw, err := c.post(ctx, info.Path, form)
	if err != nil {
		return nil, err
	}
	if !raw.Success {
		err := &APIError{API: api, Method: method}
		if raw.Error != nil {
			err.Code = raw.Error.Code
			err.Failed = raw.Error.Errors.FailedTask
		}
		return nil, err
	}
	return raw.Data, nil
}

// apiInfo returns the path and request format of an API, asking the NAS for
// them once and caching the answer.
func (c *Client) apiInfo(ctx context.Context, api string) (apiInfo, error) {
	c.mu.RLock()
	info, ok := c.apis[api]
	c.mu.RUnlock()
	if ok {
		return info, nil
	}

	raw, err := c.post(ctx, "query.cgi", buildForm("SYNO.API.Info", "query", 1, "",
		map[string]any{"query": api}, false))
	if err != nil {
		return apiInfo{}, err
	}
	if !raw.Success {
		return apiInfo{}, fmt.Errorf("cannot get information about API %s", api)
	}

	var all map[string]apiInfo
	if err := json.Unmarshal(raw.Data, &all); err != nil {
		return apiInfo{}, fmt.Errorf("cannot parse SYNO.API.Info: %w", err)
	}
	info, ok = all[api]
	if !ok {
		return apiInfo{}, fmt.Errorf("API %s is missing on this NAS", api)
	}

	c.mu.Lock()
	c.apis[api] = info
	c.mu.Unlock()
	return info, nil
}

// HasAPI reports whether this NAS has such an API. Needed to choose between
// the modern SYNO.DownloadStation2 and the legacy SYNO.DownloadStation.
func (c *Client) HasAPI(ctx context.Context, api string) bool {
	_, err := c.apiInfo(ctx, api)
	return err == nil
}

// APIMaxVersion returns the highest supported API version, 0 when the API is
// missing.
func (c *Client) APIMaxVersion(ctx context.Context, api string) int {
	info, err := c.apiInfo(ctx, api)
	if err != nil {
		return 0
	}
	return info.MaxVersion
}

func (c *Client) ensureSession(ctx context.Context) error {
	if c.cookie != "" {
		// Borrowed: the session is the browser's, and it is DSM's job to say
		// whether it is still good.
		return nil
	}
	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()
	if sid != "" {
		return nil
	}
	return c.Login(ctx)
}

func (c *Client) post(ctx context.Context, path string, form url.Values) (*response, error) {
	endpoint := c.baseURL + "/webapi/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.cookie != "" {
		req.AddCookie(&http.Cookie{Name: "id", Value: c.cookie})
		if c.synoToken != "" {
			req.Header.Set("X-SYNO-TOKEN", c.synoToken)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the request to DSM failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("cannot read the DSM response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: resp.StatusCode}
	}

	var out response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("the DSM response is not JSON: %w", err)
	}
	return &out, nil
}

// buildForm assembles the request body.
//
// With jsonFormat every value is encoded as JSON: the string "url" goes out as
// "\"url\"". That is required by APIs with requestFormat == "JSON" (all of
// DownloadStation2 and Virtualization entry.cgi), and it trips people up most.
func buildForm(api, method string, version int, sid string, params map[string]any, jsonFormat bool) url.Values {
	form := url.Values{}
	form.Set("api", api)
	form.Set("method", method)
	form.Set("version", strconv.Itoa(version))
	if sid != "" {
		form.Set("_sid", sid)
	}
	for k, v := range params {
		if jsonFormat {
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			form.Set(k, string(b))
			continue
		}
		switch t := v.(type) {
		case string:
			form.Set(k, t)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			form.Set(k, string(b))
		}
	}
	return form
}

func asAPIError(err error, target **APIError) bool {
	for err != nil {
		if e, ok := err.(*APIError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
