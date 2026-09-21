// Package config loads and validates the service configuration.
//
// Everything is configured through environment variables — neither the image
// nor the repository may contain a single value tied to a particular NAS.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full service configuration.
//
// Fields holding secrets (DSMPassword, BotToken) must not reach the logs:
// Config has a LogValue that strips them.
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

// Load reads the configuration from the environment and checks it.
//
// It returns every problem at once rather than the first one it hits:
// someone bringing the service up the first time should not fix them one by one.
func Load() (*Config, error) {
	c := &Config{
		DSMURL:        env("DSM_URL", ""),
		DSMUser:       env("DSM_USER", ""),
		DSMPassword:   env("DSM_PASSWORD", ""),
		DSMOTP:        env("DSM_OTP", ""),
		DSMInsecure:   envBool("DSM_INSECURE_TLS", false),
		BotToken:      env("TELEGRAM_BOT_TOKEN", ""),
		PublicURL:     strings.TrimRight(env("PUBLIC_URL", ""), "/"),
		ListenAddr:    env("LISTEN_ADDR", ":8080"),
		LogLevel:      env("LOG_LEVEL", "info"),
		WatchInterval: envDuration("WATCH_INTERVAL", 30*time.Second),
	}

	var problems []string

	ids, err := parseUserIDs(env("ALLOWED_USER_IDS", ""))
	if err != nil {
		problems = append(problems, "ALLOWED_USER_IDS: "+err.Error())
	}
	c.AllowedUserIDs = ids

	if c.DSMURL == "" {
		problems = append(problems, "DSM_URL: not set (for example https://192.168.1.10:5001)")
	} else if u, err := url.Parse(c.DSMURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		problems = append(problems, "DSM_URL: must look like https://host:5001")
	} else {
		c.DSMURL = strings.TrimRight(c.DSMURL, "/")
	}

	if c.DSMUser == "" {
		problems = append(problems, "DSM_USER: not set")
	}
	if c.DSMPassword == "" {
		problems = append(problems, "DSM_PASSWORD: not set")
	}
	if c.BotToken == "" {
		problems = append(problems, "TELEGRAM_BOT_TOKEN: not set, get one from @BotFather")
	}

	// An empty list means "let nobody in", not "let everybody in". A service
	// exposed to the internet must not go public over a forgotten line.
	if len(c.AllowedUserIDs) == 0 {
		problems = append(problems, "ALLOWED_USER_IDS: not set — without it access is closed to everyone")
	}

	if c.PublicURL == "" {
		problems = append(problems, "PUBLIC_URL: not set (public HTTPS address of the Mini App)")
	} else if !strings.HasPrefix(c.PublicURL, "https://") {
		problems = append(problems, "PUBLIC_URL: Telegram opens the Mini App over https only")
	}

	if c.WatchInterval < 5*time.Second {
		problems = append(problems, "WATCH_INTERVAL: no less than 5s, or DSM chokes on the polling")
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("check the configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return c, nil
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

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
