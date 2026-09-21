//go:build integration

package filestation

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

func newClient(t *testing.T) *dsm.Client {
	t.Helper()
	url, user, pass := os.Getenv("DSM_URL"), os.Getenv("DSM_USER"), os.Getenv("DSM_PASSWORD")
	if url == "" || user == "" || pass == "" {
		t.Skip("не заданы DSM_URL / DSM_USER / DSM_PASSWORD")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("DSM_INSECURE_TLS"))
	c := dsm.New(dsm.Options{BaseURL: url, User: user, Password: pass,
		InsecureTLS: insecure, Timeout: 30 * time.Second})
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })
	return c
}

// TestBrowse проверяет чтение дерева. Ничего не меняет.
func TestBrowse(t *testing.T) {
	ctx := context.Background()
	st := New(newClient(t))

	shares, err := st.Shares(ctx)
	if err != nil {
		t.Fatalf("общие папки: %v", err)
	}
	if len(shares) == 0 {
		t.Fatal("список общих папок пуст")
	}
	t.Logf("общих папок: %d", len(shares))
	for _, s := range shares {
		t.Logf("  %s", s.Path)
	}

	// Ищем первую непустую папку, в которую пускают: пустая ничего не
	// докажет про разбор ответа.
	var listed bool
	for _, s := range shares {
		entries, err := st.List(ctx, s.Path)
		if err != nil {
			t.Logf("  %s — пропускаем: %v", s.Path, err)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		listed = true
		t.Logf("%s: %d объектов", s.Path, len(entries))
		for i, e := range entries {
			if i >= 5 {
				break
			}
			kind := "файл"
			if e.IsDir {
				kind = "папка"
			}
			t.Logf("  %-5s %-40s %d Б", kind, e.Name, e.Size)
		}
		break
	}
	if !listed {
		t.Error("ни одну общую папку не удалось прочитать")
	}
}

// TestRootListsShares: пустой путь и "/" дают список общих папок.
func TestRootListsShares(t *testing.T) {
	ctx := context.Background()
	st := New(newClient(t))
	for _, p := range []string{"", "/"} {
		entries, err := st.List(ctx, p)
		if err != nil {
			t.Fatalf("список для %q: %v", p, err)
		}
		if len(entries) == 0 {
			t.Errorf("для %q список пуст", p)
		}
	}
}

// TestFileLifecycle проверяет запись: создание папки, загрузку файла,
// переименование и удаление. Меняет состояние NAS, поэтому требует
// DSM_TEST_MUTATIONS=1 и работает во временной папке, которую сам и убирает.
func TestFileLifecycle(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("тест меняет файлы на NAS; запуск только с DSM_TEST_MUTATIONS=1")
	}
	ctx := context.Background()
	st := New(newClient(t))

	parent := os.Getenv("DSM_TEST_FOLDER")
	if parent == "" {
		t.Skip("не задан DSM_TEST_FOLDER — папка, внутри которой можно создавать временные файлы")
	}

	name := fmt.Sprintf("dsm-mini-test-%d", time.Now().Unix())
	dir, err := st.CreateFolder(ctx, parent, name)
	if err != nil {
		t.Fatalf("создание папки: %v", err)
	}
	t.Logf("создана папка %s", dir)
	t.Cleanup(func() {
		if err := st.Delete(context.Background(), []string{dir}); err != nil {
			t.Errorf("УБОРКА НЕ УДАЛАСЬ, удалите вручную %s: %v", dir, err)
			return
		}
		t.Logf("папка %s удалена", dir)
	})

	content := []byte("dsm-mini integration test\n")
	if err := st.Upload(ctx, dir, "hello.txt", content, true); err != nil {
		t.Fatalf("загрузка файла: %v", err)
	}

	entries, err := st.List(ctx, dir)
	if err != nil {
		t.Fatalf("список после загрузки: %v", err)
	}
	var found *Entry
	for i := range entries {
		if entries[i].Name == "hello.txt" {
			found = &entries[i]
		}
	}
	if found == nil {
		t.Fatalf("загруженный файл не появился в %s", dir)
	}
	if found.Size != int64(len(content)) {
		t.Errorf("размер файла %d, ожидался %d", found.Size, len(content))
	}
	t.Logf("файл загружен: %s, %d Б", found.Path, found.Size)

	renamed, err := st.Rename(ctx, found.Path, "renamed.txt")
	if err != nil {
		t.Fatalf("переименование: %v", err)
	}
	t.Logf("переименован в %s", renamed)

	entries, err = st.List(ctx, dir)
	if err != nil {
		t.Fatalf("список после переименования: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "renamed.txt" {
		t.Errorf("после переименования в папке: %+v", entries)
	}
}

// TestRenameRejectsSeparators: имя с разделителем не должно уходить на NAS.
func TestRenameRejectsSeparators(t *testing.T) {
	ctx := context.Background()
	st := New(newClient(t))
	if _, err := st.Rename(ctx, "/Media/x", "../evil"); err == nil {
		t.Error("имя с разделителем пути должно отклоняться")
	}
}
