package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveStorageProfile(t *testing.T) {
	t.Setenv(storageProfileEnv, "")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	deep := filepath.Join(root, "work", "repo", "pkg")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(dir, name string) {
		if err := os.WriteFile(filepath.Join(dir, storageProfileFile), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want string, wantErr bool) {
		t.Helper()
		got, err := resolveStorageProfile(deep, home)
		if got != want || (err != nil) != wantErr {
			t.Fatalf("got %q, %v; want %q, error=%t", got, err, want, wantErr)
		}
	}

	check("", false)
	write(home, "personal")
	check("personal", false)
	write(filepath.Join(root, "work"), "jay@enzo.health\n")
	check("jay@enzo.health", false)
	write(filepath.Join(root, "work", "repo"), "../escape")
	check("", true)
	t.Setenv(storageProfileEnv, "override")
	check("override", false)
}
