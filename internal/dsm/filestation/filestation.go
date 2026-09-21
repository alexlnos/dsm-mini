// Package filestation — работа с файлами на NAS через SYNO.FileStation.*.
//
// Главная особенность: File Station НЕ принимает limit = -1. В отличие от
// Download Station, где это штатный способ получить всё разом, здесь такой
// запрос роняет обработчик в HTTP 502 — на любой версии API. Поэтому списки
// читаются страницами. Подробности в docs/synology-api.md.
package filestation

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// pageSize — размер страницы при чтении списков.
//
// Значение -1 («всё разом»), привычное по Download Station, File Station не
// принимает: запрос падает с HTTP 502 на любой версии API.
const pageSize = 500

const (
	apiList     = "SYNO.FileStation.List"
	apiDelete   = "SYNO.FileStation.Delete"
	apiRename   = "SYNO.FileStation.Rename"
	apiUpload   = "SYNO.FileStation.Upload"
	apiCreate   = "SYNO.FileStation.CreateFolder"
	apiCoreShar = "SYNO.Core.Share"
)

// Entry — файл или папка.
type Entry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified,omitzero"`
}

// Station — операции над файлами NAS.
type Station struct {
	c *dsm.Client
}

// New создаёт клиент File Station.
func New(c *dsm.Client) *Station { return &Station{c: c} }

type rawFile struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	IsDir      bool   `json:"isdir"`
	Additional struct {
		Size int64 `json:"size"`
		Time struct {
			Mtime int64 `json:"mtime"`
		} `json:"time"`
	} `json:"additional"`
}

func (f rawFile) toEntry() Entry {
	e := Entry{Name: f.Name, Path: f.Path, IsDir: f.IsDir, Size: f.Additional.Size}
	if f.Additional.Time.Mtime > 0 {
		e.Modified = time.Unix(f.Additional.Time.Mtime, 0)
	}
	return e
}

// Shares перечисляет общие папки — верхний уровень файлового дерева.
//
// Если у File Station сломан list_share, берём список из SYNO.Core.Share:
// он отдаёт те же папки и требует лишь прав администратора.
func (s *Station) Shares(ctx context.Context) ([]Entry, error) {
	var out struct {
		Shares []rawFile `json:"shares"`
	}
	err := s.c.CallVersioned(ctx, apiList, "list_share", []int{2, 1}, map[string]any{
		"limit":      pageSize,
		"offset":     0,
		"additional": []string{"size", "time"},
	}, &out)
	if err == nil {
		return toEntries(out.Shares, true), nil
	}

	var fallback struct {
		Shares []struct {
			Name string `json:"name"`
		} `json:"shares"`
	}
	if e := s.c.Call(ctx, apiCoreShar, "list", 1, map[string]any{
		"limit": pageSize, "offset": 0,
	}, &fallback); e != nil {
		// Сообщаем исходную причину: она полезнее ошибки запасного пути.
		return nil, fmt.Errorf("не получить список общих папок: %w", err)
	}

	entries := make([]Entry, 0, len(fallback.Shares))
	for _, sh := range fallback.Shares {
		entries = append(entries, Entry{Name: sh.Name, Path: "/" + sh.Name, IsDir: true})
	}
	return entries, nil
}

// List перечисляет содержимое папки. folder — путь от корня общей папки,
// например "/Media/Music".
func (s *Station) List(ctx context.Context, folder string) ([]Entry, error) {
	folder = normalize(folder)
	if folder == "/" {
		return s.Shares(ctx)
	}

	// Читаем страницами: limit = -1 здесь недопустим, а папки на домашнем
	// NAS легко перешагивают тысячу файлов.
	var files []rawFile
	for offset := 0; ; {
		var out struct {
			Total int       `json:"total"`
			Files []rawFile `json:"files"`
		}
		err := s.c.CallVersioned(ctx, apiList, "list", []int{2, 1}, map[string]any{
			"folder_path": folder,
			"limit":       pageSize,
			"offset":      offset,
			"additional":  []string{"size", "time"},
			"sort_by":     "name",
		}, &out)
		if err != nil {
			return nil, err
		}
		files = append(files, out.Files...)
		offset += len(out.Files)
		if len(out.Files) == 0 || offset >= out.Total {
			break
		}
	}

	entries := toEntries(files, false)
	// Папки выше файлов — так привычнее и совпадает с File Station.
	dirs := make([]Entry, 0, len(entries))
	plain := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
		} else {
			plain = append(plain, e)
		}
	}
	return append(dirs, plain...), nil
}

// Delete удаляет файлы и папки безвозвратно.
//
// Используется синхронный вариант: на домашних объёмах он проще, а
// асинхронный (start/status/stop с taskid) нужен для очень больших деревьев.
func (s *Station) Delete(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("не указано ни одного пути")
	}
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		clean = append(clean, normalize(p))
	}
	return s.c.CallVersioned(ctx, apiDelete, "delete", []int{2, 1}, map[string]any{
		"path":      clean,
		"recursive": true,
	}, nil)
}

// Rename переименовывает файл или папку. Возвращает новый путь.
func (s *Station) Rename(ctx context.Context, target, newName string) (string, error) {
	if strings.ContainsAny(newName, `/\`) {
		return "", fmt.Errorf("имя не может содержать разделители пути")
	}
	if newName == "" || newName == "." || newName == ".." {
		return "", fmt.Errorf("недопустимое имя")
	}
	target = normalize(target)

	// path и name здесь — массивы даже для одного элемента.
	err := s.c.CallVersioned(ctx, apiRename, "rename", []int{2, 1}, map[string]any{
		"path": []string{target},
		"name": []string{newName},
	}, nil)
	if err != nil {
		return "", err
	}
	return path.Join(path.Dir(target), newName), nil
}

// CreateFolder создаёт папку внутри parent.
func (s *Station) CreateFolder(ctx context.Context, parent, name string) (string, error) {
	if strings.ContainsAny(name, `/\`) || name == "" {
		return "", fmt.Errorf("недопустимое имя папки")
	}
	parent = normalize(parent)
	err := s.c.CallVersioned(ctx, apiCreate, "create", []int{2, 1}, map[string]any{
		"folder_path":  []string{parent},
		"name":         []string{name},
		"force_parent": false,
	}, nil)
	if err != nil {
		return "", err
	}
	return path.Join(parent, name), nil
}

// Upload кладёт файл в папку на NAS.
//
// overwrite=false означает «пропустить, если файл уже есть»: без явного
// значения DSM отвечает ошибкой при совпадении имени.
func (s *Station) Upload(ctx context.Context, folder, name string, data []byte, overwrite bool) error {
	if name == "" {
		return fmt.Errorf("не указано имя файла")
	}
	if len(data) == 0 {
		return fmt.Errorf("пустой файл")
	}
	folder = normalize(folder)
	if folder == "/" {
		return fmt.Errorf("нельзя загружать в корень: укажите общую папку")
	}

	// В multipart значения идут как есть, без JSON-кавычек.
	return s.c.CallUpload(ctx, apiUpload, "upload", 2, map[string]string{
		"path":           folder,
		"create_parents": "false",
		"overwrite":      strconv.FormatBool(overwrite),
	}, dsm.UploadFile{Field: "file", Name: name, Data: data}, nil)
}

func toEntries(raw []rawFile, dirs bool) []Entry {
	out := make([]Entry, 0, len(raw))
	for _, f := range raw {
		e := f.toEntry()
		if dirs {
			e.IsDir = true
		}
		out = append(out, e)
	}
	return out
}

// normalize приводит путь к виду, который понимает File Station: с ведущим
// слешем и без завершающего. В отличие от Download Station, где путь идёт
// без ведущего слеша, здесь он обязателен.
func normalize(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}
