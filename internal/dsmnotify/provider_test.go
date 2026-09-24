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
	deleted  []int
}

func (f *fakeDSM) Call(_ context.Context, api, method string, _ int, params map[string]any, out any) error {
	f.calls = append(f.calls, method)
	if method == "list" {
		raw, _ := json.Marshal(map[string]any{"list": f.existing})
		return json.Unmarshal(raw, out)
	}
	if method == "delete" {
		f.deleted = append(f.deleted, params["profile_id"].(int))
	}
	f.last = params
	return nil
}

const testURL = "http://127.0.0.1:8080/dsm/notify"

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func providerEntry(id int, method, name, secret string) map[string]any {
	return providerAt(id, testURL, method, name, secret)
}

func providerAt(id int, url, method, name, secret string) map[string]any {
	return map[string]any{
		"profile_id": id,
		"target_config": map[string]any{
			"url":        url,
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

// The state found on a live NAS after the SynoCommunity build moved the port:
// the old provider on 8080 still in DSM's list, the new one on 58080 working.
// The old one has to go; the new one has to stay untouched.
func TestRemovesTheOneOnAnOldPort(t *testing.T) {
	const (
		oldURL = "http://127.0.0.1:8080/dsm/notify"
		newURL = "http://127.0.0.1:58080/dsm/notify"
	)
	f := &fakeDSM{existing: []map[string]any{
		providerAt(3, oldURL, "post", providerName, "s3cret"),
		providerAt(4, newURL, "post", providerName, "s3cret"),
	}}
	if err := Ensure(context.Background(), f, newURL, "s3cret", quiet()); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 || f.deleted[0] != 3 {
		t.Fatalf("deleted %v, want only the orphan on 8080 (profile 3)", f.deleted)
	}
	for _, c := range f.calls {
		if c == "set" || c == "create" {
			t.Fatalf("the current provider was fine and should not be rewritten: %v", f.calls)
		}
	}
}

// Somebody's own webhook — to a chat, a phone, a script — is not ours, and
// neither a start nor an uninstall may touch it.
func TestLeavesOtherWebhooksAlone(t *testing.T) {
	foreign := []string{
		"https://hooks.example.com/dsm/notify",
		"http://192.168.1.20:8080/dsm/notify",
		"http://127.0.0.1:9000/other",
	}
	var existing []map[string]any
	for i, u := range foreign {
		existing = append(existing, providerAt(10+i, u, "post", "someone else", "x"))
	}

	f := &fakeDSM{existing: existing}
	if err := Ensure(context.Background(), f, testURL, "s3cret", quiet()); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 0 {
		t.Fatalf("Ensure deleted %v, which are not ours", f.deleted)
	}

	f = &fakeDSM{existing: existing}
	if err := Remove(context.Background(), f, quiet()); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 0 {
		t.Fatalf("Remove deleted %v, which are not ours", f.deleted)
	}
}

func TestRemoveTakesEveryOneOfOurs(t *testing.T) {
	f := &fakeDSM{existing: []map[string]any{
		providerAt(3, "http://127.0.0.1:8080/dsm/notify", "post", providerName, "a"),
		providerEntry(4, "post", providerName, "b"),
		providerAt(5, "https://hooks.example.com/x", "post", "someone else", "c"),
	}}
	if err := Remove(context.Background(), f, quiet()); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 2 || f.deleted[0] != 3 || f.deleted[1] != 4 {
		t.Fatalf("deleted %v, want 3 and 4 and not the foreign 5", f.deleted)
	}
}

// Two entries for the same address — a crash between create and the next
// list could leave that — collapse to one.
func TestRemovesADuplicate(t *testing.T) {
	f := &fakeDSM{existing: []map[string]any{
		providerEntry(3, "post", providerName, "s3cret"),
		providerEntry(4, "post", providerName, "s3cret"),
	}}
	if err := Ensure(context.Background(), f, testURL, "s3cret", quiet()); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 || f.deleted[0] != 4 {
		t.Fatalf("deleted %v, want the second of the two (4)", f.deleted)
	}
}
