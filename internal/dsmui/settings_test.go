package dsmui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")

	// A password with a space, a quote and a dollar sign: all three break a
	// file that is read with `.` unless the quoting holds.
	want := map[string]string{
		"DSM_USER":           "dsm-mini",
		"DSM_PASSWORD":       `a b$c'd`,
		"TELEGRAM_BOT_TOKEN": "1234567890:AAExampleTokenReplaceThisWithYours0",
		"ALLOWED_USER_IDS":   "1,2,3",
	}
	if err := writeEnv(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestReadEnvTolerates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")
	// Comments, blank lines, an unquoted value and a double-quoted one: all
	// shapes a person editing the file over SSH is likely to leave behind.
	body := "# written by hand\n\nDSM_USER=bare\nPUBLIC_URL=\"https://nas.example.com\"\nLISTEN_ADDR='127.0.0.1:8080'\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"DSM_USER": "bare", "PUBLIC_URL": "https://nas.example.com", "LISTEN_ADDR": "127.0.0.1:8080",
	} {
		if got[k] != want {
			t.Fatalf("%s: got %q, want %q", k, got[k], want)
		}
	}
}

// A missing file is the state of an installation that never ran the wizard.
// The screen is how it gets filled in, so it has to open.
func TestMissingFileIsEmpty(t *testing.T) {
	got, err := readEnv(filepath.Join(t.TempDir(), "nothing.env"))
	if err != nil {
		t.Fatalf("a missing file should not be an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want nothing", got)
	}
}

func TestValidate(t *testing.T) {
	bad := map[string]string{
		"DSM_URL":          "192.168.1.10:5001",
		"ALLOWED_USER_IDS": "12, not-a-number",
		"LISTEN_ADDR":      "8080",
	}
	for key, value := range bad {
		if validate(key, value) == "" {
			t.Fatalf("%s=%q was accepted", key, value)
		}
	}
	good := map[string]string{
		"DSM_URL":          "https://127.0.0.1:5001",
		"ALLOWED_USER_IDS": " 12, 34 ,56 ",
		"LISTEN_ADDR":      "127.0.0.1:8080",
		"PUBLIC_URL":       "",
	}
	for key, value := range good {
		if problem := validate(key, value); problem != "" {
			t.Fatalf("%s=%q was refused: %s", key, value, problem)
		}
	}
}

func TestListenPortFallsBack(t *testing.T) {
	cases := []struct {
		file, running string
		want          int
	}{
		{"127.0.0.1:9090", "127.0.0.1:8080", 9090}, // the file wins: not applied yet
		{"", "127.0.0.1:8080", 8080},
		{"nonsense", "", 8080},
	}
	for _, c := range cases {
		if got := listenPort(c.file, c.running); got != c.want {
			t.Fatalf("listenPort(%q, %q) = %d, want %d", c.file, c.running, got, c.want)
		}
	}
}
