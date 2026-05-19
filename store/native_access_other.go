// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly
// +build !linux,!darwin,!freebsd,!netbsd,!openbsd,!dragonfly

package store

// GetNativeAccess returns (nil, false) on platforms without a POSIX madvise
// wiring (Windows, plan9, solaris, aix, js/wasm, etc.). This mirrors the
// upstream Optional.empty() returned by NativeAccess.getImplementation()
// when no platform-specific subclass succeeds in its static initialiser.
func GetNativeAccess() (NativeAccess, bool) {
	return nil, false
}
