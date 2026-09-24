package dsmnotify

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

// fakeDSM answers list with whatever it holds and records the writes.
type fakeDSM struct {
	existing []map[string]any
	calls    []string
	last     map[string]any
}

func (f *fakeDSM) Call(_ context.Context, api, method string, _ int, params map[string]any, out any) error {
	f.calls = append(f.calls, method)
	if method == "list" {
		raw, _ := json.Marshal(map[string]any{"list": f.existing})
		return json.Unmarshal(raw, out)
	}
	f.last = params
	return nil
}

const testURL = "http://127.0.0.1:8080/dsm/notify"

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func providerEntry(id int, method, name, secret string) map[string]any {
	return map[string]any{
		"profile_id": id,
		"target_config": map[string]any{
			"url":        testURL,
			"req_method": method,
			"req_param":  bodyTemplate,
			"req_header": "Content-Type:application/json" + headerSep + SecretHeader + ":" + secret + headerSep,
			"provider":   name,
		},
	}
}

func TestCreatesWhenAbsent(t *testing.T) {
	f := &fakeDSM{}
	if err := Ensure(context.Background(), f, testURL, "s3cret", quiet()); err != nil {
		t.Fatal(err)
	}
	if got := f.calls; len(got) != 2 || got[1] != "create" {
		t.Fatalf("calls = %v, want list then create", got)
	}
	// The sender refuses anything but lower case; see reqMethod.
	if f.last["req_method"] != "post" {
		t.Fatalf("method %v, DSM only sends with \"post\"", f.last["req_method"])
	}
	if f.last["provider"] != providerName {
		t.Fatalf("name %v, want %q", f.last["provider"], providerName)
	}
}

// The exact state an earlier version left behind: registered with "POST",
// looking right in every listing, never sending. It has to be corrected, not
// recognised as fine.
func TestCorrectsUpperCaseMethod(t *testing.T) {
	f := &fakeDSM{existing: []map[string]any{providerEntry(3, "POST", providerName, "s3cret")}}
	if err := Ensure(context.Background(), f, testURL, "s3cret", quiet()); err != nil {
		t.Fatal(err)
	}
	if got := f.calls; len(got) != 2 || got[1] != "set" {
		t.Fatalf("calls = %v, want list then set", got)
	}
	if f.last["profile_id"] != 3 || f.last["req_method"] != "post" {
		t.Fatalf("set with %v", f.last)
	}
}

func TestRenamesAnUnnamedOne(t *testing.T) {
	f := &fakeDSM{existing: []map[string]any{providerEntry(3, "post", "", "s3cret")}}
	if err := Ensure(context.Background(), f, testURL, "s3cret", quiet()); err != nil {
		t.Fatal(err)
	}
	if got := f.calls; len(got) != 2 || got[1] != "set" {
		t.Fatalf("calls = %v, want list then set", got)
	}
}

// A restart must not rewrite an entry that is already right: DSM would log a
// change nobody made, and someone may be looking at it.
func TestLeavesACorrectOneAlone(t *testing.T) {
	f := &fakeDSM{existing: []map[string]any{providerEntry(3, "post", providerName, "s3cret")}}
	if err := Ensure(context.Background(), f, testURL, "s3cret", quiet()); err != nil {
		t.Fatal(err)
	}
	if got := f.calls; len(got) != 1 {
		t.Fatalf("calls = %v, want only the list", got)
	}
}
