//go:build integration

package filestation

import (
	"context"
	"fmt"
	"io"
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

// TestCopyMovePreview проверяет копирование, перенос и предпросмотр на живом
// NAS. Меняет файлы: нужен DSM_TEST_MUTATIONS=1 и DSM_TEST_FOLDER.
func TestCopyMovePreview(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("тест меняет файлы на NAS; запуск только с DSM_TEST_MUTATIONS=1")
	}
	parent := os.Getenv("DSM_TEST_FOLDER")
	if parent == "" {
		t.Skip("не задан DSM_TEST_FOLDER")
	}

	ctx := context.Background()
	st := New(newClient(t))

	root, err := st.CreateFolder(ctx, parent, fmt.Sprintf("dsm-mini-fs-%d", time.Now().Unix()))
	if err != nil {
		t.Fatalf("создание рабочей папки: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Delete(context.Background(), []string{root}); err != nil {
			t.Errorf("УБОРКА НЕ УДАЛАСЬ, удалите %s вручную: %v", root, err)
		}
	})

	src, err := st.CreateFolder(ctx, root, "src")
	if err != nil {
		t.Fatalf("папка источника: %v", err)
	}
	dst, err := st.CreateFolder(ctx, root, "dst")
	if err != nil {
		t.Fatalf("папка приёмника: %v", err)
	}

	content := []byte("привет из dsm-mini\nвторая строка\n")
	if err := st.Upload(ctx, src, "note.txt", content, true); err != nil {
		t.Fatalf("загрузка: %v", err)
	}

	// Копирование.
	taskID, err := st.Copy(ctx, []string{src + "/note.txt"}, dst, true)
	if err != nil {
		t.Fatalf("копирование: %v", err)
	}
	if err := waitTransfer(ctx, t, st, taskID); err != nil {
		t.Fatalf("копирование не завершилось: %v", err)
	}
	if !hasFile(ctx, t, st, dst, "note.txt") {
		t.Error("копия не появилась в папке назначения")
	}
	if !hasFile(ctx, t, st, src, "note.txt") {
		t.Error("при копировании исчез оригинал")
	}
	t.Log("копирование прошло")

	// Перенос: исходник должен опустеть.
	if err := st.Upload(ctx, src, "moved.txt", content, true); err != nil {
		t.Fatalf("загрузка для переноса: %v", err)
	}
	taskID, err = st.Move(ctx, []string{src + "/moved.txt"}, dst, true)
	if err != nil {
		t.Fatalf("перенос: %v", err)
	}
	if err := waitTransfer(ctx, t, st, taskID); err != nil {
		t.Fatalf("перенос не завершился: %v", err)
	}
	if hasFile(ctx, t, st, src, "moved.txt") {
		t.Error("после переноса файл остался в источнике")
	}
	if !hasFile(ctx, t, st, dst, "moved.txt") {
		t.Error("перенесённый файл не появился в приёмнике")
	}
	t.Log("перенос прошёл")

	// Защита от переноса папки внутрь себя.
	if _, err := st.Move(ctx, []string{src}, src+"/внутрь", true); err == nil {
		t.Error("перенос папки внутрь себя должен отклоняться")
	}

	// Предпросмотр: содержимое читается обратно.
	preview, err := st.Download(ctx, dst+"/note.txt")
	if err != nil {
		t.Fatalf("чтение файла: %v", err)
	}
	defer preview.Body.Close()
	got, err := io.ReadAll(preview.Body)
	if err != nil {
		t.Fatalf("чтение тела: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("содержимое не совпало: %q", string(got))
	}
	t.Logf("предпросмотр: %s, %d байт", preview.ContentType, len(got))

	// Миниатюра текстового файла невозможна — это не сбой.
	if _, err := st.Thumbnail(ctx, dst+"/note.txt", ThumbSmall); err == nil {
		t.Log("NAS неожиданно отдал миниатюру текстового файла")
	} else {
		t.Logf("миниатюра текста недоступна, как и ожидалось: %v", err)
	}
}

func waitTransfer(ctx context.Context, t *testing.T, st *Station, taskID string) error {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		status, err := st.Status(ctx, taskID)
		if err != nil {
			return err
		}
		if status.Finished {
			t.Logf("задача %s завершена, пропущено=%v", taskID, status.Skipped)
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("задача %s не завершилась за минуту", taskID)
}

func hasFile(ctx context.Context, t *testing.T, st *Station, folder, name string) bool {
	t.Helper()
	entries, err := st.List(ctx, folder)
	if err != nil {
		t.Fatalf("список %s: %v", folder, err)
	}
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}
