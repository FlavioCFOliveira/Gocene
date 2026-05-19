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
// PORT NOTE (intentional no-op stub — see [[project-gocene]] / Sprint 56 GOC-3459):
//
// The Java original (org.apache.lucene.util.HotspotVMOptions) reads HotSpot
// JVM tuning flags (UseAVX, UseFMA, UseSVE, MaxVectorSize, UseJVMCICompiler,
// UseXmmI2F) via the com.sun.management.HotSpotDiagnosticMXBean reflective
// MBean bridge. Its sole purpose is to let Lucene's Constants pick SIMD /
// vector code paths based on what the running HotSpot VM advertises.
//
// Go has no HotSpot VM, no JMX, and no MBean equivalent. The mechanism this
// class wraps simply does not exist in our runtime, and the upstream callers
// in Constants.java have already been replaced by build-tag- and
// runtime-feature-detection-driven paths (see util/constants.go for the
// rationale).
//
// We ship this file as a stub so that:
//
//   1. The 1:1 file-level inventory between Lucene and Gocene stays honest
//      (no missing entry under util/ for HotspotVMOptions).
//   2. Any future contributor searching for "HotspotVMOptions" lands here
//      and reads why it is intentionally empty rather than reintroducing a
//      reflective/JVM-coupled abstraction.
//   3. Go callers that mechanically translate Java sites that used
//      Optional<String> HotspotVMOptions.get(name) get a typed, total
//      function whose semantics are "this VM has no such option, ever".
//
// The exported API mirrors the Java surface shape (a package-level
// IsHotspotVM flag and a Get(name) accessor) but is deliberately frozen at
// the empty / false answer. There is no init-time work, no allocation, and
// no goroutine cost.
// -----------------------------------------------------------------------------

package util

// IsHotspotVM reports whether the host Java VM is HotSpot with a readable
// HotSpotDiagnosticMXBean. In Gocene there is no JVM, so this is always
// false. Mirrors the public-visible flag of the same name in the Java
// source.
const IsHotspotVM = false

// GetHotspotVMOption returns the value of a HotSpot VM tuning flag.
//
// In Gocene this is a total no-op: it always returns ("", false), regardless
// of the requested name. The boolean second return models Java's
// Optional<String> — false stands in for Optional.empty(). The function is
// retained so that mechanical ports of Lucene call sites compile and behave
// in the documented "option not present" branch.
//
// Callers that need CPU feature detection must use the Go-native paths in
// the cpu / vector packages, not this function.
func GetHotspotVMOption(_ string) (string, bool) {
	return "", false
}
