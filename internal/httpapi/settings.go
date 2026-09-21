package httpapi

import (
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
		Settings:  s.userSettings(u.ID),
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
		s.bad(w, "хранилище настроек недоступно")
		return
	}
	saved, err := s.settings.Set(u.ID, req)
	if err != nil {
		s.fail(w, r, err, "не сохранить настройки")
		return
	}
	s.log.Info("настройки сохранены", "user", u.ID, "pinned", len(saved.PinnedFolders))
	writeJSON(w, http.StatusOK, settingsView{
		Settings:  saved,
		Suggested: s.suggestFolders(r),
	})
}

func (s *Server) userSettings(userID int64) store.Settings {
	if s.settings == nil {
		return store.Defaults()
	}
	return s.settings.Get(userID)
}

// suggestFolders собирает папки, которые есть смысл предложить: папку по
// умолчанию и те, что уже используются задачами.
func (s *Server) suggestFolders(r *http.Request) []string {
	ctx := r.Context()
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
	tasks, err := s.ds.List(ctx)
	if err != nil {
		s.log.Warn("не получить задачи для списка папок", "err", err)
		return out
	}
	// С конца: недавние задачи интереснее старых.
	for i := len(tasks) - 1; i >= 0 && len(out) < 12; i-- {
		add(tasks[i].Destination)
	}
	return out
}

// folderChoices возвращает папки для экрана добавления в том порядке, в каком
// их увидит пользователь: сначала закреплённые, затем, если разрешено,
// недавние.
func (s *Server) folderChoices(r *http.Request, userID int64) []string {
	settings := s.userSettings(userID)

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
	if settings.ShowRecent || len(out) == 0 {
		for _, f := range s.suggestFolders(r) {
			add(f)
		}
	}
	return out
}
