// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Lucene's Constants exposes JVM-specific values
// (java.vm.vendor, java.vm.name, HotspotVMOptions.UseAVX, etc.). None
// of those have meaningful Go analogues — Go has neither HotSpot, nor
// a security manager, nor system properties.
//
// The Go port retains the entries that are observable from a Go
// process: GOOS-derived OS flags, GOARCH-derived bit-width and CPU
// architecture flags, and Go-runtime metadata in place of JVM vendor.
// Hotspot-vector heuristics are dropped: SIMD/FMA paths in Gocene are
// guarded by build tags and runtime feature detection in their own
// package, not by hard-coded constants here.
//
// DEFAULT_READADVICE is kept as the string identifier; the typed enum
// lives in the store package and is wired up when store needs it.
// -----------------------------------------------------------------------------

package util

import (
	"os"
	"runtime"
	"strings"
)

// Unknown is the sentinel returned when an environment value cannot be
// determined. Mirrors the private UNKNOWN constant in Java.
const Unknown = "Unknown"

// OS detection. Values are derived from runtime.GOOS at package init
// and remain stable for the lifetime of the process. Mirrors LINUX,
// WINDOWS, MAC_OS_X, FREE_BSD from Java's Constants. SUN_OS has no
// modern relevance and is kept as a constant false.
var (
	// OSName is the host operating system, in the Lucene style ("Linux",
	// "Mac OS X", "Windows", "FreeBSD", etc.). Derived from
	// runtime.GOOS, not from any environment variable.
	OSName = osName()

	// OSArch is the host architecture, equivalent to runtime.GOARCH
	// ("amd64", "arm64", ...). Mirrors os.arch in Java.
	OSArch = runtime.GOARCH

	// IsLinux reports whether the binary is running on Linux.
	IsLinux = runtime.GOOS == "linux"

	// IsWindows reports whether the binary is running on Windows.
	IsWindows = runtime.GOOS == "windows"

	// IsMacOSX reports whether the binary is running on macOS.
	IsMacOSX = runtime.GOOS == "darwin"

	// IsFreeBSD reports whether the binary is running on FreeBSD.
	IsFreeBSD = runtime.GOOS == "freebsd"

	// IsSunOS reports whether the binary is running on Solaris/SunOS.
	IsSunOS = runtime.GOOS == "solaris" || runtime.GOOS == "illumos"
)

// Runtime metadata, replacing Java's JVM_VENDOR / JVM_NAME / JAVA_VENDOR.
// The Go runtime is uniformly identified as "go"; the variables exist
// so callers porting from Java can find an analogue if they need to
// log the runtime in question.
var (
	// RuntimeVendor identifies the Go toolchain vendor. The Go runtime
	// has no concept of multiple vendors for the same Go version, so
	// this reports "go".
	RuntimeVendor = "go"

	// RuntimeName reports the Go runtime version string, e.g.
	// "go1.24.3". Mirrors JVM_NAME / JAVA_VENDOR semantically.
	RuntimeName = runtime.Version()
)

// Is64Bit reports whether the process runs on a 64-bit architecture.
// Mirrors JRE_IS_64BIT in Java; on Go, the size is a compile-time
// fact derived from the unsafe.Sizeof of a uintptr.
const Is64Bit = strconvIs64Bit

// strconvIs64Bit is computed at compile time via the uintptr-sized
// constant trick. On 64-bit targets ^uintptr(0) >> 63 is 1 (so the
// const becomes true); on 32-bit it is 0 (so the const becomes
// false). Keeping it as a typed constant avoids a runtime branch.
const strconvIs64Bit = ^uintptr(0)>>63 == 1

// osName returns a Lucene-style operating system name derived from
// runtime.GOOS, falling back to Unknown for platforms without a
// canonical Lucene mapping.
func osName() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux"
	case "darwin":
		return "Mac OS X"
	case "windows":
		return "Windows"
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	case "netbsd":
		return "NetBSD"
	case "dragonfly":
		return "DragonFly"
	case "solaris":
		return "SunOS"
	case "illumos":
		return "illumos"
	default:
		return Unknown
	}
}

// DefaultReadAdviceName is the environment-driven default for index
// read advice. The typed enum lives in the store package; this is the
// string lookup key. Mirrors DEFAULT_READADVICE in Java's Constants,
// but reads from the LUCENE_DEFAULT_READ_ADVICE environment variable
// instead of a System.getProperty entry.
var DefaultReadAdviceName = defaultReadAdviceName()

func defaultReadAdviceName() string {
	if v, ok := os.LookupEnv("LUCENE_DEFAULT_READ_ADVICE"); ok {
		return strings.ToUpper(strings.TrimSpace(v))
	}
	return "NORMAL"
}
