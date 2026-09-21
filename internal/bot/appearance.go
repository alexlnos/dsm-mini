package bot

import (
	"context"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/alexlnos/dsm-mini/internal/i18n"
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

// DefaultAppearance — вид бота на заданном языке.
//
// Telegram хранит имя, описание и команды отдельно для каждого языка и
// показывает их по языку клиента. Пустой язык — значения по умолчанию,
// которые видят все, для кого своего перевода нет.
func DefaultAppearance(lang i18n.Lang) Appearance {
	return Appearance{
		Name:             i18n.T(lang, "look.name"),
		ShortDescription: i18n.T(lang, "look.short"),
		Description:      i18n.T(lang, "look.description"),
		MenuButtonText:   i18n.T(lang, "look.menuButton"),
		Commands: []models.BotCommand{
			{Command: "start", Description: i18n.T(lang, "look.cmdStart")},
			{Command: "status", Description: i18n.T(lang, "look.cmdStatus")},
		},
	}
}

// ConfigureAll выставляет вид бота на всех языках словаря.
//
// Telegram хранит имя, описание и команды отдельно для каждого языка и
// показывает их по языку клиента. Значения без языка видят все, для кого
// своего перевода нет.
func (b *Bot) ConfigureAll(ctx context.Context) {
	// Сначала — общий вид: он же запасной для незнакомых языков.
	b.Configure(ctx, "", DefaultAppearance(i18n.Fallback))
	for _, lang := range i18n.Languages() {
		b.Configure(ctx, string(lang), DefaultAppearance(lang))
	}
	// Кнопка меню у Telegram одна на бота, без языков.
	if err := b.setMenuButton(ctx, DefaultAppearance(i18n.Fallback).MenuButtonText); err != nil {
		b.log.Warn("не настроить кнопку меню", "err", err)
	}
	b.log.Info("оформление бота применено", "языков", len(i18n.Languages())+1)
}

// Configure приводит вид бота к заданному для одного языка.
//
// Значения сперва читаются: Telegram резко ограничивает смену имени (после
// нескольких подряд приходит «retry after 60»), а при перезапуске сервиса
// меняться обычно нечему. Заодно это бережёт лимиты при десяти языках.
//
// Ошибки не прерывают запуск: неудавшаяся настройка оформления — повод для
// предупреждения в журнале, а не причина оставить пользователя без бота.
func (b *Bot) Configure(ctx context.Context, lang string, look Appearance) {
	type step struct {
		what    string
		current func() (string, error)
		wanted  string
		apply   func() error
	}

	steps := []step{
		{
			what: "имя",
			current: func() (string, error) {
				n, err := b.api.GetMyName(ctx, &tg.GetMyNameParams{LanguageCode: lang})
				if err != nil {
					return "", err
				}
				return n.Name, nil
			},
			wanted: look.Name,
			apply: func() error {
				_, err := b.api.SetMyName(ctx, &tg.SetMyNameParams{Name: look.Name, LanguageCode: lang})
				return err
			},
		},
		{
			what: "краткое описание",
			current: func() (string, error) {
				d, err := b.api.GetMyShortDescription(ctx, &tg.GetMyShortDescriptionParams{LanguageCode: lang})
				if err != nil {
					return "", err
				}
				return d.ShortDescription, nil
			},
			wanted: look.ShortDescription,
			apply: func() error {
				_, err := b.api.SetMyShortDescription(ctx, &tg.SetMyShortDescriptionParams{
					ShortDescription: look.ShortDescription,
					LanguageCode:     lang,
				})
				return err
			},
		},
		{
			what: "описание",
			current: func() (string, error) {
				d, err := b.api.GetMyDescription(ctx, &tg.GetMyDescriptionParams{LanguageCode: lang})
				if err != nil {
					return "", err
				}
				return d.Description, nil
			},
			wanted: look.Description,
			apply: func() error {
				_, err := b.api.SetMyDescription(ctx, &tg.SetMyDescriptionParams{
					Description:  look.Description,
					LanguageCode: lang,
				})
				return err
			},
		},
		{
			what: "команды",
			current: func() (string, error) {
				cmds, err := b.api.GetMyCommands(ctx, &tg.GetMyCommandsParams{LanguageCode: lang})
				if err != nil {
					return "", err
				}
				return commandsKey(cmds), nil
			},
			wanted: commandsKey(look.Commands),
			apply: func() error {
				_, err := b.api.SetMyCommands(ctx, &tg.SetMyCommandsParams{
					Commands:     look.Commands,
					LanguageCode: lang,
				})
				return err
			},
		},
	}

	for _, s := range steps {
		current, err := s.current()
		if err != nil {
			b.log.Warn("не прочитать оформление бота", "что", s.what, "язык", lang, "err", err)
			continue
		}
		if current == s.wanted {
			continue
		}
		if err := s.apply(); err != nil {
			b.log.Warn("не настроить оформление бота", "что", s.what, "язык", lang, "err", err)
			continue
		}
		b.log.Debug("оформление бота обновлено", "что", s.what, "язык", lang)
	}
}

// commandsKey сводит список команд к строке, чтобы сравнивать одним действием.
func commandsKey(cmds []models.BotCommand) string {
	var sb strings.Builder
	for _, c := range cmds {
		sb.WriteString(c.Command)
		sb.WriteString("=")
		sb.WriteString(c.Description)
		sb.WriteString(";")
	}
	return sb.String()
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
