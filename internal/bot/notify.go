package bot

import (
	"context"

	tg "github.com/go-telegram/bot"
)

// Notify отправляет сообщение в чат. Реализует watcher.Notifier.
func (b *Bot) Notify(ctx context.Context, chatID int64, text string) error {
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID: chatID,
		Text:   text,
		// Ссылки в названиях раздач разворачивались бы в предпросмотр —
		// в уведомлении это лишний шум.
		LinkPreviewOptions: &tgDisabledPreview,
	})
	return err
}
