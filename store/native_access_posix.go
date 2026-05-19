// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly
// +build linux darwin freebsd netbsd openbsd dragonfly

package store

// GetNativeAccess returns the posix-backed NativeAccess singleton built in
// posix_native_access.go's init. The (impl, true) shape is the Go-idiomatic
// equivalent of the upstream Optional.of(INSTANCE) returned by
// PosixNativeAccess.getInstance() on supported platforms.
func GetNativeAccess() (NativeAccess, bool) {
	return posixInstance, true
}
