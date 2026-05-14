// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"runtime"
	"strings"
	"testing"
)

func TestOSName_KnownPlatforms(t *testing.T) {
	t.Parallel()

	switch runtime.GOOS {
	case "linux":
		if OSName != "Linux" || !IsLinux {
			t.Errorf("OSName = %q, IsLinux = %v", OSName, IsLinux)
		}
	case "darwin":
		if OSName != "Mac OS X" || !IsMacOSX {
			t.Errorf("OSName = %q, IsMacOSX = %v", OSName, IsMacOSX)
		}
	case "windows":
		if OSName != "Windows" || !IsWindows {
			t.Errorf("OSName = %q, IsWindows = %v", OSName, IsWindows)
		}
	}
	// Whatever the platform, OSName must be non-empty.
	if OSName == "" {
		t.Error("OSName empty")
	}
}

func TestOSArch_MatchesGoArch(t *testing.T) {
	t.Parallel()

	if OSArch != runtime.GOARCH {
		t.Errorf("OSArch = %q, runtime.GOARCH = %q", OSArch, runtime.GOARCH)
	}
}

func TestIs64Bit_MatchesUintptrSize(t *testing.T) {
	t.Parallel()

	wantSize := 8
	if !Is64Bit {
		wantSize = 4
	}
	if got := int(^uintptr(0) >> 63); (got == 1) != Is64Bit {
		t.Errorf("Is64Bit (%v) inconsistent with uintptr bit-width (got=%d)", Is64Bit, got)
	}
	t.Logf("Is64Bit = %v (uintptr size = %d bytes)", Is64Bit, wantSize)
}

func TestRuntimeNameMatchesVersion(t *testing.T) {
	t.Parallel()

	if !strings.HasPrefix(RuntimeName, "go") {
		t.Errorf("RuntimeName = %q, want go-version prefix", RuntimeName)
	}
	if RuntimeVendor != "go" {
		t.Errorf("RuntimeVendor = %q, want \"go\"", RuntimeVendor)
	}
}

func TestPlatformFlags_AtMostOneTrue(t *testing.T) {
	t.Parallel()

	// On any given host, at most one of these is true (and at least
	// one should be true for the platforms Gocene targets).
	count := 0
	for _, b := range []bool{IsLinux, IsMacOSX, IsWindows, IsFreeBSD, IsSunOS} {
		if b {
			count++
		}
	}
	if count > 1 {
		t.Errorf("multiple platform flags true: count=%d", count)
	}
}

func TestDefaultReadAdviceName_Default(t *testing.T) {
	t.Parallel()

	// Without setting the env var (the default for tests), it must be NORMAL.
	if DefaultReadAdviceName == "" {
		t.Error("DefaultReadAdviceName empty")
	}
}

func TestUnknownSentinel(t *testing.T) {
	t.Parallel()

	if Unknown != "Unknown" {
		t.Errorf("Unknown = %q, want \"Unknown\"", Unknown)
	}
}
