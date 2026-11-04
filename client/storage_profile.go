package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A storage profile is a named store of cookies and localStorage that survives
// restarts. TERMIUM_STORAGE_PROFILE names it, else the nearest .termium-profile
// file above the working directory, else ~/.termium-profile; with none, nothing
// persists.
const (
	storageProfileEnv  = "TERMIUM_STORAGE_PROFILE"
	storageProfileFile = ".termium-profile"
)

var storageProfileName = regexp.MustCompile(`^[A-Za-z0-9._@+-]{1,128}$`)

func resolveStorageProfile(dir, home string) (string, error) {
	if name := os.Getenv(storageProfileEnv); name != "" {
		return validStorageProfile(name, storageProfileEnv)
	}
	for dir != "" {
		if name, ok, err := readStorageProfile(filepath.Join(dir, storageProfileFile)); ok || err != nil {
			return name, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if home != "" {
		if name, ok, err := readStorageProfile(filepath.Join(home, storageProfileFile)); ok || err != nil {
			return name, err
		}
	}
	return "", nil
}

func readStorageProfile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, nil
	}
	name, err := validStorageProfile(strings.TrimSpace(string(data)), path)
	return name, true, err
}

func validStorageProfile(name, source string) (string, error) {
	if !storageProfileName.MatchString(name) || name == "." || name == ".." {
		return "", fmt.Errorf("%s: invalid profile name %q (letters, digits and . _ @ + - only)", source, name)
	}
	return name, nil
}
