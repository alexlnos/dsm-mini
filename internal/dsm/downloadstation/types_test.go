package downloadstation

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestStatusFromCodeTable checks the map against getStatusString from
// Download Station's own interface — the table it was transcribed from.
//
// The codes from 7 on were once shifted by one, which showed a task being
// unpacked as a failed one. A test is cheaper than the next such report.
func TestStatusFromCodeTable(t *testing.T) {
	table := map[int]Status{
		1: StatusWaiting, 11: StatusWaiting, 12: StatusWaiting,
		2: StatusDownloading,
		3: StatusPaused,
		4: StatusFinishing, 13: StatusFinishing, 14: StatusFinishing,
		5:  StatusFinished,
		6:  StatusHashChecking,
		7:  StatusSeeding, // pre-seeding
		8:  StatusSeeding,
		9:  StatusWaiting, // filehosting_waiting
		10: StatusExtracting,
		15: StatusError, // captcha_needed: needs a human
	}
	for code, want := range table {
		if got := StatusFromCode(code); got != want {
			t.Errorf("code %d: got %q, want %q", code, got, want)
		}
	}
}

// TestStatusFromCodeErrors covers the rule rather than the list: anything
// outside the table is a failure, exactly as Download Station's own
// getStatusString does with its default branch.
func TestStatusFromCodeErrors(t *testing.T) {
	// 101 is the plain failure seen on a live NAS; 105 is "disk full" and
	// 123 "invalid torrent"; 4242 is nothing at all.
	for _, code := range []int{101, 102, 105, 123, 134, 4242} {
		if got := StatusFromCode(code); got != StatusError {
			t.Errorf("code %d: got %q, want %q", code, got, StatusError)
		}
	}
}

// TestStatusFromStringMatchesCodes keeps the two API generations agreeing:
// the same state must come out the same whichever one reported it. The pairs
// were confirmed by asking both about one task on a live NAS.
func TestStatusFromStringMatchesCodes(t *testing.T) {
	pairs := []struct {
		code int
		text string
	}{
		{2, "downloading"},
		{5, "finished"},
		{3, "paused"},
		{8, "seeding"},
		{101, "error"},
		{9, "filehosting_waiting"},
	}
	for _, p := range pairs {
		byCode, byText := StatusFromCode(p.code), StatusFromString(p.text)
		if byCode != byText {
			t.Errorf("code %d gave %q, string %q gave %q", p.code, byCode, p.text, byText)
		}
	}
}

// TestReasonFromCode covers the failures named to the person, and the rule
// that an unnamed one stays silent rather than borrowing a neighbour's words.
func TestReasonFromCode(t *testing.T) {
	named := map[int]Reason{
		102: ReasonLink,
		103: ReasonDestination,
		104: ReasonDestination,
		105: ReasonDiskFull,
		106: ReasonDiskFull,
		107: ReasonTimeout,
		113: ReasonDuplicate,
		118: ReasonExtract,
		123: ReasonTorrent,
		129: ReasonExtract,
	}
	for code, want := range named {
		if got := ReasonFromCode(code); got != want {
			t.Errorf("code %d: got %q, want %q", code, got, want)
		}
	}

	// 101 is the plain failure: Download Station has no word for it either,
	// and the legacy API calls its detail "unknown". 127 (missing Python) and
	// 130 (an NZB article) belong to scenarios this app does not offer.
	for _, code := range []int{101, 127, 130, 4242} {
		if got := ReasonFromCode(code); got != ReasonNone {
			t.Errorf("code %d: got %q, expected no reason", code, got)
		}
	}
}

// TestFailReasonReachesTheApp guards the field name the Mini App reads, and
// that a task without a reason does not carry an empty one.
func TestFailReasonReachesTheApp(t *testing.T) {
	withReason, err := json.Marshal(Task{Status: StatusError, FailReason: ReasonDiskFull})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(withReason), `"fail_reason":"diskFull"`) {
		t.Errorf("the reason did not reach the JSON: %s", withReason)
	}

	plain, err := json.Marshal(Task{Status: StatusError})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "fail_reason") {
		t.Errorf("a task without a reason carries an empty one: %s", plain)
	}
}
