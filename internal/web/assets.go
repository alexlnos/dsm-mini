// Package web встраивает собранное Mini App в бинарник.
//
// Файлы появляются здесь после сборки фронтенда: `npm run build` в каталоге
// web/ кладёт результат в internal/web/dist. Без них сервис работает как
// обычный бот, только без Mini App.
package web

import (
	"embed"
	"errors"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// ErrNotBuilt означает, что фронтенд не собран.
var ErrNotBuilt = errors.New("Mini App не собрано: выполните сборку в каталоге web/")

// Assets возвращает файловую систему с собранным приложением.
func Assets() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, ErrNotBuilt
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, ErrNotBuilt
	}
	return sub, nil
}
