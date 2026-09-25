// Package config loads and checks the service settings.
//
// They live in one file, config.env in the package var, and the settings
// window inside DSM is what writes it. Nothing is asked at installation: a
// fresh package has no file at all, serves only that window, and starts the
// rest once the window has saved enough to start on. Neither the binary nor
// the repository may hold a single value tied to a particular NAS.
//
// The service reads the file itself rather than having the start script
// export it. The window applies a change by starting the service's insides
// again within the same process, and an environment exported once at process
// start would go on holding the old values. Environment variables fill in
// only the keys the file does not mention — that is for running the binary by
// hand while developing it.
package config

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config is the full service configuration.
//
// Fields holding secrets (DSMPassword, BotToken) must not reach the logs.
type Config struct {
	DSMURL      string
	DSMUser     string
	DSMPassword string
	DSMOTP      string
	DSMInsecure bool

	BotToken       string
	AllowedUserIDs []int64
	PublicURL      string

	ListenAddr    string
	WatchInterval time.Duration
	LogLevel      string
}

// DefaultPort is where the service listens unless told otherwise. It is the
// same in this repository's package and in the SynoCommunity one, so that the
// two can replace each other: 8080 belongs to SABnzbd in the SynoCommunity
// port list, and their rule for internal services is 49152–65535.
const DefaultPort = 58080

// Problem is one reason the settings are not enough to start on.
//
// It travels to the settings window as a key and a code rather than as text:
// the window speaks ten languages, and the log speaks English.
type Problem struct {
	Key  string `json:"key"`
	Code string `json:"code"`
}

// The codes a Problem carries.
const (
	// Missing: a required value is not there at all.
	Missing = "missing"
	// BadURL: an address that is not http(s)://host.
	BadURL = "url"
	// NotHTTPS: Telegram opens a Mini App over https only.
	NotHTTPS = "https"
	// BadIDs: the allow list is not numbers separated by commas.
	BadIDs = "ids"
	// BadListen: the listen address is not host:port.
	BadListen = "listen"
	// TooOften: the watcher interval is shorter than DSM puts up with.
	TooOften = "interval"
)

func (p Problem) String() string {
	switch p.Code {
	case Missing:
		return p.Key + ": not set"
	case BadURL:
		return p.Key + ": must look like https://host:5001"
	case NotHTTPS:
		return p.Key + ": Telegram opens the Mini App over https only"
	case BadIDs:
		return p.Key + ": expected numeric ids separated by commas"
	case BadListen:
		return p.Key + ": must look like 127.0.0.1:58080"
	case TooOften:
		return p.Key + ": no less than 5s, or DSM chokes on the polling"
	}
	return p.Key + ": " + p.Code
}

// Required are the settings only the person installing can know, in the
// order the settings window asks for them. Everything else has a default.
var Required = []string{
	"DSM_USER", "DSM_PASSWORD", "TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "PUBLIC_URL",
}

// Defaults are the values the optional settings take when nothing sets them,
// for the settings window to show in the empty fields.
func Defaults() map[string]string {
	return map[string]string{
		"DSM_URL":     defaultDSMURL(),
		"LISTEN_ADDR": "127.0.0.1:" + strconv.Itoa(DefaultPort),
	}
}

// Load reads the settings file and checks what it says.
//
// A missing file is not an error: that is a package nobody has set up yet,
// and it comes back as a list of problems. The error is for a file that is
// there and cannot be read — on a NAS, one left behind by a package that ran
// as another user.
//
// The Config is never nil, even with problems or an error: the settings
// window has to be served whatever state the file is in, and it needs the
// defaults — which port to listen on, where DSM is — to be served at all.
func Load(path string) (*Config, []Problem, error) {
	values, err := ReadFile(path)
	if err != nil {
		c, _ := parse(nil)
		return c, nil, err
	}
	c, problems := parse(values)
	return c, problems, nil
}

// Check says what is wrong with one value, as a Problem code, or "" when it is
// fine. It is shallow on purpose: whether a token or a password works is not
// something a pattern knows, and the service reports that once it tries them.
// An empty value passes: whether a key may be empty is Load's business.
func Check(key, v string) string {
	if v == "" {
		return ""
	}
	switch key {
	case "DSM_URL":
		u, err := url.Parse(v)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return BadURL
		}
	case "PUBLIC_URL":
		u, err := url.Parse(v)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return BadURL
		}
		if u.Scheme != "https" {
			return NotHTTPS
		}
	case "ALLOWED_USER_IDS":
		if _, err := parseUserIDs(v); err != nil {
			return BadIDs
		}
	case "LISTEN_ADDR":
		_, port, err := net.SplitHostPort(v)
		if err != nil {
			return BadListen
		}
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return BadListen
		}
	}
	return ""
}

func parse(values map[string]string) (*Config, []Problem) {
	get := func(key, def string) string {
		// A key the file names wins even when it is empty: ALLOWED_USER_IDS=''
		// in the file means "nobody", and an environment variable must not be
		// able to quietly say otherwise.
		if v, ok := values[key]; ok {
			return strings.TrimSpace(v)
		}
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	c := &Config{
		DSMURL:      strings.TrimRight(get("DSM_URL", defaultDSMURL()), "/"),
		DSMUser:     get("DSM_USER", ""),
		DSMPassword: get("DSM_PASSWORD", ""),
		DSMOTP:      get("DSM_OTP", ""),
		BotToken:    get("TELEGRAM_BOT_TOKEN", ""),
		PublicURL:   strings.TrimRight(get("PUBLIC_URL", ""), "/"),
		// Loopback, not ":58080": the DSM reverse proxy connects through
		// localhost, and binding every interface would put the app on the
		// LAN as well, next to the one address that is supposed to reach it.
		ListenAddr: get("LISTEN_ADDR", "127.0.0.1:"+strconv.Itoa(DefaultPort)),
		LogLevel:   get("LOG_LEVEL", "info"),
	}

	// Up to 1.0.0 the package wrote ":port" — every interface — so the app
	// answered anyone on the local network, around the reverse proxy it is
	// meant to sit behind. That was a mistake rather than a choice, so it is
	// read as loopback. Only the bare form: an address someone set on
	// purpose, 0.0.0.0:58080 or a particular interface, stays as it is.
	if strings.HasPrefix(c.ListenAddr, ":") {
		c.ListenAddr = "127.0.0.1" + c.ListenAddr
	}

	// The service runs on the NAS and reaches DSM over loopback, where
	// there is nothing in between to be fooled, while DSM answers with its
	// own self-signed certificate. So verification is off for loopback
	// unless the file says otherwise, and on for anything else.
	c.DSMInsecure = parseBool(get("DSM_INSECURE_TLS", ""), isLoopbackURL(c.DSMURL))
	c.WatchInterval = parseDuration(get("WATCH_INTERVAL", ""), 30*time.Second)

	var problems []Problem
	for _, key := range Required {
		if get(key, "") == "" {
			problems = append(problems, Problem{Key: key, Code: Missing})
		}
	}
	for key, v := range map[string]string{
		"DSM_URL": c.DSMURL, "PUBLIC_URL": c.PublicURL,
		"ALLOWED_USER_IDS": get("ALLOWED_USER_IDS", ""), "LISTEN_ADDR": c.ListenAddr,
	} {
		if code := Check(key, v); code != "" {
			problems = append(problems, Problem{Key: key, Code: code})
		}
	}
	if c.WatchInterval < 5*time.Second {
		problems = append(problems, Problem{Key: "WATCH_INTERVAL", Code: TooOften})
	}

	// An empty list means "let nobody in", not "let everybody in", and it is
	// reported as missing rather than accepted: a service exposed to the
	// internet must not go public over a forgotten line, and one nobody may
	// use is not worth starting.
	c.AllowedUserIDs, _ = parseUserIDs(get("ALLOWED_USER_IDS", ""))

	sortProblems(problems)
	return c, problems
}

// sortProblems puts them in the order the window asks the questions, so the
// log and the screen read the same way.
func sortProblems(p []Problem) {
	rank := func(key string) int {
		for i, k := range Required {
			if k == key {
				return i
			}
		}
		return len(Required)
	}
	sort.SliceStable(p, func(i, j int) bool {
		if ri, rj := rank(p[i].Key), rank(p[j].Key); ri != rj {
			return ri < rj
		}
		return p[i].Key < p[j].Key
	})
}

// IsAllowed reports whether this Telegram user may use the service.
func (c *Config) IsAllowed(userID int64) bool {
	for _, id := range c.AllowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// DSMWebConfig is where DSM describes its own web server. World-readable,
// undocumented, read off DSM 7.4: `port` is plain HTTP, `ssl.port` HTTPS.
// A variable so the tests can point it elsewhere.
var DSMWebConfig = "/usr/syno/etc/www/DSM.json"

// defaultDSMURL is DSM on this machine, on whatever HTTPS port it was moved
// to. The port matters more than it looks: the settings window checks who is
// asking by calling DSM, so with a wrong port nobody could open the window
// that would fix it.
func defaultDSMURL() string {
	port := 5001
	if raw, err := os.ReadFile(DSMWebConfig); err == nil {
		var web struct {
			SSL struct {
				Port int `json:"port"`
			} `json:"ssl"`
		}
		if json.Unmarshal(raw, &web) == nil && web.SSL.Port > 0 && web.SSL.Port < 65536 {
			port = web.SSL.Port
		}
	}
	return "https://localhost:" + strconv.Itoa(port)
}

func isLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func parseUserIDs(raw string) ([]int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, errors.New("expected numeric IDs separated by commas, got " + strconv.Quote(part))
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseBool(v string, def bool) bool {
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func parseDuration(v string, def time.Duration) time.Duration {
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// Describe joins problems into the one line the log gets.
func Describe(problems []Problem) string {
	parts := make([]string, len(problems))
	for i, p := range problems {
		parts[i] = p.String()
	}
	return strings.Join(parts, "; ")
}
