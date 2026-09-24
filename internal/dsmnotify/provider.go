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
	"regexp"
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

	// The method has to be lower case. DSM's own check on create accepts
	// "POST", stores it, and then the sender refuses it every time:
	//
	//   curl.cpp:177 Invalid HTTP method: [POST]. Only 'get' and 'post' accepted.
	//
	// in /var/log/synoscgi.log, while the API answers send_test with a bare
	// 4682 and nothing reaches the service. Read off a live NAS; the provider
	// looked perfect in every listing.
	reqMethod = "post"

	// providerName is what DSM shows in its list of webhooks, next to the
	// owner's phone and whatever else sends there — so it is the package's own
	// visible name, the one Package Center shows, and says whose webhook this
	// is without anyone having to open it. It has to agree with `displayname`
	// in INFO, `dname` in the catalogue and DISPLAY_NAME in the spksrc recipe.
	//
	// The field that carries it is `provider`, not `target_name` — the latter
	// is accepted by create and silently dropped, which left the entry
	// nameless.
	providerName = "DSM mini — Telegram Mini App"
)

// Caller is the part of the DSM client this package needs.
type Caller interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
}

// ours recognises a webhook this service registered, whatever port it was on:
// loopback, and the path nothing else on a NAS would use. It is what lets a
// provider left behind by an earlier port, or by an uninstalled package, be
// told apart from somebody's own webhook to a chat or a phone.
var ours = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+/dsm/notify$`)

type providerList struct {
	List []struct {
		ProfileID    int `json:"profile_id"`
		TargetConfig struct {
			URL       string `json:"url"`
			ReqHeader string `json:"req_header"`
			ReqParam  string `json:"req_param"`
			ReqMethod string `json:"req_method"`
			Provider  string `json:"provider"`
		} `json:"target_config"`
	} `json:"list"`
}

// Ensure makes DSM call url on every notification, creating the provider or
// correcting an existing one.
//
// It is safe to call on every start: an entry that already says the right
// thing is left alone, so restarts do not pile up providers or rewrite
// settings somebody may be looking at.
//
// It also removes this service's providers on any other port. The port can
// change — the SynoCommunity build moved it from 8080 to 58080 — and a
// provider pointing at the old one stays in DSM's list forever otherwise,
// with DSM calling an address nothing answers on every notification. One
// such orphan was found on a live NAS and had to be deleted by hand.
func Ensure(ctx context.Context, c Caller, url, secret string, log *slog.Logger) error {
	var list providerList
	if err := c.Call(ctx, providerAPI, "list", 2, nil, &list); err != nil {
		return fmt.Errorf("cannot read the notification providers: %w", err)
	}

	header := "Content-Type:application/json" + headerSep +
		SecretHeader + ":" + secret + headerSep

	current := -1
	for i, p := range list.List {
		if !ours.MatchString(p.TargetConfig.URL) {
			continue
		}
		if p.TargetConfig.URL != url || current >= 0 {
			// Another port, or a second entry for this one.
			if err := remove(ctx, c, p.ProfileID); err != nil {
				log.Warn("cannot remove a stale dsm notification webhook",
					"profile", p.ProfileID, "url", p.TargetConfig.URL, "err", err)
				continue
			}
			log.Info("stale dsm notification webhook removed",
				"profile", p.ProfileID, "url", p.TargetConfig.URL)
			continue
		}
		current = i
	}

	if current >= 0 {
		p := list.List[current]
		// The method is compared exactly, not case-insensitively: a provider an
		// earlier version registered with "POST" looks right and never sends,
		// and this is where it gets corrected on the next start.
		if p.TargetConfig.ReqHeader == header &&
			p.TargetConfig.ReqParam == bodyTemplate &&
			p.TargetConfig.ReqMethod == reqMethod &&
			p.TargetConfig.Provider == providerName {
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

// Remove deletes every provider this service registered. It is what the
// package calls on a real uninstall: nothing else would, and the entry would
// outlive the package, with DSM calling a port nobody listens on.
//
// Not on an upgrade — the uninstall hook runs then too, and removing the
// provider there would only have the next start create it again under a new
// number, losing whatever was changed in DSM's notification rules meanwhile.
// The package scripts check SYNOPKG_PKG_STATUS before calling this.
func Remove(ctx context.Context, c Caller, log *slog.Logger) error {
	var list providerList
	if err := c.Call(ctx, providerAPI, "list", 2, nil, &list); err != nil {
		return fmt.Errorf("cannot read the notification providers: %w", err)
	}
	removed := 0
	for _, p := range list.List {
		if !ours.MatchString(p.TargetConfig.URL) {
			continue
		}
		if err := remove(ctx, c, p.ProfileID); err != nil {
			return fmt.Errorf("cannot remove the notification provider %d: %w", p.ProfileID, err)
		}
		removed++
	}
	log.Info("dsm notification webhooks removed", "count", removed)
	return nil
}

func remove(ctx context.Context, c Caller, profileID int) error {
	return c.Call(ctx, providerAPI, "delete", 2, map[string]any{"profile_id": profileID}, nil)
}

func payload(url, header string) map[string]any {
	return map[string]any{
		"type":             "webhook",
		"url":              url,
		"req_method":       reqMethod,
		"req_header":       header,
		"req_param":        bodyTemplate,
		"template_id":      templateAll,
		"needssl":          false,
		"interval":         0,
		"use_default_lang": true,
		"prefix":           "",
		"sepchar":          "",
		"port":             0,
		"provider":         providerName,
	}
}
