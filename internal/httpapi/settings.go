package httpapi

import (
	"context"
	"net/http"

	"github.com/alexlnos/dsm-mini/internal/store"
)

// settingsView — настройки вместе с тем, что нужно интерфейсу для их показа.
type settingsView struct {
	store.Settings
	// Suggested — папки, которые можно закрепить: папка по умолчанию из
	// Download Station и те, куда недавно качали.
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
	s.log.Info("настройки сохранены", "user", u.ID, "pinned", len(saved.PinnedFolders))
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
		// Настройки — не то, ради чего стоит отказывать в работе: показываем
		// значения по умолчанию и пишем в журнал.
		s.log.Warn("не прочитать настройки", "user", userID, "err", err)
		return store.Defaults()
	}
	return settings
}

// suggestFolders предлагает папки, которые есть смысл закрепить: папку по
// умолчанию из Download Station и те, куда пользователь уже складывал
// загрузки сам.
//
// Папки существующих задач сюда не попадают намеренно: это чужие раздачи и
// давние закачки, к которым новая загрузка отношения не имеет.
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
			s.log.Warn("не прочитать историю папок", "err", err)
		}
		for _, folder := range recent {
			add(folder)
		}
	}
	return out
}

// folderChoices возвращает папки для экрана добавления в том порядке, в каком
// их увидит пользователь: сначала закреплённые, затем, если разрешено,
// недавние.
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
	// Недавние — это собственная история, а не папки задач Download Station.
	if settings.ShowRecent || len(out) == 0 {
		for _, f := range s.suggestFolders(r) {
			add(f)
		}
	}
	return out
}
