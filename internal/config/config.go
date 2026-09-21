// Package config загружает и проверяет конфигурацию сервиса.
//
// Вся настройка идёт через переменные окружения — в образе и в репозитории
// не должно быть ни одного значения, привязанного к конкретному NAS.
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

// Config — полная конфигурация сервиса.
//
// Поля с секретами (DSMPassword, BotToken) не должны попадать в логи:
// у Config есть LogValue, который их вырезает.
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

// Load читает конфигурацию из окружения и проверяет её.
//
// Возвращает все найденные проблемы разом, а не первую попавшуюся: человек,
// который поднимает сервис впервые, не должен чинить их по одной.
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
		problems = append(problems, "DSM_URL: не задан (например https://192.168.1.10:5001)")
	} else if u, err := url.Parse(c.DSMURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		problems = append(problems, "DSM_URL: должен быть вида https://host:5001")
	} else {
		c.DSMURL = strings.TrimRight(c.DSMURL, "/")
	}

	if c.DSMUser == "" {
		problems = append(problems, "DSM_USER: не задан")
	}
	if c.DSMPassword == "" {
		problems = append(problems, "DSM_PASSWORD: не задан")
	}
	if c.BotToken == "" {
		problems = append(problems, "TELEGRAM_BOT_TOKEN: не задан, получите у @BotFather")
	}

	// Пустой список — это не «пускать всех», а «не пускать никого». Сервис,
	// открытый в интернет, не должен становиться публичным из-за забытой строки.
	if len(c.AllowedUserIDs) == 0 {
		problems = append(problems, "ALLOWED_USER_IDS: не задан — без него доступ закрыт для всех")
	}

	if c.PublicURL == "" {
		problems = append(problems, "PUBLIC_URL: не задан (публичный HTTPS-адрес Mini App)")
	} else if !strings.HasPrefix(c.PublicURL, "https://") {
		problems = append(problems, "PUBLIC_URL: Telegram открывает Mini App только по https")
	}

	if c.WatchInterval < 5*time.Second {
		problems = append(problems, "WATCH_INTERVAL: не меньше 5s, иначе DSM захлебнётся опросом")
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("проверьте конфигурацию:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return c, nil
}

// IsAllowed сообщает, разрешён ли доступ этому Telegram-пользователю.
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
			return nil, errors.New("ожидались числовые ID через запятую, получено " + strconv.Quote(part))
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
