//go:build integration

package downloadstation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// torrentURL — официальный торрент Ubuntu Server: легальный, с живыми сидами
// и стабильным адресом. Задача создаётся и тут же удаляется, скачаться ничего
// не успевает.
const torrentURL = "https://releases.ubuntu.com/24.04/ubuntu-24.04.5-live-server-amd64.iso.torrent"

// TestCreateAndDelete проверяет полный цикл управления задачей на живом NAS.
//
// Тест ИЗМЕНЯЕТ состояние Download Station, поэтому требует отдельного
// разрешения через DSM_TEST_MUTATIONS=1 — одного тега integration мало.
// Созданная задача удаляется в t.Cleanup даже при падении теста.
func TestCreateAndDelete(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("тест меняет состояние NAS; запуск только с DSM_TEST_MUTATIONS=1")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	for _, gen := range []Generation{GenerationV2, GenerationLegacy} {
		t.Run(string(gen), func(t *testing.T) {
			st, err := NewWithGeneration(ctx, c, gen)
			if err != nil {
				t.Fatalf("создание: %v", err)
			}

			// Кладём в папку по умолчанию, но передаём её ЯВНО: именно на
			// явном destination проявляется разница в кавычировании между
			// поколениями API. Новых папок при этом не появляется.
			dest, err := st.DefaultDestination(ctx)
			if err != nil {
				t.Fatalf("папка по умолчанию: %v", err)
			}

			before, err := taskIDs(ctx, st)
			if err != nil {
				t.Fatalf("список до создания: %v", err)
			}

			if err := st.Create(ctx, CreateRequest{URLs: []string{torrentURL}, Destination: dest}); err != nil {
				t.Fatalf("создание задачи: %v", err)
			}

			task, err := waitForNewTask(ctx, t, st, before)
			if err != nil {
				t.Fatalf("новая задача не появилась: %v", err)
			}

			// Удаляем при любом исходе проверок ниже.
			t.Cleanup(func() {
				if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
					t.Errorf("УБОРКА НЕ УДАЛАСЬ, задачу %s (%s) надо удалить вручную: %v",
						task.ID, task.Title, err)
					return
				}
				t.Logf("задача %s удалена", task.ID)
			})

			t.Logf("создана %s | %s | статус %s | папка %q",
				task.ID, task.Title, task.Status, task.Destination)

			if task.Destination != dest {
				t.Errorf("папка назначения %q, ожидалась %q — проверьте кавычирование destination",
					task.Destination, dest)
			}
			// Поколения расходятся: v2 распознаёт .torrent по ссылке и сразу
			// заводит bt-задачу, легаси сначала качает сам файл как https.
			t.Logf("тип задачи %q", task.Type)

			if err := st.Pause(ctx, []string{task.ID}); err != nil {
				t.Errorf("пауза: %v", err)
			} else if got := waitForStatus(ctx, st, task.ID, StatusPaused); got != StatusPaused {
				t.Errorf("после паузы статус %q, ожидался paused", got)
			} else {
				t.Log("пауза сработала")
			}

			if err := st.Resume(ctx, []string{task.ID}); err != nil {
				t.Errorf("возобновление: %v", err)
			} else {
				t.Logf("возобновление сработало, статус %q",
					waitForStatus(ctx, st, task.ID, StatusDownloading))
			}
		})
	}

	// После всех уборок задач быть не должно.
	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("проверка после уборки: %v", err)
	}
	tasks, err := st.List(ctx)
	if err != nil {
		t.Fatalf("список после уборки: %v", err)
	}
	for _, task := range tasks {
		if task.Title != "" && contains(task.Title, "ubuntu-24.04") {
			t.Errorf("осталась тестовая задача %s (%s) — удалите вручную", task.ID, task.Title)
		}
	}
}

func taskIDs(ctx context.Context, st Station) (map[string]bool, error) {
	tasks, err := st.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		ids[t.ID] = true
	}
	return ids, nil
}

// waitForNewTask ждёт задачу, которой не было до создания. Download Station
// сначала скачивает сам .torrent и только потом заводит задачу, так что
// мгновенного появления ждать нельзя.
func waitForNewTask(ctx context.Context, t *testing.T, st Station, before map[string]bool) (Task, error) {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		tasks, err := st.List(ctx)
		if err != nil {
			return Task{}, err
		}
		for _, task := range tasks {
			if !before[task.ID] {
				return task, nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return Task{}, context.DeadlineExceeded
}

func waitForStatus(ctx context.Context, st Station, id string, want Status) Status {
	var last Status
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		tasks, err := st.List(ctx)
		if err != nil {
			return last
		}
		for _, task := range tasks {
			if task.ID == id {
				last = task.Status
				if task.Status == want {
					return want
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	return last
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestCreateFromTorrentFile ставит задачу содержимым .torrent — путь, которым
// пойдёт бот, когда файл присылают прямо в чат.
//
// Меняет состояние NAS: требует DSM_TEST_MUTATIONS=1, задачу удаляет за собой.
func TestCreateFromTorrentFile(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("тест меняет состояние NAS; запуск только с DSM_TEST_MUTATIONS=1")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	torrent := fetchTorrent(t)
	t.Logf("торрент получен, %d Б", len(torrent))

	for _, gen := range []Generation{GenerationV2, GenerationLegacy} {
		t.Run(string(gen), func(t *testing.T) {
			st, err := NewWithGeneration(ctx, c, gen)
			if err != nil {
				t.Fatalf("создание: %v", err)
			}
			dest, err := st.DefaultDestination(ctx)
			if err != nil {
				t.Fatalf("папка по умолчанию: %v", err)
			}

			before, err := taskIDs(ctx, st)
			if err != nil {
				t.Fatalf("список до: %v", err)
			}

			err = st.Create(ctx, CreateRequest{
				TorrentFile: torrent,
				FileName:    "ubuntu-24.04.5-live-server-amd64.iso.torrent",
				Destination: dest,
			})
			if err != nil {
				// На DSM 7 легаси-обработчик загрузки файла отвечает 101 на
				// всех трёх своих версиях: его заменил DownloadStation2.
				// Ветка остаётся для DSM 6, где проверить её нечем.
				var apiErr *dsm.APIError
				if gen == GenerationLegacy && errors.As(err, &apiErr) && apiErr.Code == 101 {
					t.Skipf("легаси-загрузка файла недоступна на этом DSM (код 101) — ожидаемо для DSM 7")
				}
				t.Fatalf("создание из файла: %v", err)
			}

			task, err := waitForNewTask(ctx, t, st, before)
			if err != nil {
				t.Fatalf("задача не появилась: %v", err)
			}
			t.Cleanup(func() {
				if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
					t.Errorf("УБОРКА НЕ УДАЛАСЬ, удалите %s вручную: %v", task.ID, err)
					return
				}
				t.Logf("задача %s удалена", task.ID)
			})

			t.Logf("создана %s | %s | тип %s | папка %q", task.ID, task.Title, task.Type, task.Destination)
			// Файл .torrent разбирается на NAS, поэтому задача сразу
			// торрентная — в отличие от постановки по ссылке на тот же файл.
			if task.Type != "bt" {
				t.Errorf("тип задачи %q, ожидался bt", task.Type)
			}
			if task.Destination != dest {
				t.Errorf("папка %q, ожидалась %q", task.Destination, dest)
			}
		})
	}
}

func fetchTorrent(t *testing.T) []byte {
	t.Helper()
	if p := os.Getenv("DSM_TEST_TORRENT"); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("не прочитать %s: %v", p, err)
		}
		return data
	}
	req, err := http.NewRequest(http.MethodGet, torrentURL, nil)
	if err != nil {
		t.Fatalf("запрос: %v", err)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		t.Skipf("не скачать торрент (нет интернета?): %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("чтение торрента: %v", err)
	}
	return data
}

// TestCreateFromMagnet ставит задачу из magnet-ссылки, заданной в
// DSM_TEST_MAGNET, и удаляет её. Нужен, чтобы проверить путь целиком на
// настоящей ссылке, а не на торрент-файле по HTTP.
func TestCreateFromMagnet(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("тест меняет состояние NAS; запуск только с DSM_TEST_MUTATIONS=1")
	}
	magnet := os.Getenv("DSM_TEST_MAGNET")
	if magnet == "" {
		t.Skip("не задан DSM_TEST_MAGNET")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("создание: %v", err)
	}
	dest := os.Getenv("DSM_TEST_FOLDER_DS")
	if dest == "" {
		if dest, err = st.DefaultDestination(ctx); err != nil {
			t.Fatalf("папка по умолчанию: %v", err)
		}
	}

	before, err := taskIDs(ctx, st)
	if err != nil {
		t.Fatalf("список до: %v", err)
	}

	if err := st.Create(ctx, CreateRequest{URLs: []string{magnet}, Destination: dest}); err != nil {
		t.Fatalf("создание из magnet: %v", err)
	}

	task, err := waitForNewTask(ctx, t, st, before)
	if err != nil {
		t.Fatalf("задача не появилась: %v", err)
	}

	keep := os.Getenv("DSM_TEST_KEEP") == "1"
	if !keep {
		t.Cleanup(func() {
			if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
				t.Errorf("УБОРКА НЕ УДАЛАСЬ, удалите %s вручную: %v", task.ID, err)
				return
			}
			t.Logf("задача %s удалена", task.ID)
		})
	} else {
		t.Logf("DSM_TEST_KEEP=1 — задача %s оставлена качаться", task.ID)
	}

	t.Logf("создана %s", task.ID)
	t.Logf("  название: %s", task.Title)
	t.Logf("  тип: %s, статус: %s, размер: %d Б", task.Type, task.Status, task.Size)
	t.Logf("  папка: %s", task.Destination)

	if task.Type != "bt" {
		t.Errorf("тип %q, ожидался bt", task.Type)
	}
	if task.Destination != dest {
		t.Errorf("папка %q, ожидалась %q", task.Destination, dest)
	}
}

// TestActiveTaskDetails проверяет всё, что доступно только у работающей
// задачи: список файлов, приоритеты, трекеры и смену папки.
//
// Задача создаётся, проверяется и удаляется. Требует DSM_TEST_MUTATIONS=1.
func TestActiveTaskDetails(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("тест меняет состояние NAS; запуск только с DSM_TEST_MUTATIONS=1")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("создание: %v", err)
	}
	if st.Generation() != "v2" {
		t.Skip("файлы и приоритеты доступны только через DownloadStation2")
	}

	dest, err := st.DefaultDestination(ctx)
	if err != nil {
		t.Fatalf("папка по умолчанию: %v", err)
	}
	before, err := taskIDs(ctx, st)
	if err != nil {
		t.Fatalf("список до: %v", err)
	}
	if err := st.Create(ctx, CreateRequest{
		TorrentFile: fetchTorrent(t),
		FileName:    "test.torrent",
		Destination: dest,
	}); err != nil {
		t.Fatalf("создание задачи: %v", err)
	}

	task, err := waitForNewTask(ctx, t, st, before)
	if err != nil {
		t.Fatalf("задача не появилась: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
			t.Errorf("УБОРКА НЕ УДАЛАСЬ, удалите %s вручную: %v", task.ID, err)
		}
	})

	// Файлы появляются не сразу: сначала NAS проверяет хеши.
	var files []File
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		files, err = st.Files(ctx, task.ID)
		if err == nil && len(files) > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		t.Fatalf("файлы задачи: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("список файлов пуст")
	}
	t.Logf("файлов: %d", len(files))
	for _, f := range files {
		t.Logf("  [%d] %s — %d Б, приоритет %s, качать=%v",
			f.Index, f.Name, f.Size, f.Priority, f.Wanted)
	}

	// Приоритет и признак «качать».
	if err := st.SetFile(ctx, task.ID, []int{files[0].Index}, PriorityLow, nil); err != nil {
		t.Fatalf("смена приоритета файла: %v", err)
	}
	no := false
	if err := st.SetFile(ctx, task.ID, []int{files[0].Index}, "", &no); err != nil {
		t.Fatalf("снятие признака wanted: %v", err)
	}

	updated, err := st.Files(ctx, task.ID)
	if err != nil {
		t.Fatalf("файлы после изменения: %v", err)
	}
	if updated[0].Priority != PriorityLow {
		t.Errorf("приоритет %q, ожидался low", updated[0].Priority)
	}
	if updated[0].Wanted {
		t.Error("файл остался в списке на загрузку")
	}
	t.Logf("после изменения: приоритет %s, качать=%v", updated[0].Priority, updated[0].Wanted)

	if trackers, err := st.Trackers(ctx, task.ID); err != nil {
		t.Errorf("трекеры: %v", err)
	} else {
		t.Logf("трекеров: %d", len(trackers))
	}

	// Приоритет задачи в очереди.
	if err := st.SetPriority(ctx, []string{task.ID}, PriorityHigh); err != nil {
		t.Errorf("приоритет задачи: %v", err)
	}

	// Смена папки: берём общую папку, отличную от текущей.
	target := os.Getenv("DSM_TEST_FOLDER_DS")
	if target == "" {
		target = "Download"
	}
	if err := st.SetDestination(ctx, []string{task.ID}, target); err != nil {
		t.Fatalf("смена папки на %q: %v", target, err)
	}
	tasks, err := st.List(ctx)
	if err != nil {
		t.Fatalf("список после смены папки: %v", err)
	}
	for _, tk := range tasks {
		if tk.ID == task.ID {
			if tk.Destination != target {
				t.Errorf("папка %q, ожидалась %q", tk.Destination, target)
			} else {
				t.Logf("папка сменилась на %s", tk.Destination)
			}
		}
	}

	// Несуществующая папка должна отвергаться понятной ошибкой.
	if err := st.SetDestination(ctx, []string{task.ID}, "ПапкиТакойНет"); err == nil {
		t.Error("смена на несуществующую папку прошла успешно")
	} else {
		t.Logf("несуществующая папка → %v", err)
	}
}
