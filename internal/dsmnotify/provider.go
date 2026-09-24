// Package dsmnotify carries what DSM itself announces — the security advisor,
// storage warnings, package updates — through to Telegram.
//
// DSM has no API for reading its notification feed: SYNO.Core.DSMNotify
// answers 103, "no such method", on DSM 7.4. What it does have is a webhook
// provider, the same mechanism its SMS and Line integrations use: DSM calls a
// URL of our choosing on every notification. The service runs on the NAS, so
// that call never leaves loopback.
package dsmnotify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

const (
	// The API that owns webhook providers. Its shape was read off a live NAS
	// rather than a document: `type` has to be the string "webhook", and every
	// numeric value is refused with 4681.
	providerAPI = "SYNO.Core.Notification.Push.Webhook.Provider"

	// DSM's own template 1, "All": info, warning and error. Filtering happens
	// on our side, where the person's choice is.
	templateAll = 1

	// DSM joins request headers with a bare \r — not \r\n. Taken from a
	// provider it stored after its own interface wrote one.
	headerSep = "\r"

	// SecretHeader carries the secret DSM was given when the provider was
	// registered. The endpoint is served by the same server as the Mini App
	// and is therefore reachable through the reverse proxy like any other
	// path, so this header is what tells DSM's call from anyone else's.
	SecretHeader = "X-Dsm-Mini-Secret"

	// The message itself. DSM substitutes the notification text here.
	bodyTemplate = `{"text":"@@TEXT@@"}`
)

// Caller is the part of the DSM client this package needs.
type Caller interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
}

type providerList struct {
	List []struct {
		ProfileID    int `json:"profile_id"`
		TargetConfig struct {
			URL       string `json:"url"`
			ReqHeader string `json:"req_header"`
			ReqParam  string `json:"req_param"`
			ReqMethod string `json:"req_method"`
		} `json:"target_config"`
	} `json:"list"`
}

// Ensure makes DSM call url on every notification, creating the provider or
// correcting an existing one.
//
// It is safe to call on every start: an entry that already says the right
// thing is left alone, so restarts do not pile up providers or rewrite
// settings somebody may be looking at.
func Ensure(ctx context.Context, c Caller, url, secret string, log *slog.Logger) error {
	var list providerList
	if err := c.Call(ctx, providerAPI, "list", 2, nil, &list); err != nil {
		return fmt.Errorf("cannot read the notification providers: %w", err)
	}

	header := "Content-Type:application/json" + headerSep +
		SecretHeader + ":" + secret + headerSep

	for _, p := range list.List {
		if p.TargetConfig.URL != url {
			continue
		}
		if p.TargetConfig.ReqHeader == header &&
			p.TargetConfig.ReqParam == bodyTemplate &&
			strings.EqualFold(p.TargetConfig.ReqMethod, "POST") {
			log.Debug("dsm notification webhook already registered", "profile", p.ProfileID)
			return nil
		}
		params := payload(url, header)
		params["profile_id"] = p.ProfileID
		if err := c.Call(ctx, providerAPI, "set", 2, params, nil); err != nil {
			return fmt.Errorf("cannot correct the notification provider: %w", err)
		}
		log.Info("dsm notification webhook corrected", "profile", p.ProfileID)
		return nil
	}

	if err := c.Call(ctx, providerAPI, "create", 2, payload(url, header), nil); err != nil {
		return fmt.Errorf("cannot register the notification provider: %w", err)
	}
	log.Info("dsm notification webhook registered", "url", url)
	return nil
}

func payload(url, header string) map[string]any {
	return map[string]any{
		"type":             "webhook",
		"url":              url,
		"req_method":       "POST",
		"req_header":       header,
		"req_param":        bodyTemplate,
		"template_id":      templateAll,
		"needssl":          false,
		"interval":         0,
		"use_default_lang": true,
		"prefix":           "",
		"sepchar":          "",
		"port":             0,
		"provider":         "",
	}
}
