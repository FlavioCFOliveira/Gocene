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
	"io"
	"sync/atomic"
)

// InfoStream is the Go port of the abstract class
// org.apache.lucene.util.InfoStream. It is a pluggable sink for verbose
// diagnostic messages emitted by IndexWriter, merges, and related
// components. Each call carries a "component" tag so listeners can
// filter by area of the codebase.
//
// Implementations should be safe for concurrent use; callers typically
// guard each Message call with IsEnabled to avoid building expensive
// formatted strings when the stream is silent.
//
// The io.Closer constraint mirrors Java's Closeable: implementations
// may hold OS handles (files, sockets) and must release them when the
// surrounding IndexWriter shuts down. Trivial implementations such as
// NoOpInfoStream return nil from Close.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/InfoStream.java
type InfoStream interface {
	io.Closer

	// Message logs the given message under the given component tag.
	// Implementations should respect IsEnabled and return early when
	// the component is disabled.
	Message(component, message string)

	// IsEnabled reports whether messages tagged with the given
	// component will be emitted by this stream.
	IsEnabled(component string) bool
}

// NoOpInfoStream is a singleton InfoStream that discards every message
// and reports every component as disabled. It mirrors the
// {@code InfoStream.NO_OUTPUT} singleton from Lucene and is the
// default value returned by DefaultInfoStream until a real
// implementation is installed.
//
//nolint:gochecknoglobals // mirrors the NO_OUTPUT singleton in Lucene.
var NoOpInfoStream InfoStream = noOpInfoStream{}

// noOpInfoStream is the concrete type backing NoOpInfoStream.
type noOpInfoStream struct{}

// Message discards the call.
func (noOpInfoStream) Message(string, string) {}

// IsEnabled always returns false.
func (noOpInfoStream) IsEnabled(string) bool { return false }

// Close is a no-op.
func (noOpInfoStream) Close() error { return nil }

// defaultInfoStream holds the process-wide default returned by
// DefaultInfoStream. Atomic operations keep reads on the hot logging
// path lock-free; SetDefaultInfoStream stores under the same atomic.
//
//nolint:gochecknoglobals // mirrors the static Default in Lucene.
var defaultInfoStream atomic.Pointer[InfoStream]

func init() {
	defaultInfoStream.Store(&NoOpInfoStream)
}

// DefaultInfoStream returns the current process-wide default
// InfoStream. The initial value is NoOpInfoStream.
func DefaultInfoStream() InfoStream {
	return *defaultInfoStream.Load()
}

// SetDefaultInfoStream installs s as the process-wide default
// InfoStream. Passing nil restores NoOpInfoStream so the
// IsEnabled/Message contract remains safe for callers.
func SetDefaultInfoStream(s InfoStream) {
	if s == nil {
		defaultInfoStream.Store(&NoOpInfoStream)
		return
	}
	defaultInfoStream.Store(&s)
}
