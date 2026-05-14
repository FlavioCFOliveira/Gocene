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
// Java's NamedThreadFactory wraps Thread creation so every thread
// receives a name of the form "<prefix>-<pool>-thread-<thread#>". The
// canonical use site is ThreadPoolExecutor for index merges and
// background tasks.
//
// Go has no Thread class and no equivalent of Thread.setName(). The
// closest analogue is runtime/pprof.Do which tags the calling
// goroutine with key=value labels visible in CPU/heap profiles and in
// goroutine stack dumps. The Go port therefore exposes a small
// "factory" type whose Run() method launches the supplied function in
// a fresh goroutine under the appropriate pprof label.
//
// The exported API is deliberately minimal:
//   - NewNamedThreadFactory(prefix string) returns a factory.
//   - factory.Run(ctx, fn) launches fn in a new goroutine tagged with
//     a "Lucene"-style name and an incrementing thread counter.
//   - factory.Next() returns the formatted name a new goroutine would
//     receive without launching it (useful for tests and logs).
//
// The numbering rules match Java exactly: a process-wide pool counter
// (atomic) feeds the "<prefix>-<pool#>-thread" tag held by the
// factory; a per-factory thread counter feeds the "<…>-<thread#>"
// suffix on every Run().
// -----------------------------------------------------------------------------

package util

import (
	"context"
	"fmt"
	"runtime/pprof"
	"sync/atomic"
)

// threadPoolNumber mirrors the Java static AtomicInteger of the same
// name. It is shared across all factories so each gets a unique pool
// id.
//
//nolint:gochecknoglobals // intentional, mirrors Java's static field.
var threadPoolNumber atomic.Int64

func init() {
	threadPoolNumber.Store(1)
}

// NamedThreadFactory is the Go port of
// org.apache.lucene.util.NamedThreadFactory. It produces named
// goroutines via runtime/pprof labels.
type NamedThreadFactory struct {
	threadNamePrefix string
	threadNumber     atomic.Int64
}

// NewNamedThreadFactory returns a new factory whose goroutines will be
// tagged with names of the form
// "<sanitised-prefix>-<pool-id>-thread-<thread-id>". A blank prefix is
// normalised to "Lucene", matching the Java behaviour.
func NewNamedThreadFactory(prefix string) *NamedThreadFactory {
	f := &NamedThreadFactory{
		threadNamePrefix: fmt.Sprintf("%s-%d-thread", checkPrefix(prefix), threadPoolNumber.Add(1)-1),
	}
	f.threadNumber.Store(1)
	return f
}

// checkPrefix mirrors the Java NamedThreadFactory.checkPrefix helper.
func checkPrefix(prefix string) string {
	if prefix == "" {
		return "Lucene"
	}
	return prefix
}

// Next returns the name the next goroutine spawned by Run will
// receive. It increments the factory's thread counter — call it
// either as part of a Run, or once when you intend to use the name
// externally (the counter is not rolled back).
func (f *NamedThreadFactory) Next() string {
	return fmt.Sprintf("%s-%d", f.threadNamePrefix, f.threadNumber.Add(1)-1)
}

// Peek returns the name the next goroutine would receive without
// incrementing the counter. Intended for tests and diagnostics.
func (f *NamedThreadFactory) Peek() string {
	return fmt.Sprintf("%s-%d", f.threadNamePrefix, f.threadNumber.Load())
}

// Prefix returns the factory's "<prefix>-<pool#>-thread" string. This
// is the value Java callers see via NamedThreadFactory#threadNamePrefix.
func (f *NamedThreadFactory) Prefix() string {
	return f.threadNamePrefix
}

// Run launches fn in a new goroutine tagged with a pprof label whose
// key is "lucene-thread" and whose value is the freshly minted name.
// When fn returns the goroutine exits.
//
// The supplied context is propagated into the labelled context so fn
// may use it for cancellation; passing nil is permitted (interpreted
// as context.Background()).
func (f *NamedThreadFactory) Run(ctx context.Context, fn func(context.Context)) {
	if ctx == nil {
		ctx = context.Background()
	}
	name := f.Next()
	labels := pprof.Labels("lucene-thread", name)
	go pprof.Do(ctx, labels, fn)
}
