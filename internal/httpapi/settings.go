package httpapi

import (
	"context"
	"net/http"

	"github.com/alexlnos/dsm-mini/internal/store"
)

// settingsView is the settings plus what the interface needs to show them.
type settingsView struct {
	store.Settings
	// Suggested are folders worth pinning: the Download Station default and
	// the ones downloaded into recently.
	Suggested []string `json:"suggested"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	u, _ := userFrom(r.Context())
	writeJSON(w, http.StatusOK, settingsView{
		Settings:  s.userSettings(r.Context(), u.ID),
		Suggested: s.suggestFolders(r),
	})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req store.Settings
	if !decode(w, r, &req, s) {
		return
	}
	u, _ := userFrom(r.Context())

	if s.settings == nil {
		s.bad(w, r, "bad.noStore")
		return
	}
	saved, err := s.settings.Set(r.Context(), u.ID, req)
	if err != nil {
		s.fail(w, r, err, "api.settings")
		return
	}
	s.log.Info("settings saved", "user", u.ID, "pinned", len(saved.PinnedFolders))
	writeJSON(w, http.StatusOK, settingsView{
		Settings:  saved,
		Suggested: s.suggestFolders(r),
	})
}

func (s *Server) userSettings(ctx context.Context, userID int64) store.Settings {
	if s.settings == nil {
		return store.Defaults()
	}
	settings, err := s.settings.Get(ctx, userID)
	if err != nil {
		// Settings are not worth refusing service over: we show the defaults
		// and write to the log.
		s.log.Warn("cannot read the settings", "user", userID, "err", err)
		return store.Defaults()
	}
	return settings
}

// suggestFolders offers folders worth pinning: the Download Station default
// one and those the user has already put downloads into themselves.
//
// Folders of existing tasks deliberately stay out: those are other people's
// torrents and long-past downloads, unrelated to a new one.
func (s *Server) suggestFolders(r *http.Request) []string {
	ctx := r.Context()
	u, _ := userFrom(ctx)

	seen := map[string]bool{}
	var out []string
	add := func(folder string) {
		if folder == "" || seen[folder] {
			return
		}
		seen[folder] = true
		out = append(out, folder)
	}

	if def, err := s.ds.DefaultDestination(ctx); err == nil {
		add(def)
	}
	if s.settings != nil {
		recent, err := s.settings.RecentFolders(ctx, u.ID, 8)
		if err != nil {
			s.log.Warn("cannot read the folder history", "err", err)
		}
		for _, folder := range recent {
			add(folder)
		}
	}
	return out
}

// folderChoices returns the folders for the add screen in the order the user
// will see them: pinned first, then, if allowed, the recent ones.
func (s *Server) folderChoices(r *http.Request, userID int64) []string {
	settings := s.userSettings(r.Context(), userID)

	seen := map[string]bool{}
	out := make([]string, 0, 12)
	add := func(f string) {
		if f == "" || seen[f] {
			return
		}
		seen[f] = true
		out = append(out, f)
	}

	for _, f := range settings.PinnedFolders {
		add(f)
	}
	// Recent means the user's own history, not Download Station task folders.
	if settings.ShowRecent || len(out) == 0 {
		for _, f := range s.suggestFolders(r) {
			add(f)
		}
	}
	return out
}
