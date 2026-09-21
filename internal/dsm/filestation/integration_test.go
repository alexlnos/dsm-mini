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
		t.Skip("DSM_URL / DSM_USER / DSM_PASSWORD are not set")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("DSM_INSECURE_TLS"))
	c := dsm.New(dsm.Options{BaseURL: url, User: user, Password: pass,
		InsecureTLS: insecure, Timeout: 30 * time.Second})
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })
	return c
}

// TestBrowse checks reading the tree. It changes nothing.
func TestBrowse(t *testing.T) {
	ctx := context.Background()
	st := New(newClient(t))

	shares, err := st.Shares(ctx)
	if err != nil {
		t.Fatalf("shared folders: %v", err)
	}
	if len(shares) == 0 {
		t.Fatal("the list of shared folders is empty")
	}
	t.Logf("shared folders: %d", len(shares))
	for _, s := range shares {
		t.Logf("  %s", s.Path)
	}

	// Look for the first non-empty folder we are allowed into: an empty one
	// proves nothing about parsing the response.
	var listed bool
	for _, s := range shares {
		entries, err := st.List(ctx, s.Path)
		if err != nil {
			t.Logf("  %s — skipping: %v", s.Path, err)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		listed = true
		t.Logf("%s: %d entries", s.Path, len(entries))
		for i, e := range entries {
			if i >= 5 {
				break
			}
			kind := "file"
			if e.IsDir {
				kind = "folder"
			}
			t.Logf("  %-6s %-40s %d B", kind, e.Name, e.Size)
		}
		break
	}
	if !listed {
		t.Error("not a single shared folder could be read")
	}
}

// TestRootListsShares: an empty path and "/" both give the shared folder list.
func TestRootListsShares(t *testing.T) {
	ctx := context.Background()
	st := New(newClient(t))
	for _, p := range []string{"", "/"} {
		entries, err := st.List(ctx, p)
		if err != nil {
			t.Fatalf("list for %q: %v", p, err)
		}
		if len(entries) == 0 {
			t.Errorf("the list for %q is empty", p)
		}
	}
}

// TestFileLifecycle checks writing: creating a folder, uploading a file,
// renaming and deleting. It changes NAS state, so it needs
// DSM_TEST_MUTATIONS=1 and works in a temporary folder it cleans up itself.
func TestFileLifecycle(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("the test changes files on the NAS; run only with DSM_TEST_MUTATIONS=1")
	}
	ctx := context.Background()
	st := New(newClient(t))

	parent := os.Getenv("DSM_TEST_FOLDER")
	if parent == "" {
		t.Skip("DSM_TEST_FOLDER is not set — the folder to create temporary files in")
	}

	name := fmt.Sprintf("dsm-mini-test-%d", time.Now().Unix())
	dir, err := st.CreateFolder(ctx, parent, name)
	if err != nil {
		t.Fatalf("creating the folder: %v", err)
	}
	t.Logf("folder %s created", dir)
	t.Cleanup(func() {
		if err := st.Delete(context.Background(), []string{dir}); err != nil {
			t.Errorf("CLEANUP FAILED, delete %s by hand: %v", dir, err)
			return
		}
		t.Logf("folder %s deleted", dir)
	})

	content := []byte("dsm-mini integration test\n")
	if err := st.Upload(ctx, dir, "hello.txt", content, true); err != nil {
		t.Fatalf("uploading the file: %v", err)
	}

	entries, err := st.List(ctx, dir)
	if err != nil {
		t.Fatalf("list after the upload: %v", err)
	}
	var found *Entry
	for i := range entries {
		if entries[i].Name == "hello.txt" {
			found = &entries[i]
		}
	}
	if found == nil {
		t.Fatalf("the uploaded file did not appear in %s", dir)
	}
	if found.Size != int64(len(content)) {
		t.Errorf("file size %d, expected %d", found.Size, len(content))
	}
	t.Logf("file uploaded: %s, %d B", found.Path, found.Size)

	renamed, err := st.Rename(ctx, found.Path, "renamed.txt")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	t.Logf("renamed to %s", renamed)

	entries, err = st.List(ctx, dir)
	if err != nil {
		t.Fatalf("list after the rename: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "renamed.txt" {
		t.Errorf("in the folder after the rename: %+v", entries)
	}
}

// TestRenameRejectsSeparators: a name with a separator must not reach the NAS.
func TestRenameRejectsSeparators(t *testing.T) {
	ctx := context.Background()
	st := New(newClient(t))
	if _, err := st.Rename(ctx, "/Media/x", "../evil"); err == nil {
		t.Error("a name with a path separator must be refused")
	}
}

// TestCopyMovePreview checks copying, moving and previewing on a live NAS.
// It changes files: needs DSM_TEST_MUTATIONS=1 and DSM_TEST_FOLDER.
func TestCopyMovePreview(t *testing.T) {
	if os.Getenv("DSM_TEST_MUTATIONS") != "1" {
		t.Skip("the test changes files on the NAS; run only with DSM_TEST_MUTATIONS=1")
	}
	parent := os.Getenv("DSM_TEST_FOLDER")
	if parent == "" {
		t.Skip("DSM_TEST_FOLDER is not set")
	}

	ctx := context.Background()
	st := New(newClient(t))

	root, err := st.CreateFolder(ctx, parent, fmt.Sprintf("dsm-mini-fs-%d", time.Now().Unix()))
	if err != nil {
		t.Fatalf("creating the working folder: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Delete(context.Background(), []string{root}); err != nil {
			t.Errorf("CLEANUP FAILED, delete %s by hand: %v", root, err)
		}
	})

	src, err := st.CreateFolder(ctx, root, "src")
	if err != nil {
		t.Fatalf("source folder: %v", err)
	}
	dst, err := st.CreateFolder(ctx, root, "dst")
	if err != nil {
		t.Fatalf("destination folder: %v", err)
	}

	content := []byte("hello from dsm-mini\nsecond line\n")
	if err := st.Upload(ctx, src, "note.txt", content, true); err != nil {
		t.Fatalf("upload: %v", err)
	}

	// Copying.
	taskID, err := st.Copy(ctx, []string{src + "/note.txt"}, dst, true)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := waitTransfer(ctx, t, st, taskID); err != nil {
		t.Fatalf("the copy did not finish: %v", err)
	}
	if !hasFile(ctx, t, st, dst, "note.txt") {
		t.Error("the copy did not appear in the destination folder")
	}
	if !hasFile(ctx, t, st, src, "note.txt") {
		t.Error("the original vanished during the copy")
	}
	t.Log("copying went through")

	// Moving: the source must end up empty.
	if err := st.Upload(ctx, src, "moved.txt", content, true); err != nil {
		t.Fatalf("upload for the move: %v", err)
	}
	taskID, err = st.Move(ctx, []string{src + "/moved.txt"}, dst, true)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if err := waitTransfer(ctx, t, st, taskID); err != nil {
		t.Fatalf("the move did not finish: %v", err)
	}
	if hasFile(ctx, t, st, src, "moved.txt") {
		t.Error("after the move the file stayed in the source")
	}
	if !hasFile(ctx, t, st, dst, "moved.txt") {
		t.Error("the moved file did not appear in the destination")
	}
	t.Log("moving went through")

	// Guard against moving a folder inside itself.
	if _, err := st.Move(ctx, []string{src}, src+"/inside", true); err == nil {
		t.Error("moving a folder inside itself must be refused")
	}

	// Preview: the contents are read back.
	preview, err := st.Download(ctx, dst+"/note.txt")
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	defer preview.Body.Close()
	got, err := io.ReadAll(preview.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("the contents did not match: %q", string(got))
	}
	t.Logf("preview: %s, %d bytes", preview.ContentType, len(got))

	// A thumbnail of a text file is impossible — that is not a failure.
	if _, err := st.Thumbnail(ctx, dst+"/note.txt", ThumbSmall); err == nil {
		t.Log("the NAS unexpectedly returned a thumbnail of a text file")
	} else {
		t.Logf("no thumbnail for text, as expected: %v", err)
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
			t.Logf("task %s finished, skipped=%v", taskID, status.Skipped)
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("task %s did not finish within a minute", taskID)
}

func hasFile(ctx context.Context, t *testing.T, st *Station, folder, name string) bool {
	t.Helper()
	entries, err := st.List(ctx, folder)
	if err != nil {
		t.Fatalf("list %s: %v", folder, err)
	}
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}
