package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

// authTTL — как долго initData считается свежей.
//
// Telegram подписывает её один раз при открытии Mini App, а окно живёт
// часами. Сутки — компромисс между удобством и тем, чтобы перехваченная
// строка не работала вечно.
const authTTL = 24 * time.Hour

type ctxKey int

const userKey ctxKey = iota

// user — кто открыл Mini App.
type user struct {
	ID        int64
	Username  string
	FirstName string
	Language  string
}

// userFrom достаёт пользователя, которого положил authMiddleware.
func userFrom(ctx context.Context) (user, bool) {
	u, ok := ctx.Value(userKey).(user)
	return u, ok
}

// authMiddleware пускает дальше только запросы с подлинной initData от
// разрешённого пользователя.
//
// Проверяется две вещи, и обе обязательны:
//
//  1. Подпись initData ключом, производным от токена бота. Она доказывает,
//     что строку выдал Telegram, а не подделал кто угодно, открывший адрес.
//  2. Идентификатор пользователя в списке разрешённых. Подпись сама по себе
//     подтверждает лишь то, что человек открыл бота, — а бота может открыть
//     любой.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := initDataFrom(r)
		if raw == "" {
			s.deny(w, r, http.StatusUnauthorized, "нет данных авторизации Telegram", "отсутствует initData")
			return
		}

		if err := initdata.Validate(raw, s.botToken, authTTL); err != nil {
			// Причину наружу не отдаём: она подсказывала бы, как подобрать подпись.
			s.deny(w, r, http.StatusUnauthorized, "данные авторизации недействительны", "подпись initData не прошла: "+err.Error())
			return
		}

		parsed, err := initdata.Parse(raw)
		if err != nil {
			s.deny(w, r, http.StatusUnauthorized, "данные авторизации недействительны", "не разобрать initData: "+err.Error())
			return
		}
		if parsed.User.ID == 0 {
			s.deny(w, r, http.StatusUnauthorized, "данные авторизации недействительны", "в initData нет пользователя")
			return
		}

		if !s.isAllowed(parsed.User.ID) {
			s.deny(w, r, http.StatusForbidden, "доступ закрыт",
				"пользователь вне списка разрешённых")
			return
		}

		u := user{
			ID:        parsed.User.ID,
			Username:  parsed.User.Username,
			FirstName: parsed.User.FirstName,
			Language:  parsed.User.LanguageCode,
		}
		// Запоминаем язык: уведомления о завершённых задачах уходят сами, и
		// спросить его в тот момент не у кого.
		if s.settings != nil {
			if err := s.settings.RememberLanguage(r.Context(), u.ID, u.Language); err != nil {
				s.log.Warn("не запомнить язык", "user", u.ID, "err", err)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

// initDataFrom читает initData из заголовка, а при его отсутствии — из строки
// запроса.
//
// Основной способ — заголовок `Authorization: tma <initData>`: так строка не
// попадает в журналы прокси и в историю браузера.
func initDataFrom(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if rest, ok := strings.CutPrefix(h, "tma "); ok {
			return strings.TrimSpace(rest)
		}
	}
	if h := r.Header.Get("X-Telegram-Init-Data"); h != "" {
		return h
	}
	return r.URL.Query().Get("_auth")
}

// deny отвечает отказом: в журнал — подробности, наружу — общая фраза.
func (s *Server) deny(w http.ResponseWriter, r *http.Request, status int, public, detail string) {
	s.log.Warn("запрос отклонён",
		"path", r.URL.Path,
		"status", status,
		"reason", detail,
		"remote", remoteAddr(r))
	writeJSON(w, status, map[string]string{"error": public})
}

func remoteAddr(r *http.Request) string {
	// За reverse proxy DSM реальный адрес приходит в заголовке.
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("не отправить ответ", "err", err)
	}
}
