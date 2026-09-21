package downloadstation

import "testing"

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
