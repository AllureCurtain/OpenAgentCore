//go:build unix

package runtimefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnixDefaultStatePermissionsAndUserSelectedFiles(t *testing.T) {
	root, name := privateRoot(t)
	if err := WritePrivateAtomic(root, "credential", []byte("value")); err != nil {
		t.Fatal(err)
	}
	info, err := root.Stat("credential")
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("default credential mode", err)
	}
	if err := os.Chmod(filepath.Join(name, "credential"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivate(root, "credential", 100); err != nil {
		t.Fatal("user-selected mode rejected", err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Link(filepath.Join(name, "credential"), linked); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivate(root, "credential", 100); err != nil {
		t.Fatal("user-selected link rejected", err)
	}
	alias := filepath.Join(name, "alias")
	if err := os.Symlink("credential", alias); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivate(root, "alias", 100); err != nil {
		t.Fatal("user-selected alias rejected", err)
	}
	if err := os.Chmod(name, 0755); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(name); err != nil {
		t.Fatal("user-selected directory mode rejected", err)
	}
}
