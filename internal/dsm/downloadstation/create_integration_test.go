//go:build integration

package downloadstation

import (
	"context"
	"os"
	"testing"
	"time"
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
