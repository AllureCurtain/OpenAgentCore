package runtimefs

import "testing"

func TestWindowsLocalPathsRejectAlternateSpellings(t *testing.T) {
	for _, path := range []string{`C:\private\file:stream`, `C:\private\name.`, `C:\private\name `, `\\?\C:\private`, `\\.\pipe\name`, `C:relative`} {
		if ValidateLocalPath(path) == nil {
			t.Fatalf("unsafe Windows path accepted: %q", path)
		}
	}
	for _, path := range []string{`C:\private\root`, `\\server\share\private`} {
		if err := ValidateLocalPath(path); err != nil {
			t.Fatal("native path rejected", err)
		}
	}
}
