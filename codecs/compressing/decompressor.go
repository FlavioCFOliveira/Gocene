// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package compressing

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Decompressor reconstructs a window of an originally compressed payload.
//
// This is the Go port of the abstract class
// org.apache.lucene.codecs.compressing.Decompressor. The Java type
// additionally implements java.lang.Cloneable; the Go interface exposes
// Clone explicitly so callers can produce independent instances without
// resorting to reflection.
//
// Implementations are expected to be reused across many Decompress calls
// within a single goroutine but must not be shared concurrently across
// goroutines; create independent clones for concurrent reads.
type Decompressor interface {
	// Decompress decodes bytes that were stored between offsets [offset,
	// offset+length) of the original payload, reading from the compressed
	// stream in and writing the result into bytes.
	//
	// Contract (verbatim from Lucene 10.4.0):
	//   - On return, bytes.Length MUST equal length.
	//   - bytes.Offset MUST point to the start of the requested window
	//     within bytes.Bytes.
	//   - Implementations are free to grow bytes.Bytes as needed.
	//
	// originalLength is the total length of the uncompressed payload (i.e.
	// the value used at compression time); it is greater than or equal to
	// offset+length.
	Decompress(in store.DataInput, originalLength, offset, length int, bytes *util.BytesRef) error

	// Clone returns a Decompressor that may be used independently of the
	// receiver. For stateless implementations Clone may return the receiver
	// itself; for implementations that own a per-instance scratch buffer it
	// must produce a fresh instance with its own state.
	Clone() Decompressor
}
