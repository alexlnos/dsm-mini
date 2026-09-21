package bot

import (
	"context"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Appearance — то, как бот выглядит в Telegram.
//
// Всё это выставляется из кода при каждом запуске, поэтому вид бота живёт
// в репозитории, а не в настройках @BotFather, которые нигде не записаны и
// теряются при смене владельца.
//
// Единственное, чего не даёт Bot API, — аватар: его меняют только вручную
// через @BotFather (/setuserpic).
type Appearance struct {
	// Name — имя в шапке чата, до 64 символов.
	Name string
	// ShortDescription — строка в профиле бота, до 120 символов.
	ShortDescription string
	// Description — текст на пустом экране чата до первого сообщения,
	// до 512 символов.
	Description string
	// MenuButtonText — подпись кнопки, открывающей Mini App.
	MenuButtonText string
	// Commands — меню команд слева от поля ввода.
	Commands []models.BotCommand
}

// DefaultAppearance — вид бота по умолчанию.
//
// Значения на русском: проект собирается под домашний NAS, и владелец обычно
// один. Для другого языка достаточно передать свой Appearance.
func DefaultAppearance() Appearance {
	return Appearance{
		Name:             "Загрузки NAS",
		ShortDescription: "Управление закачками и файлами на домашнем Synology прямо из Telegram.",
		Description: "Пришлите magnet-ссылку или файл .torrent — предложу папку и поставлю в очередь. " +
			"Кнопка ниже открывает приложение: там видно, что качается, сколько осталось " +
			"и что лежит на NAS.",
		MenuButtonText: "Загрузки",
		Commands: []models.BotCommand{
			{Command: "start", Description: "Открыть приложение"},
			{Command: "status", Description: "Что качается сейчас"},
		},
	}
}

// Configure приводит вид бота в Telegram к заданному.
//
// Ошибки не прерывают запуск: неудавшаяся настройка оформления — повод для
// предупреждения в журнале, а не причина оставить пользователя без бота.
// Telegram к тому же отклоняет повторную установку того же значения, и это
// нормально.
func (b *Bot) Configure(ctx context.Context, look Appearance) {
	type step struct {
		what string
		run  func() error
	}

	steps := []step{
		{"имя", func() error {
			_, err := b.api.SetMyName(ctx, &tg.SetMyNameParams{Name: look.Name})
			return err
		}},
		{"краткое описание", func() error {
			_, err := b.api.SetMyShortDescription(ctx, &tg.SetMyShortDescriptionParams{
				ShortDescription: look.ShortDescription,
			})
			return err
		}},
		{"описание", func() error {
			_, err := b.api.SetMyDescription(ctx, &tg.SetMyDescriptionParams{
				Description: look.Description,
			})
			return err
		}},
		{"команды", func() error {
			_, err := b.api.SetMyCommands(ctx, &tg.SetMyCommandsParams{
				Commands: look.Commands,
			})
			return err
		}},
		{"кнопка меню", func() error { return b.setMenuButton(ctx, look.MenuButtonText) }},
	}

	for _, s := range steps {
		if err := s.run(); err != nil {
			b.log.Warn("не настроить оформление бота", "что", s.what, "err", err)
			continue
		}
		b.log.Debug("оформление бота обновлено", "что", s.what)
	}
	b.log.Info("оформление бота применено", "name", look.Name)
}

// setMenuButton вешает на кнопку рядом с полем ввода открытие Mini App.
//
// Без публичного адреса ставить нечего: Telegram принимает в Mini App только
// https, поэтому оставляем обычное меню команд.
func (b *Bot) setMenuButton(ctx context.Context, text string) error {
	if b.publicURL == "" {
		_, err := b.api.SetChatMenuButton(ctx, &tg.SetChatMenuButtonParams{
			MenuButton: models.MenuButtonCommands{Type: "commands"},
		})
		return err
	}
	_, err := b.api.SetChatMenuButton(ctx, &tg.SetChatMenuButtonParams{
		MenuButton: models.MenuButtonWebApp{
			Type:   "web_app",
			Text:   text,
			WebApp: models.WebAppInfo{URL: b.publicURL},
		},
	})
	return err
}
