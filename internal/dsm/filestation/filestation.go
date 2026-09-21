// Package filestation works with files on the NAS through SYNO.FileStation.*.
//
// The main quirk: File Station does NOT accept limit = -1. Unlike Download
// Station, where that is the normal way to get everything at once, here such
// a request crashes the handler into HTTP 502 — on any API version. So lists
// are read page by page. Details in docs/synology-api.md.
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

// pageSize is the page size when reading lists.
//
// The value -1 ("everything at once"), familiar from Download Station, is not
// accepted by File Station: the request fails with HTTP 502 on any API version.
const pageSize = 500

const (
	apiList     = "SYNO.FileStation.List"
	apiDelete   = "SYNO.FileStation.Delete"
	apiRename   = "SYNO.FileStation.Rename"
	apiUpload   = "SYNO.FileStation.Upload"
	apiCreate   = "SYNO.FileStation.CreateFolder"
	apiThumb    = "SYNO.FileStation.Thumb"
	apiDownload = "SYNO.FileStation.Download"
	apiCoreShar = "SYNO.Core.Share"
)

// Entry is a file or a folder.
type Entry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified,omitzero"`
}

// Station covers operations on NAS files.
type Station struct {
	c *dsm.Client
}

// New creates a File Station client.
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

// Shares lists the shared folders — the top level of the file tree.
//
// When File Station's list_share is broken, we take the list from
// SYNO.Core.Share: it returns the same folders and needs only admin rights.
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
		// Report the original cause: it is more useful than the fallback's error.
		return nil, fmt.Errorf("cannot get the list of shared folders: %w", err)
	}

	entries := make([]Entry, 0, len(fallback.Shares))
	for _, sh := range fallback.Shares {
		entries = append(entries, Entry{Name: sh.Name, Path: "/" + sh.Name, IsDir: true})
	}
	return entries, nil
}

// List returns the contents of a folder. folder is a path from the shared
// folder root, for example "/Media/Music".
func (s *Station) List(ctx context.Context, folder string) ([]Entry, error) {
	folder = normalize(folder)
	if folder == "/" {
		return s.Shares(ctx)
	}

	// Read page by page: limit = -1 is not allowed here, and folders on a home
	// NAS easily go past a thousand files.
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
	// Folders above files — that is more familiar and matches File Station.
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

// Delete removes files and folders for good.
//
// The synchronous variant is used: it is simpler at home-sized volumes, while
// the asynchronous one (start/status/stop with a taskid) is for huge trees.
func (s *Station) Delete(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths given")
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

// Rename renames a file or a folder. Returns the new path.
func (s *Station) Rename(ctx context.Context, target, newName string) (string, error) {
	if strings.ContainsAny(newName, `/\`) {
		return "", fmt.Errorf("a name cannot contain path separators")
	}
	if newName == "" || newName == "." || newName == ".." {
		return "", fmt.Errorf("invalid name")
	}
	target = normalize(target)

	// path and name are arrays here even for a single item.
	err := s.c.CallVersioned(ctx, apiRename, "rename", []int{2, 1}, map[string]any{
		"path": []string{target},
		"name": []string{newName},
	}, nil)
	if err != nil {
		return "", err
	}
	return path.Join(path.Dir(target), newName), nil
}

// CreateFolder creates a folder inside parent.
func (s *Station) CreateFolder(ctx context.Context, parent, name string) (string, error) {
	if strings.ContainsAny(name, `/\`) || name == "" {
		return "", fmt.Errorf("invalid folder name")
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

// Upload puts a file into a folder on the NAS.
//
// overwrite=false means "skip if the file is already there": without an
// explicit value DSM answers with an error on a name clash.
func (s *Station) Upload(ctx context.Context, folder, name string, data []byte, overwrite bool) error {
	if name == "" {
		return fmt.Errorf("no file name given")
	}
	if len(data) == 0 {
		return fmt.Errorf("empty file")
	}
	folder = normalize(folder)
	if folder == "/" {
		return fmt.Errorf("cannot upload into the root: name a shared folder")
	}

	// In multipart the values go as they are, without JSON quotes.
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

// normalize brings a path to the shape File Station understands: with a
// leading slash and without a trailing one. Unlike Download Station, where
// the path goes without a leading slash, here it is mandatory.
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
