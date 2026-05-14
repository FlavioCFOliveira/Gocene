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

package util

import (
	"reflect"
	"sync"
)

// IgnoreRandomChains is the Go analogue of the Java annotation
// org.apache.lucene.util.IgnoreRandomChains.
//
// In Lucene, IgnoreRandomChains is a runtime-retention annotation
// applied to Tokenizer / TokenFilter / CharFilter classes that the
// test framework's random-analysis-chain generator must NOT include
// (typically because they make non-deterministic assumptions about
// their input or have heavy resource requirements). The annotation
// carries an optional reason explaining the exclusion.
//
// Go has no annotation system, so this port models the same intent as
// a small process-wide registry keyed by reflect.Type. Test
// infrastructure registers offending types via Register, with an
// optional reason describing why they must be skipped, and queries the
// registry via IsIgnoredForRandomChains during chain generation. The
// registry is safe for concurrent use.
//
// This is a non-byte-format port: the observable behavior mirrored is
// "this type must be skipped by random chain generators", not the
// annotation's binary layout in the class file (which has no Go
// equivalent).
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/IgnoreRandomChains.java
var ignoreRandomChainsRegistry = struct {
	sync.RWMutex
	reasons map[reflect.Type]string
}{reasons: make(map[reflect.Type]string)}

// RegisterIgnoredForRandomChains marks the dynamic type of sample as
// excluded from random analysis chains. reason is an optional
// human-readable explanation (may be ""). Subsequent calls with the
// same dynamic type overwrite the reason.
//
// Passing a nil sample is a no-op, matching the Java annotation's
// behavior of not affecting any class when not applied.
func RegisterIgnoredForRandomChains(sample any, reason string) {
	if sample == nil {
		return
	}
	t := reflect.TypeOf(sample)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	ignoreRandomChainsRegistry.Lock()
	ignoreRandomChainsRegistry.reasons[t] = reason
	ignoreRandomChainsRegistry.Unlock()
}

// IsIgnoredForRandomChains reports whether the dynamic type of sample
// has been registered as excluded from random analysis chains. nil
// samples always report false.
func IsIgnoredForRandomChains(sample any) bool {
	if sample == nil {
		return false
	}
	t := reflect.TypeOf(sample)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	ignoreRandomChainsRegistry.RLock()
	_, ok := ignoreRandomChainsRegistry.reasons[t]
	ignoreRandomChainsRegistry.RUnlock()
	return ok
}

// IgnoredForRandomChainsReason returns the registered exclusion reason
// for the dynamic type of sample, or ("", false) when the type is not
// registered.
func IgnoredForRandomChainsReason(sample any) (string, bool) {
	if sample == nil {
		return "", false
	}
	t := reflect.TypeOf(sample)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	ignoreRandomChainsRegistry.RLock()
	defer ignoreRandomChainsRegistry.RUnlock()
	reason, ok := ignoreRandomChainsRegistry.reasons[t]
	return reason, ok
}

// UnregisterIgnoredForRandomChains removes the entry for the dynamic
// type of sample. Intended for test teardown so that the global
// registry does not leak between tests. Idempotent.
func UnregisterIgnoredForRandomChains(sample any) {
	if sample == nil {
		return
	}
	t := reflect.TypeOf(sample)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	ignoreRandomChainsRegistry.Lock()
	delete(ignoreRandomChainsRegistry.reasons, t)
	ignoreRandomChainsRegistry.Unlock()
}
