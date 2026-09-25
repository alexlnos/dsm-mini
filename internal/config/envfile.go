package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// envLine matches KEY='value' as the service writes it, and KEY=value as a
// person editing the file by hand is likely to.
var envLine = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(.*)$`)

// ReadFile reads a settings file.
//
// A missing file is not an error: a fresh installation has none until the
// settings window saves one.
func ReadFile(path string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := envLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out[m[1]] = unquote(m[2])
	}
	return out, scan.Err()
}

func unquote(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], `'\''`, `'`)
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// WriteFile rewrites a settings file through a temporary one: a half-written
// file is a service that starts with half its settings.
//
// Replacing rather than rewriting in place has a second use. A file left
// behind by a package that ran as another user cannot be opened, but DSM
// hands the directory to the new package user on an upgrade, and a directory
// one owns is one whose entries one may replace. So new settings can be saved
// over settings nobody can read.
//
// Values go in single quotes with the shell's escaping, so the file stays
// readable with `.` for anyone who does that by hand: a password with a space
// or a dollar sign would otherwise break it.
func WriteFile(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s='%s'\n", k, strings.ReplaceAll(values[k], `'`, `'\''`))
	}

	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
