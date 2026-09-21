package bot

import (
	"context"

	tg "github.com/go-telegram/bot"
)

// Notify sends a message to a chat. Implements watcher.Notifier.
func (b *Bot) Notify(ctx context.Context, chatID int64, text string) error {
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID: chatID,
		Text:   text,
		// Links inside torrent names would expand into a preview — needless
		// noise in a notification.
		LinkPreviewOptions: &tgDisabledPreview,
	})
	return err
}
