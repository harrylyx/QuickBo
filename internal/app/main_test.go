package app

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points the user's directories at a temporary one for the whole
// run: New loads the history and favorites from the config directory, and a
// test must neither show nor change a real user's.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "quickbo-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "APPDATA", "LOCALAPPDATA", "USERPROFILE"} {
		os.Setenv(k, dir)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// New must not reach the real user's files under test.
func TestNewUsesTheTestHome(t *testing.T) {
	if got := settingsPath(); !filepathHasPrefix(got, os.Getenv("HOME")) {
		t.Fatalf("settings at %s, outside the test home %s", got, os.Getenv("HOME"))
	}
	if n := len(New().lib.watches()); n != 0 {
		t.Fatalf("a fresh app under test must have no history, has %d", n)
	}
}

func filepathHasPrefix(p, dir string) bool {
	return dir != "" && len(p) > len(dir) && p[:len(dir)] == dir
}
