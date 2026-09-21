package downloadstation

import "testing"

// TestStatusFromCodeConfirmed locks in the codes seen on a live NAS.
//
// Each of these was verified by asking both API generations about the same
// task: the legacy one answers with a string, the modern one with a number.
// 101 is here because a failed download once reached the app as "unknown",
// painted in the calm blue of an ordinary download.
func TestStatusFromCodeConfirmed(t *testing.T) {
	confirmed := map[int]Status{
		2:   StatusDownloading,
		5:   StatusFinished,
		101: StatusError,
	}
	for code, want := range confirmed {
		if got := StatusFromCode(code); got != want {
			t.Errorf("code %d: got %q, want %q", code, got, want)
		}
	}
}

// TestStatusFromCodeUnknown checks that an unmapped code degrades quietly
// rather than being reported as some neighbouring state.
func TestStatusFromCodeUnknown(t *testing.T) {
	if got := StatusFromCode(4242); got != StatusUnknown {
		t.Errorf("an unmapped code gave %q, want %q", got, StatusUnknown)
	}
}

// TestStatusFromStringMatchesCodes keeps the two generations agreeing: the
// same state must come out the same whichever API reported it.
func TestStatusFromStringMatchesCodes(t *testing.T) {
	pairs := []struct {
		code int
		text string
	}{
		{2, "downloading"},
		{5, "finished"},
		{101, "error"},
		{3, "paused"},
		{7, "seeding"},
	}
	for _, p := range pairs {
		byCode, byText := StatusFromCode(p.code), StatusFromString(p.text)
		if byCode != byText {
			t.Errorf("code %d gave %q, string %q gave %q", p.code, byCode, p.text, byText)
		}
	}
}
