package bot

import "github.com/go-telegram/bot/models"

var tgDisabledPreview = models.LinkPreviewOptions{IsDisabled: boolPtr(true)}

func boolPtr(v bool) *bool { return &v }
