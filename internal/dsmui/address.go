package dsmui

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexlnos/dsm-mini/internal/config"
)

// Telegram opens a Mini App only over a public HTTPS address with a real
// certificate, so something has to tie a name from the internet to a port on
// this NAS. DSM can do every part of it, and this is where the screen finds
// out which parts are already done.
type addressView struct {
	// Public is what the service is configured with right now.
	Public string `json:"public"`
	// ListenPort is the port a reverse proxy rule has to point at.
	ListenPort int `json:"listen_port"`
	// ExternalIP is what the NAS believes its address on the internet is —
	// the value a domain's A record has to match.
	ExternalIP string `json:"external_ip"`
	// Matching is the rule that already sends a name to our port, if any.
	// When it is there, nothing else on this screen needs doing.
	Matching *proxyRule `json:"matching"`
	// Proxies is every rule DSM has, so a person can see what is taken.
	Proxies []proxyRule `json:"proxies"`
	// DDNS is the names DSM keeps up to date by itself.
	DDNS []ddnsRecord `json:"ddns"`
}

type proxyRule struct {
	FQDN        string `json:"fqdn"`
	Port        int    `json:"port"`
	HTTPS       bool   `json:"https"`
	BackendHost string `json:"backend_host"`
	BackendPort int    `json:"backend_port"`
	Description string `json:"description"`
}

type ddnsRecord struct {
	Hostname string `json:"hostname"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

func (s *Server) handleAddress(w http.ResponseWriter, r *http.Request) {
	values, ok := s.readSettings(w, r)
	if !ok {
		return
	}

	view := addressView{
		Public:     values["PUBLIC_URL"],
		ListenPort: listenPort(values["LISTEN_ADDR"], s.listenAddr),
	}

	// Each of these is optional: a NAS with no DDNS and no rules is exactly
	// the case the screen exists for, so one empty answer must not blank the
	// whole page. Failures are logged and the section is simply left out.
	nas := s.session(r)
	view.Proxies = s.readProxies(r, nas)
	for i, p := range view.Proxies {
		if isLoopback(p.BackendHost) && p.BackendPort == view.ListenPort {
			view.Matching = &view.Proxies[i]
			break
		}
	}
	view.ExternalIP = s.readExternalIP(r, nas)
	view.DDNS = s.readDDNS(r, nas)

	writeJSON(w, http.StatusOK, view)
}

func (s *Server) readProxies(r *http.Request, nas Caller) []proxyRule {
	var body struct {
		Entries []struct {
			Description string `json:"description"`
			Frontend    struct {
				FQDN     string `json:"fqdn"`
				Port     int    `json:"port"`
				Protocol int    `json:"protocol"`
			} `json:"frontend"`
			Backend struct {
				FQDN string `json:"fqdn"`
				Port int    `json:"port"`
			} `json:"backend"`
		} `json:"entries"`
	}
	if err := nas.Call(r.Context(), "SYNO.Core.AppPortal.ReverseProxy", "list", 1, nil, &body); err != nil {
		s.log.Warn("cannot read the reverse proxy rules", "err", err)
		return nil
	}
	out := make([]proxyRule, 0, len(body.Entries))
	for _, e := range body.Entries {
		out = append(out, proxyRule{
			FQDN: e.Frontend.FQDN,
			Port: e.Frontend.Port,
			// DSM stores the protocol as a number: 0 is http, 1 https.
			HTTPS:       e.Frontend.Protocol == 1,
			BackendHost: e.Backend.FQDN,
			BackendPort: e.Backend.Port,
			Description: e.Description,
		})
	}
	return out
}

func (s *Server) readExternalIP(r *http.Request, nas Caller) string {
	var body []struct {
		IP   string `json:"ip"`
		Type string `json:"type"`
	}
	if err := nas.Call(r.Context(), "SYNO.Core.DDNS.ExtIP", "list", 1, nil, &body); err != nil {
		s.log.Warn("cannot read the external address", "err", err)
		return ""
	}
	for _, e := range body {
		if e.IP != "" {
			return e.IP
		}
	}
	return ""
}

func (s *Server) readDDNS(r *http.Request, nas Caller) []ddnsRecord {
	var body struct {
		Records []struct {
			Hostname string `json:"hostname"`
			Provider string `json:"provider"`
			Status   string `json:"status"`
		} `json:"records"`
	}
	if err := nas.Call(r.Context(), "SYNO.Core.DDNS.Record", "list", 1, nil, &body); err != nil {
		s.log.Warn("cannot read the ddns records", "err", err)
		return nil
	}
	out := make([]ddnsRecord, 0, len(body.Records))
	for _, e := range body.Records {
		out = append(out, ddnsRecord{Hostname: e.Hostname, Provider: e.Provider, Status: e.Status})
	}
	return out
}

// handleCreateProxy adds the rule that sends a name to this service and
// records the address as the public one.
//
// It does not touch ports or certificates: whether the router forwards 443 and
// whether the name has a certificate are things the screen reports and the
// person decides on, because getting either wrong silently is worse than
// saying plainly that it is not done.
func (s *Server) handleCreateProxy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FQDN string `json:"fqdn"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(strings.ToLower(req.FQDN))
	if name == "" || strings.ContainsAny(name, "/: ") {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "enter a name like nas.example.com", "field": "fqdn"})
		return
	}

	values, ok := s.readSettings(w, r)
	if !ok {
		return
	}
	port := listenPort(values["LISTEN_ADDR"], s.listenAddr)

	entry := map[string]any{
		"description": "dsm-mini Telegram Mini App",
		"frontend": map[string]any{
			"protocol": 1, "fqdn": name, "port": 443,
			"https": map[string]any{"hsts": false, "http2": false},
		},
		"backend":               map[string]any{"protocol": 0, "fqdn": "localhost", "port": port},
		"proxy_connect_timeout": 60, "proxy_read_timeout": 60, "proxy_send_timeout": 60,
		"proxy_intercept_errors": false, "customize_headers": []any{},
	}
	if err := s.session(r).Call(r.Context(), "SYNO.Core.AppPortal.ReverseProxy", "create", 1,
		map[string]any{"entry": entry}, nil); err != nil {
		s.fail(w, r, err, "cannot create the reverse proxy rule")
		return
	}

	values["PUBLIC_URL"] = "https://" + name
	if err := config.WriteFile(SettingsFile(s.stateDir), values); err != nil {
		s.fail(w, r, err, "the rule was created but the address was not saved")
		return
	}
	who, _ := SessionFrom(r.Context())
	s.log.Info("reverse proxy rule created", "fqdn", name, "port", port, "by", who.User)

	// The bot puts the address on its Mini App button, so it starts again
	// with the new one, the same way a save from the form does.
	writeJSON(w, http.StatusOK, map[string]any{"public": values["PUBLIC_URL"], "applying": true})
	s.restart()
}

// listenPort is the port a rule has to point at. The file wins over the
// address the service is actually on: a port changed on this screen may not
// have taken effect yet, and the rule should describe where it will be.
func listenPort(fromFile, running string) int {
	for _, addr := range []string{fromFile, running} {
		if addr == "" {
			continue
		}
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		if n, err := strconv.Atoi(port); err == nil && n > 0 {
			return n
		}
	}
	return config.DefaultPort
}

func isLoopback(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
