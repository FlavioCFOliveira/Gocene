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
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
)

// stackHeader reads the first line of runtime.Stack(false) into buf
// and returns the number of bytes written. The first line always
// begins with "goroutine N [...]".
func stackHeader(buf []byte) int {
	return runtime.Stack(buf, false)
}

// printStreamMessageID is the process-wide id counter mirroring Java's
// PrintStreamInfoStream.MESSAGE_ID static AtomicInteger.
//
//nolint:gochecknoglobals // intentional, mirrors static Java field.
var printStreamMessageID atomic.Int64

// PrintStreamInfoStream is an InfoStream that writes messages to an
// io.Writer with the same line layout as
// org.apache.lucene.util.PrintStreamInfoStream:
//
//	<component> <messageID> [<timestamp>; <threadName>]: <message>
//
// The Go port substitutes a pprof "lucene-thread" label (set by
// [NamedThreadFactory]) for Java's Thread.currentThread().getName().
// Goroutines without a label fall back to the literal string
// "goroutine".
type PrintStreamInfoStream struct {
	mu        sync.Mutex
	stream    io.Writer
	messageID int64
	isSystem  bool
}

// NewPrintStreamInfoStream constructs a PrintStreamInfoStream over
// the given writer, assigning a fresh process-wide message id.
func NewPrintStreamInfoStream(w io.Writer) *PrintStreamInfoStream {
	return NewPrintStreamInfoStreamWithID(w, printStreamMessageID.Add(1)-1)
}

// NewPrintStreamInfoStreamWithID constructs a PrintStreamInfoStream
// using an explicit message id. Mirrors the second Java constructor.
func NewPrintStreamInfoStreamWithID(w io.Writer, messageID int64) *PrintStreamInfoStream {
	return &PrintStreamInfoStream{
		stream:    w,
		messageID: messageID,
		isSystem:  isStdStream(w),
	}
}

// isStdStream reports whether w is os.Stdout or os.Stderr.
func isStdStream(w io.Writer) bool {
	return w == os.Stdout || w == os.Stderr
}

// Message writes the given line to the underlying stream, prefixed
// with component / messageID / timestamp / thread name as in the
// Java reference. Safe for concurrent use; serialised via a mutex
// so multi-goroutine logs don't interleave.
//
// Thread-name resolution falls back to a synthesised "goroutine-N"
// string when the caller is not running under a [NamedThreadFactory]
// label context. Callers wanting the real label should prefer
// [PrintStreamInfoStream.MessageCtx].
func (p *PrintStreamInfoStream) Message(component, message string) {
	p.MessageCtx(context.Background(), component, message)
}

// MessageCtx is the context-aware variant of [PrintStreamInfoStream.Message].
// The pprof labels carried by ctx are consulted to recover the
// originating goroutine name (set by [NamedThreadFactory.Run]).
func (p *PrintStreamInfoStream) MessageCtx(ctx context.Context, component, message string) {
	ts := p.timestamp()
	thread := goroutineNameFromCtx(ctx)
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.stream, "%s %d [%s; %s]: %s\n", component, p.messageID, ts, thread, message)
}

// IsEnabled mirrors Java: PrintStreamInfoStream always reports true.
func (p *PrintStreamInfoStream) IsEnabled(component string) bool { return true }

// Close closes the underlying stream unless it is a process std stream
// (os.Stdout/os.Stderr), matching Java's isSystemStream guard.
func (p *PrintStreamInfoStream) Close() error {
	if p.isSystem {
		return nil
	}
	if c, ok := p.stream.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// IsSystemStream reports whether the underlying writer is os.Stdout
// or os.Stderr. Mirrors Java's public isSystemStream() helper.
func (p *PrintStreamInfoStream) IsSystemStream() bool { return p.isSystem }

// MessageID returns the id assigned at construction. Mirrors the
// protected messageID field in Java.
func (p *PrintStreamInfoStream) MessageID() int64 { return p.messageID }

// timestamp returns the current time formatted as RFC 3339 in UTC
// with nanosecond precision — the closest Go analogue to Java's
// Instant.now().toString().
func (p *PrintStreamInfoStream) timestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// goroutineNameFromCtx returns the pprof "lucene-thread" label
// attached to ctx (set by [NamedThreadFactory.Run]). When the label
// is absent the helper synthesises a stable but anonymous fallback
// derived from the goroutine memory address — the closest Go can get
// to Java's Thread.currentThread().getName() without a labelled
// context.
func goroutineNameFromCtx(ctx context.Context) string {
	var name string
	pprof.ForLabels(ctx, func(k, v string) bool {
		if k == "lucene-thread" {
			name = v
			return false
		}
		return true
	})
	if name != "" {
		return name
	}
	return fmt.Sprintf("goroutine-%d", goroutineIDFromStack())
}

// goroutineIDFromStack extracts the runtime goroutine id from the
// first line of runtime.Stack(). Not free, but called only on the
// fallback path when no label is present.
func goroutineIDFromStack() uint64 {
	// runtime.Stack writes "goroutine N [running]:\n...". We parse N.
	var buf [64]byte
	n := stackHeader(buf[:])
	var id uint64
	for i := len("goroutine "); i < n; i++ {
		c := buf[i]
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + uint64(c-'0')
	}
	return id
}

// Compile-time interface check.
var _ InfoStream = (*PrintStreamInfoStream)(nil)
