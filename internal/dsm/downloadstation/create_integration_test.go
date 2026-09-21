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

// torrentURL is the official Ubuntu Server torrent: legal, with live seeds
// and a stable address. The task is created and deleted right away, nothing
// has time to download.
const torrentURL = "https://releases.ubuntu.com/24.04/ubuntu-24.04.5-live-server-amd64.iso.torrent"

// TestCreateAndDelete checks the full task lifecycle on a live NAS.
//
// The test CHANGES Download Station state, so it needs separate permission
// through DSM_TEST_MUTATIONS=1 — the integration tag alone is not enough.
// The created task is removed in t.Cleanup even when the test fails.
func TestCreateAndDelete(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("the test changes NAS state; run only with DSM_TEST_MUTATIONS=1")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	for _, gen := range []Generation{GenerationV2, GenerationLegacy} {
		t.Run(string(gen), func(t *testing.T) {
			st, err := NewWithGeneration(ctx, c, gen)
			if err != nil {
				t.Fatalf("creation: %v", err)
			}

			// We drop it into the default folder but pass that folder EXPLICITLY:
			// the difference in destination quoting between generations shows up
			// exactly on an explicit destination. No new folders appear either.
			dest, err := st.DefaultDestination(ctx)
			if err != nil {
				t.Fatalf("default folder: %v", err)
			}

			before, err := taskIDs(ctx, st)
			if err != nil {
				t.Fatalf("list before creation: %v", err)
			}

			if err := st.Create(ctx, CreateRequest{URLs: []string{torrentURL}, Destination: dest}); err != nil {
				t.Fatalf("task creation: %v", err)
			}

			task, err := waitForNewTask(ctx, t, st, before)
			if err != nil {
				t.Fatalf("the new task did not appear: %v", err)
			}

			// Delete whatever the checks below decide.
			t.Cleanup(func() {
				if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
					t.Errorf("CLEANUP FAILED, task %s (%s) must be deleted by hand: %v",
						task.ID, task.Title, err)
					return
				}
				t.Logf("task %s deleted", task.ID)
			})

			t.Logf("created %s | %s | status %s | folder %q",
				task.ID, task.Title, task.Status, task.Destination)

			if task.Destination != dest {
				t.Errorf("destination folder %q, expected %q — check the destination quoting",
					task.Destination, dest)
			}
			// The generations diverge: v2 recognises a .torrent by the link and
			// starts a bt task at once, legacy first downloads the file over https.
			t.Logf("task type %q", task.Type)

			if err := st.Pause(ctx, []string{task.ID}); err != nil {
				t.Errorf("pause: %v", err)
			} else if got := waitForStatus(ctx, st, task.ID, StatusPaused); got != StatusPaused {
				t.Errorf("after pausing the status is %q, expected paused", got)
			} else {
				t.Log("pausing worked")
			}

			if err := st.Resume(ctx, []string{task.ID}); err != nil {
				t.Errorf("resume: %v", err)
			} else {
				t.Logf("resuming worked, status %q",
					waitForStatus(ctx, st, task.ID, StatusDownloading))
			}
		})
	}

	// After all the cleanups there must be no tasks left.
	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("check after cleanup: %v", err)
	}
	tasks, err := st.List(ctx)
	if err != nil {
		t.Fatalf("list after cleanup: %v", err)
	}
	for _, task := range tasks {
		if task.Title != "" && contains(task.Title, "ubuntu-24.04") {
			t.Errorf("a test task is left over: %s (%s) — delete it by hand", task.ID, task.Title)
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

// waitForNewTask waits for a task that was not there before creation. Download
// Station first downloads the .torrent itself and only then starts the task,
// so it cannot be expected to appear instantly.
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

// TestCreateFromTorrentFile queues a task from .torrent contents — the path
// the bot takes when the file is sent straight into the chat.
//
// Changes NAS state: needs DSM_TEST_MUTATIONS=1, removes the task afterwards.
func TestCreateFromTorrentFile(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("the test changes NAS state; run only with DSM_TEST_MUTATIONS=1")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	torrent := fetchTorrent(t)
	t.Logf("torrent fetched, %d B", len(torrent))

	for _, gen := range []Generation{GenerationV2, GenerationLegacy} {
		t.Run(string(gen), func(t *testing.T) {
			st, err := NewWithGeneration(ctx, c, gen)
			if err != nil {
				t.Fatalf("creation: %v", err)
			}
			dest, err := st.DefaultDestination(ctx)
			if err != nil {
				t.Fatalf("default folder: %v", err)
			}

			before, err := taskIDs(ctx, st)
			if err != nil {
				t.Fatalf("list before: %v", err)
			}

			err = st.Create(ctx, CreateRequest{
				TorrentFile: torrent,
				FileName:    "ubuntu-24.04.5-live-server-amd64.iso.torrent",
				Destination: dest,
			})
			if err != nil {
				// On DSM 7 the legacy file upload handler answers 101 on all
				// three of its versions: DownloadStation2 replaced it. The
				// branch stays for DSM 6, where there is nothing to test it on.
				var apiErr *dsm.APIError
				if gen == GenerationLegacy && errors.As(err, &apiErr) && apiErr.Code == 101 {
					t.Skipf("legacy file upload is unavailable on this DSM (code 101) — expected on DSM 7")
				}
				t.Fatalf("creation from a file: %v", err)
			}

			task, err := waitForNewTask(ctx, t, st, before)
			if err != nil {
				t.Fatalf("the task did not appear: %v", err)
			}
			t.Cleanup(func() {
				if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
					t.Errorf("CLEANUP FAILED, delete %s by hand: %v", task.ID, err)
					return
				}
				t.Logf("task %s deleted", task.ID)
			})

			t.Logf("created %s | %s | type %s | folder %q", task.ID, task.Title, task.Type, task.Destination)
			// The .torrent file is parsed on the NAS, so the task is a torrent
			// right away — unlike queueing by a link to the same file.
			if task.Type != "bt" {
				t.Errorf("task type %q, expected bt", task.Type)
			}
			if task.Destination != dest {
				t.Errorf("folder %q, expected %q", task.Destination, dest)
			}
		})
	}
}

func fetchTorrent(t *testing.T) []byte {
	t.Helper()
	if p := os.Getenv("DSM_TEST_TORRENT"); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("cannot read %s: %v", p, err)
		}
		return data
	}
	req, err := http.NewRequest(http.MethodGet, torrentURL, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		t.Skipf("cannot download the torrent (no internet?): %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the torrent: %v", err)
	}
	return data
}

// TestCreateFromMagnet queues a task from the magnet link given in
// DSM_TEST_MAGNET and deletes it. Needed to check the whole path on a real
// link rather than on a torrent file over HTTP.
func TestCreateFromMagnet(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("the test changes NAS state; run only with DSM_TEST_MUTATIONS=1")
	}
	magnet := os.Getenv("DSM_TEST_MAGNET")
	if magnet == "" {
		t.Skip("DSM_TEST_MAGNET is not set")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("creation: %v", err)
	}
	dest := os.Getenv("DSM_TEST_FOLDER_DS")
	if dest == "" {
		if dest, err = st.DefaultDestination(ctx); err != nil {
			t.Fatalf("default folder: %v", err)
		}
	}

	before, err := taskIDs(ctx, st)
	if err != nil {
		t.Fatalf("list before: %v", err)
	}

	if err := st.Create(ctx, CreateRequest{URLs: []string{magnet}, Destination: dest}); err != nil {
		t.Fatalf("creation from magnet: %v", err)
	}

	task, err := waitForNewTask(ctx, t, st, before)
	if err != nil {
		t.Fatalf("the task did not appear: %v", err)
	}

	keep := os.Getenv("DSM_TEST_KEEP") == "1"
	if !keep {
		t.Cleanup(func() {
			if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
				t.Errorf("CLEANUP FAILED, delete %s by hand: %v", task.ID, err)
				return
			}
			t.Logf("task %s deleted", task.ID)
		})
	} else {
		t.Logf("DSM_TEST_KEEP=1 — task %s left downloading", task.ID)
	}

	t.Logf("created %s", task.ID)
	t.Logf("  title: %s", task.Title)
	t.Logf("  type: %s, status: %s, size: %d B", task.Type, task.Status, task.Size)
	t.Logf("  folder: %s", task.Destination)

	if task.Type != "bt" {
		t.Errorf("type %q, expected bt", task.Type)
	}
	if task.Destination != dest {
		t.Errorf("folder %q, expected %q", task.Destination, dest)
	}
}

// TestActiveTaskDetails checks everything available only for a running task:
// the file list, priorities, trackers and changing the folder.
//
// The task is created, checked and deleted. Needs DSM_TEST_MUTATIONS=1.
func TestActiveTaskDetails(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("the test changes NAS state; run only with DSM_TEST_MUTATIONS=1")
	}

	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("creation: %v", err)
	}
	if st.Generation() != "v2" {
		t.Skip("files and priorities are available only through DownloadStation2")
	}

	dest, err := st.DefaultDestination(ctx)
	if err != nil {
		t.Fatalf("default folder: %v", err)
	}
	before, err := taskIDs(ctx, st)
	if err != nil {
		t.Fatalf("list before: %v", err)
	}
	if err := st.Create(ctx, CreateRequest{
		TorrentFile: fetchTorrent(t),
		FileName:    "test.torrent",
		Destination: dest,
	}); err != nil {
		t.Fatalf("task creation: %v", err)
	}

	task, err := waitForNewTask(ctx, t, st, before)
	if err != nil {
		t.Fatalf("the task did not appear: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Delete(context.Background(), []string{task.ID}, false); err != nil {
			t.Errorf("CLEANUP FAILED, delete %s by hand: %v", task.ID, err)
		}
	})

	// Files do not appear at once: the NAS checks the hashes first.
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
		t.Fatalf("task files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("the file list is empty")
	}
	t.Logf("files: %d", len(files))
	for _, f := range files {
		t.Logf("  [%d] %s — %d B, priority %s, download=%v",
			f.Index, f.Name, f.Size, f.Priority, f.Wanted)
	}

	// Priority and the "download it" flag.
	if err := st.SetFile(ctx, task.ID, []int{files[0].Index}, PriorityLow, nil); err != nil {
		t.Fatalf("changing the file priority: %v", err)
	}
	no := false
	if err := st.SetFile(ctx, task.ID, []int{files[0].Index}, "", &no); err != nil {
		t.Fatalf("clearing the wanted flag: %v", err)
	}

	updated, err := st.Files(ctx, task.ID)
	if err != nil {
		t.Fatalf("files after the change: %v", err)
	}
	if updated[0].Priority != PriorityLow {
		t.Errorf("priority %q, expected low", updated[0].Priority)
	}
	if updated[0].Wanted {
		t.Error("the file stayed on the download list")
	}
	t.Logf("after the change: priority %s, download=%v", updated[0].Priority, updated[0].Wanted)

	if trackers, err := st.Trackers(ctx, task.ID); err != nil {
		t.Errorf("trackers: %v", err)
	} else {
		t.Logf("trackers: %d", len(trackers))
	}

	// The task priority in the queue.
	if err := st.SetPriority(ctx, []string{task.ID}, PriorityHigh); err != nil {
		t.Errorf("task priority: %v", err)
	}

	// Changing the folder: take a shared folder other than the current one.
	target := os.Getenv("DSM_TEST_FOLDER_DS")
	if target == "" {
		target = "Download"
	}
	if err := st.SetDestination(ctx, []string{task.ID}, target); err != nil {
		t.Fatalf("changing the folder to %q: %v", target, err)
	}
	tasks, err := st.List(ctx)
	if err != nil {
		t.Fatalf("list after the folder change: %v", err)
	}
	for _, tk := range tasks {
		if tk.ID == task.ID {
			if tk.Destination != target {
				t.Errorf("folder %q, expected %q", tk.Destination, target)
			} else {
				t.Logf("the folder changed to %s", tk.Destination)
			}
		}
	}

	// A missing folder must be refused with a clear error.
	if err := st.SetDestination(ctx, []string{task.ID}, "NoSuchFolder"); err == nil {
		t.Error("changing to a missing folder succeeded")
	} else {
		t.Logf("missing folder → %v", err)
	}
}
