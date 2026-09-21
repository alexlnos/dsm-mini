//go:build integration

// Integration tests against a real NAS.
//
// Run with:
//
//	DSM_URL=https://nas:5001 DSM_USER=... DSM_PASSWORD=... \
//	  go test -tags=integration ./internal/dsm/downloadstation/ -v
//
// The tests only read state: no task is created, paused
// or deleted.
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
		t.Skip("DSM_URL / DSM_USER / DSM_PASSWORD are not set — integration tests skipped")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("DSM_INSECURE_TLS"))
	return dsm.New(dsm.Options{
		BaseURL: url, User: user, Password: pass,
		InsecureTLS: insecure, Timeout: 20 * time.Second,
	})
}

// TestBothGenerations checks that the modern and the legacy path see the NAS
// the same way. On DSM 7 both APIs are available at once, and that is the only
// chance to make sure the legacy branch has not rotted without a DSM 6 at hand.
func TestBothGenerations(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	gens := []Generation{GenerationV2, GenerationLegacy}
	byGen := make(map[Generation][]Task, len(gens))

	for _, gen := range gens {
		st, err := NewWithGeneration(ctx, c, gen)
		if err != nil {
			t.Fatalf("%s: creation: %v", gen, err)
		}

		tasks, err := st.List(ctx)
		if err != nil {
			t.Fatalf("%s: task list: %v", gen, err)
		}
		byGen[gen] = tasks
		t.Logf("%s: %d tasks", gen, len(tasks))
		for _, task := range tasks {
			t.Logf("  %s | %s | %s | %.1f%% | %d B | %s",
				task.ID, task.Status, task.Type, task.Progress()*100, task.Size, task.Destination)
		}

		stats, err := st.Stats(ctx)
		if err != nil {
			t.Errorf("%s: statistics: %v", gen, err)
		} else {
			t.Logf("%s: speed ↓%d ↑%d B/s", gen, stats.SpeedDown, stats.SpeedUp)
		}

		dest, err := st.DefaultDestination(ctx)
		if err != nil {
			t.Errorf("%s: default folder: %v", gen, err)
		} else if dest == "" {
			t.Errorf("%s: the default folder is empty", gen)
		} else {
			t.Logf("%s: default folder %q", gen, dest)
		}

		vols, err := st.Volumes(ctx)
		if err != nil {
			t.Errorf("%s: volumes: %v", gen, err)
		}
		for _, v := range vols {
			t.Logf("%s: volume %s — %d free of %d B", gen, v.MountPoint, v.SizeFree, v.SizeTotal)
		}
	}

	v2, legacy := byGen[GenerationV2], byGen[GenerationLegacy]
	if len(v2) != len(legacy) {
		t.Fatalf("different task counts: v2 %d, legacy %d", len(v2), len(legacy))
	}

	// The task order matches between the two APIs, but relying on it is unwise.
	legacyByID := make(map[string]Task, len(legacy))
	for _, task := range legacy {
		legacyByID[task.ID] = task
	}
	for _, a := range v2 {
		b, ok := legacyByID[a.ID]
		if !ok {
			t.Errorf("task %s is in v2 but not in legacy", a.ID)
			continue
		}
		// The main check: the numeric status code in v2 and the string in
		// legacy must map to one and the same state.
		if a.Status != b.Status {
			t.Errorf("task %s: status v2 %q, legacy %q", a.ID, a.Status, b.Status)
		}
		if a.Title != b.Title {
			t.Errorf("task %s: title v2 %q, legacy %q", a.ID, a.Title, b.Title)
		}
		if a.Size != b.Size {
			t.Errorf("task %s: size v2 %d, legacy %d", a.ID, a.Size, b.Size)
		}
		if a.Destination != b.Destination {
			t.Errorf("task %s: folder v2 %q, legacy %q", a.ID, a.Destination, b.Destination)
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			t.Errorf("task %s: created v2 %s, legacy %s — check created_time against create_time",
				a.ID, a.CreatedAt, b.CreatedAt)
		}
	}
}

// TestAutoPicksV2 makes sure the automatic choice takes the modern API on DSM 7.
func TestAutoPicksV2(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("automatic choice: %v", err)
	}
	t.Logf("the automatic choice gave generation %q", st.Generation())
	if !c.HasAPI(ctx, apiTaskV2) {
		t.Skip("this NAS has no DownloadStation2 — nothing to compare with")
	}
	if st.Generation() != "v2" {
		t.Errorf("with DownloadStation2 available v2 was expected, got %q", st.Generation())
	}
}
