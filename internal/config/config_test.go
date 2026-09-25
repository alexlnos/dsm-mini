package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

var allKeys = []string{
	"DSM_URL", "DSM_USER", "DSM_PASSWORD", "DSM_OTP", "DSM_INSECURE_TLS",
	"TELEGRAM_BOT_TOKEN", "ALLOWED_USER_IDS", "PUBLIC_URL",
	"LISTEN_ADDR", "WATCH_INTERVAL", "LOG_LEVEL",
}

// isolate keeps a developer's own environment, which may well hold a live
// DSM_URL, out of the tests, and points the DSM web config at a file the test
// controls.
func isolate(t *testing.T) string {
	t.Helper()
	for _, k := range allKeys {
		t.Setenv(k, "")
	}
	dir := t.TempDir()
	old := DSMWebConfig
	DSMWebConfig = filepath.Join(dir, "DSM.json")
	t.Cleanup(func() { DSMWebConfig = old })
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const complete = `DSM_USER='dsm-mini'
DSM_PASSWORD='a b$c'
TELEGRAM_BOT_TOKEN='1234567890:AAExampleTokenReplaceThisWithYours0'
ALLOWED_USER_IDS='12, 34'
PUBLIC_URL='https://nas.example.com/'
`

// A package nobody has set up yet has no file at all. That is not a failure:
// it is the list of what the settings window has to ask, in its order.
func TestNoFileIsAListOfQuestions(t *testing.T) {
	dir := isolate(t)
	c, problems, err := Load(filepath.Join(dir, "config.env"))
	if err != nil {
		t.Fatalf("a missing file is not an error: %v", err)
	}
	if c == nil {
		t.Fatal("the defaults are needed even with nothing set")
	}
	if len(problems) != len(Required) {
		t.Fatalf("problems %v, want every required key", problems)
	}
	for i, key := range Required {
		if problems[i] != (Problem{Key: key, Code: Missing}) {
			t.Fatalf("problem %d is %v, want %s missing", i, problems[i], key)
		}
	}
}

func TestDefaults(t *testing.T) {
	dir := isolate(t)
	path := filepath.Join(dir, "config.env")
	write(t, path, complete)
	write(t, DSMWebConfig, `{"port": 5000, "ssl": {"port": 5443}}`)

	c, problems, err := Load(path)
	if err != nil || len(problems) != 0 {
		t.Fatalf("err %v, problems %v", err, problems)
	}
	// DSM on this machine, on the HTTPS port it was moved to.
	if c.DSMURL != "https://localhost:5443" {
		t.Fatalf("DSM_URL %q", c.DSMURL)
	}
	if !c.DSMInsecure {
		t.Fatal("DSM over loopback answers with its own certificate; verification has to be off")
	}
	if c.ListenAddr != "127.0.0.1:58080" {
		t.Fatalf("LISTEN_ADDR %q", c.ListenAddr)
	}
	if c.PublicURL != "https://nas.example.com" {
		t.Fatalf("PUBLIC_URL %q: the trailing slash has to go", c.PublicURL)
	}
	if c.DSMPassword != "a b$c" || len(c.AllowedUserIDs) != 2 {
		t.Fatalf("values read wrong: %+v", c)
	}
}

// Without DSM's own description of itself the port is the stock one.
func TestDSMPortFallsBack(t *testing.T) {
	isolate(t)
	if got := defaultDSMURL(); got != "https://localhost:5001" {
		t.Fatalf("got %q", got)
	}
}

// Up to 1.0.0 the package wrote ":port", which is every interface.
func TestBarePortMeansLoopback(t *testing.T) {
	dir := isolate(t)
	path := filepath.Join(dir, "config.env")
	write(t, path, complete+"LISTEN_ADDR=':8080'\n")
	c, _, _ := Load(path)
	if c.ListenAddr != "127.0.0.1:8080" {
		t.Fatalf("got %q", c.ListenAddr)
	}

	write(t, path, complete+"LISTEN_ADDR='0.0.0.0:8080'\n")
	c, _, _ = Load(path)
	if c.ListenAddr != "0.0.0.0:8080" {
		t.Fatalf("an address set on purpose was changed to %q", c.ListenAddr)
	}
}

// The file wins, even when what it says is "nobody": an environment variable
// must not open a service that the file closes.
func TestFileWinsOverEnvironment(t *testing.T) {
	dir := isolate(t)
	path := filepath.Join(dir, "config.env")
	write(t, path, `DSM_USER='u'
DSM_PASSWORD='p'
TELEGRAM_BOT_TOKEN='1234567890:AAExampleTokenReplaceThisWithYours0'
PUBLIC_URL='https://nas.example.com'
ALLOWED_USER_IDS=''
`)
	t.Setenv("ALLOWED_USER_IDS", "1")
	c, problems, _ := Load(path)
	if len(c.AllowedUserIDs) != 0 {
		t.Fatalf("the environment opened the service to %v", c.AllowedUserIDs)
	}
	if len(problems) != 1 || problems[0] != (Problem{Key: "ALLOWED_USER_IDS", Code: Missing}) {
		t.Fatalf("problems %v", problems)
	}
}

// Running the binary by hand, with the settings in the environment.
func TestEnvironmentFillsIn(t *testing.T) {
	dir := isolate(t)
	t.Setenv("DSM_USER", "u")
	t.Setenv("DSM_PASSWORD", "p")
	t.Setenv("TELEGRAM_BOT_TOKEN", "1234567890:AAExampleTokenReplaceThisWithYours0")
	t.Setenv("ALLOWED_USER_IDS", "1")
	t.Setenv("PUBLIC_URL", "https://nas.example.com")
	_, problems, err := Load(filepath.Join(dir, "config.env"))
	if err != nil || len(problems) != 0 {
		t.Fatalf("err %v, problems %v", err, problems)
	}
}

func TestBadValues(t *testing.T) {
	dir := isolate(t)
	path := filepath.Join(dir, "config.env")
	write(t, path, `DSM_URL='192.168.1.10:5001'
DSM_USER='u'
DSM_PASSWORD='p'
TELEGRAM_BOT_TOKEN='1234567890:AAExampleTokenReplaceThisWithYours0'
ALLOWED_USER_IDS='12, not-a-number'
PUBLIC_URL='http://nas.example.com'
LISTEN_ADDR='8080'
`)
	_, problems, _ := Load(path)
	want := map[Problem]bool{
		{Key: "ALLOWED_USER_IDS", Code: BadIDs}: true,
		{Key: "PUBLIC_URL", Code: NotHTTPS}:     true,
		{Key: "DSM_URL", Code: BadURL}:          true,
		{Key: "LISTEN_ADDR", Code: BadListen}:   true,
	}
	if len(problems) != len(want) {
		t.Fatalf("problems %v", problems)
	}
	for _, p := range problems {
		if !want[p] {
			t.Fatalf("unexpected %v among %v", p, problems)
		}
	}
}

// DSM somewhere else is reached over a network, and there the certificate
// is checked unless the file says otherwise.
func TestRemoteDSMIsVerified(t *testing.T) {
	dir := isolate(t)
	path := filepath.Join(dir, "config.env")
	write(t, path, complete+"DSM_URL='https://nas.example.com:5001'\n")
	c, _, _ := Load(path)
	if c.DSMInsecure {
		t.Fatal("a remote DSM went unverified")
	}
	write(t, path, complete+"DSM_URL='https://nas.example.com:5001'\nDSM_INSECURE_TLS='true'\n")
	c, _, _ = Load(path)
	if !c.DSMInsecure {
		t.Fatal("an explicit DSM_INSECURE_TLS was ignored")
	}
}

// The state found on a live NAS after a package that ran as another user was
// replaced: the file is there and cannot be opened. That has to come back as
// an error the caller can recognise, with the defaults still usable.
func TestUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	dir := isolate(t)
	path := filepath.Join(dir, "config.env")
	write(t, path, complete)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(path)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("got %v, want a permission error", err)
	}
	if c == nil || c.ListenAddr == "" {
		t.Fatal("the defaults are needed to serve the window that explains this")
	}
}

func TestFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.env")

	// A password with a space, a quote and a dollar sign: all three break a
	// file that is read with `.` unless the quoting holds.
	want := map[string]string{
		"DSM_USER":           "dsm-mini",
		"DSM_PASSWORD":       `a b$c'd`,
		"TELEGRAM_BOT_TOKEN": "1234567890:AAExampleTokenReplaceThisWithYours0",
		"ALLOWED_USER_IDS":   "1,2,3",
	}
	if err := WriteFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the file holds a password and a token; mode %v, err %v", info.Mode().Perm(), err)
	}
}

func TestReadFileTolerates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.env")
	// Comments, blank lines, an unquoted value and a double-quoted one: all
	// shapes a person editing the file over SSH is likely to leave behind.
	write(t, path, "# written by hand\n\nDSM_USER=bare\nPUBLIC_URL=\"https://nas.example.com\"\nLISTEN_ADDR='127.0.0.1:8080'\n")
	got, err := ReadFile(path)
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

// A file that belongs to somebody else cannot be written, but it can be
// replaced by whoever owns the directory — which is how the window saves new
// settings over ones a previous package user left behind.
func TestWriteReplacesAFileItCannotOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens everything")
	}
	path := filepath.Join(t.TempDir(), "config.env")
	write(t, path, "DSM_USER='old'\n")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, map[string]string{"DSM_USER": "new"}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil || got["DSM_USER"] != "new" {
		t.Fatalf("got %v, err %v", got, err)
	}
}
