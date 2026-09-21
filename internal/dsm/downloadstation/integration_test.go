//go:build integration

// Интеграционные тесты против настоящего NAS.
//
// Запуск:
//
//	DSM_URL=https://nas:5001 DSM_USER=... DSM_PASSWORD=... \
//	  go test -tags=integration ./internal/dsm/downloadstation/ -v
//
// Тесты только читают состояние: ни одна задача не создаётся, не ставится на
// паузу и не удаляется.
package downloadstation

import (
	"context"
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
		t.Skip("не заданы DSM_URL / DSM_USER / DSM_PASSWORD — интеграционные тесты пропущены")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("DSM_INSECURE_TLS"))
	return dsm.New(dsm.Options{
		BaseURL: url, User: user, Password: pass,
		InsecureTLS: insecure, Timeout: 20 * time.Second,
	})
}

// TestBothGenerations проверяет, что современный и легаси путь видят NAS
// одинаково. На DSM 7 доступны оба API сразу, и это единственная возможность
// убедиться, что легаси-ветка не сгнила, не имея под рукой DSM 6.
func TestBothGenerations(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	gens := []Generation{GenerationV2, GenerationLegacy}
	byGen := make(map[Generation][]Task, len(gens))

	for _, gen := range gens {
		st, err := NewWithGeneration(ctx, c, gen)
		if err != nil {
			t.Fatalf("%s: создание: %v", gen, err)
		}

		tasks, err := st.List(ctx)
		if err != nil {
			t.Fatalf("%s: список задач: %v", gen, err)
		}
		byGen[gen] = tasks
		t.Logf("%s: задач %d", gen, len(tasks))
		for _, task := range tasks {
			t.Logf("  %s | %s | %s | %.1f%% | %d B | %s",
				task.ID, task.Status, task.Type, task.Progress()*100, task.Size, task.Destination)
		}

		stats, err := st.Stats(ctx)
		if err != nil {
			t.Errorf("%s: статистика: %v", gen, err)
		} else {
			t.Logf("%s: скорость ↓%d ↑%d B/s", gen, stats.SpeedDown, stats.SpeedUp)
		}

		dest, err := st.DefaultDestination(ctx)
		if err != nil {
			t.Errorf("%s: папка по умолчанию: %v", gen, err)
		} else if dest == "" {
			t.Errorf("%s: папка по умолчанию пуста", gen)
		} else {
			t.Logf("%s: папка по умолчанию %q", gen, dest)
		}

		vols, err := st.Volumes(ctx)
		if err != nil {
			t.Errorf("%s: тома: %v", gen, err)
		}
		for _, v := range vols {
			t.Logf("%s: том %s — свободно %d из %d B", gen, v.MountPoint, v.SizeFree, v.SizeTotal)
		}
	}

	v2, legacy := byGen[GenerationV2], byGen[GenerationLegacy]
	if len(v2) != len(legacy) {
		t.Fatalf("разное число задач: v2 %d, legacy %d", len(v2), len(legacy))
	}

	// Порядок задач у двух API совпадает, но полагаться на это не стоит.
	legacyByID := make(map[string]Task, len(legacy))
	for _, task := range legacy {
		legacyByID[task.ID] = task
	}
	for _, a := range v2 {
		b, ok := legacyByID[a.ID]
		if !ok {
			t.Errorf("задача %s есть в v2, но не в legacy", a.ID)
			continue
		}
		// Главная проверка: числовой код статуса в v2 и строка в legacy
		// должны приводиться к одному и тому же состоянию.
		if a.Status != b.Status {
			t.Errorf("задача %s: статус v2 %q, legacy %q", a.ID, a.Status, b.Status)
		}
		if a.Title != b.Title {
			t.Errorf("задача %s: название v2 %q, legacy %q", a.ID, a.Title, b.Title)
		}
		if a.Size != b.Size {
			t.Errorf("задача %s: размер v2 %d, legacy %d", a.ID, a.Size, b.Size)
		}
		if a.Destination != b.Destination {
			t.Errorf("задача %s: папка v2 %q, legacy %q", a.ID, a.Destination, b.Destination)
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			t.Errorf("задача %s: создана v2 %s, legacy %s — проверьте created_time против create_time",
				a.ID, a.CreatedAt, b.CreatedAt)
		}
	}
}

// TestAutoPicksV2 убеждается, что на DSM 7 автовыбор берёт современный API.
func TestAutoPicksV2(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("автовыбор: %v", err)
	}
	t.Logf("автовыбор дал поколение %q", st.Generation())
	if !c.HasAPI(ctx, apiTaskV2) {
		t.Skip("на этом NAS нет DownloadStation2 — сравнивать не с чем")
	}
	if st.Generation() != "v2" {
		t.Errorf("при доступном DownloadStation2 ожидалось v2, получено %q", st.Generation())
	}
}
