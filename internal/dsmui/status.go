package dsmui

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/alexlnos/dsm-mini/internal/config"
)

// Status is what the service is doing, as the settings window shows it.
//
// It replaces the question "why is it not working" being answered from the
// log, or from Package Center, which only knows "stopped" and offers Repair.
// A service that is not set up, cannot sign in to DSM or had its token
// refused stays up and says so here instead of exiting.
type Status struct {
	mu sync.RWMutex
	v  StatusView
}

// StatusView is the status as the window receives it.
type StatusView struct {
	// State sums the rest up:
	//   setup    — the settings are not enough to start on;
	//   starting — DSM and Telegram are being reached;
	//   running  — both answered;
	//   failed   — one of them refused, and it will not change by itself.
	State string `json:"state"`
	// Problems lists what the settings lack, in the order the window asks.
	Problems []config.Problem `json:"problems,omitempty"`
	// Unreadable is set when files of an earlier installation are in the way.
	Unreadable *Unreadable `json:"unreadable,omitempty"`
	DSM        Link        `json:"dsm"`
	Bot        Link        `json:"bot"`
	// Since is when this run of the service began, in milliseconds. After a
	// save the window waits for it to change: until it does, the answer comes
	// from the run that is about to stop, and its state is the old one.
	Since int64 `json:"since"`
}

// Link is one of the two connections the service needs.
type Link struct {
	// State is "", "waiting", "ok" or "failed". Waiting is for what cures
	// itself — a DSM that is still booting, a Telegram that is out of reach;
	// failed is for what needs a person: a refused password, a refused token.
	State string `json:"state,omitempty"`
	// Reason says which: "auth", "unreachable", "download_station", "token".
	Reason string `json:"reason,omitempty"`
	// Code is DSM's own code for a refused sign-in: a wrong password and a
	// demand for a two-factor code need different advice.
	Code int `json:"code,omitempty"`
	// Name is the bot's username once Telegram has answered.
	Name string `json:"name,omitempty"`
}

// Link states and reasons.
const (
	LinkWaiting = "waiting"
	LinkOK      = "ok"
	LinkFailed  = "failed"

	ReasonAuth            = "auth"
	ReasonUnreachable     = "unreachable"
	ReasonDownloadStation = "download_station"
	ReasonToken           = "token"
)

// Unreadable describes files the service found and cannot open.
//
// That is what switching between two builds of the package that run as
// different users leaves behind: DSM hands the data folder to the new user on
// an upgrade, but not the files inside it. Package Center then shows a stopped
// package and a Repair button, and nothing anywhere says why.
type Unreadable struct {
	Files []string `json:"files"`
	// Owner is who the files belong to, when that can be told.
	Owner string `json:"owner,omitempty"`
	// Command is the one line that hands them back, for whoever has SSH and
	// wants to keep the old settings rather than type them again.
	Command string `json:"command"`
}

// NewStatus starts out as "starting": nothing is known yet.
func NewStatus() *Status {
	return &Status{v: StatusView{Since: time.Now().UnixMilli()}}
}

// SetProblems records what the settings lack.
func (s *Status) SetProblems(p []config.Problem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v.Problems = p
}

// SetUnreadable records files that are in the way.
func (s *Status) SetUnreadable(u *Unreadable) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v.Unreadable = u
}

// SetDSM records how the connection to DSM is doing.
func (s *Status) SetDSM(l Link) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v.DSM = l
}

// SetBot records how the connection to Telegram is doing.
func (s *Status) SetBot(l Link) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v.Bot = l
}

// View returns a copy with State worked out.
func (s *Status) View() StatusView {
	s.mu.RLock()
	v := s.v
	s.mu.RUnlock()

	switch {
	case len(v.Problems) > 0 || v.Unreadable != nil:
		v.State = "setup"
	case v.DSM.State == LinkFailed || v.Bot.State == LinkFailed:
		v.State = "failed"
	case v.DSM.State == LinkOK && v.Bot.State == LinkOK:
		v.State = "running"
	default:
		v.State = "starting"
	}
	return v
}

// FindUnreadable looks at the files in dir that the service needs and
// returns the ones it cannot open, or nil when every one of them is fine or
// absent.
func FindUnreadable(dir string, names ...string) *Unreadable {
	var out Unreadable
	for _, name := range names {
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			f.Close()
			continue
		}
		if !os.IsPermission(err) {
			continue
		}
		out.Files = append(out.Files, name)
		if out.Owner == "" {
			out.Owner = ownerOf(path)
		}
	}
	if len(out.Files) == 0 {
		return nil
	}

	// The real path rather than /var/packages/<id>/var, which is a symlink:
	// chown -R on a symlink changes the link and nothing it points at.
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		real = dir
	}
	me := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	if u, err := user.Current(); err == nil {
		me = u.Username
		if g, err := user.LookupGroupId(u.Gid); err == nil {
			me += ":" + g.Name
		}
	}
	out.Command = fmt.Sprintf("sudo chown -R %s %s", me, real)
	return &out
}

func ownerOf(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	uid := strconv.FormatUint(uint64(st.Uid), 10)
	if u, err := user.LookupId(uid); err == nil {
		return u.Username
	}
	return uid
}
