package bot

import (
	"context"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/alexlnos/dsm-mini/internal/i18n"
)

// Appearance is how the bot looks in Telegram.
//
// All of it is set from code on every start, so the bot's looks live in the
// repository rather than in @BotFather settings, which are written down
// nowhere and get lost when the owner changes.
//
// The only thing the Bot API does not offer is the avatar: it is changed by
// hand through @BotFather (/setuserpic).
type Appearance struct {
	// Name is the name in the chat header, up to 64 characters.
	Name string
	// ShortDescription is the line in the bot profile, up to 120 characters.
	ShortDescription string
	// Description is the text on an empty chat screen before the first
	// message, up to 512 characters.
	Description string
	// MenuButtonText is the label of the button that opens the Mini App.
	MenuButtonText string
	// Commands is the command menu left of the input field.
	Commands []models.BotCommand
}

// DefaultAppearance is the bot's looks in a given language.
//
// Telegram stores the name, description and commands separately per language
// and shows them by the client's language. An empty language means the
// defaults, seen by everyone who has no translation of their own.
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

// ConfigureAll sets the bot's looks in every language of the dictionary.
//
// Telegram stores the name, description and commands separately per language
// and shows them by the client's language. Values with no language are seen
// by everyone who has no translation of their own.
func (b *Bot) ConfigureAll(ctx context.Context) {
	// The common looks first: they double as the fallback for unknown languages.
	b.Configure(ctx, "", DefaultAppearance(i18n.Fallback))
	for _, lang := range i18n.Languages() {
		b.Configure(ctx, string(lang), DefaultAppearance(lang))
	}
	// Telegram has one menu button per bot, with no languages.
	if err := b.setMenuButton(ctx, DefaultAppearance(i18n.Fallback).MenuButtonText); err != nil {
		b.log.Warn("cannot set the menu button", "err", err)
	}
	b.log.Info("bot appearance applied", "languages", len(i18n.Languages())+1)
}

// Configure brings the bot's looks to the given ones for a single language.
//
// Values are read first: Telegram limits name changes harshly (after a few
// in a row it answers "retry after 60"), and on a service restart there is
// usually nothing to change. It also saves the rate limits across ten languages.
//
// Errors do not stop the startup: a failed appearance setting is worth a
// warning in the log, not leaving the user without a bot.
func (b *Bot) Configure(ctx context.Context, lang string, look Appearance) {
	type step struct {
		what    string
		current func() (string, error)
		wanted  string
		apply   func() error
	}

	steps := []step{
		{
			what: "name",
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
			what: "short description",
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
			what: "description",
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
			what: "commands",
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
			b.log.Warn("cannot read the bot appearance", "what", s.what, "lang", lang, "err", err)
			continue
		}
		if current == s.wanted {
			continue
		}
		if err := s.apply(); err != nil {
			b.log.Warn("cannot set the bot appearance", "what", s.what, "lang", lang, "err", err)
			continue
		}
		b.log.Debug("bot appearance updated", "what", s.what, "lang", lang)
	}
}

// commandsKey folds the command list into a string to compare in one step.
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

// setMenuButton makes the button next to the input field open the Mini App.
//
// With no public address there is nothing to set: Telegram accepts only https
// for a Mini App, so we leave the plain command menu.
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
