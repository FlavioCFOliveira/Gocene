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

// Package compressing provides the abstract compression/decompression
// contracts used by Lucene stored-fields and term-vectors codecs. It is the
// Go port of org.apache.lucene.codecs.compressing.
package compressing

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Compressor is a data compressor.
//
// This is the Go port of the abstract class
// org.apache.lucene.codecs.compressing.Compressor. In the Java source the
// type is an abstract class that additionally implements java.io.Closeable;
// here we use a Go interface with an explicit Close method that mirrors
// AutoCloseable/Closeable's idempotent-release contract.
//
// Implementations are expected to be reused across many Compress calls within
// a single goroutine but must not be shared concurrently across goroutines.
type Compressor interface {
	// Compress reads every remaining byte from buffersInput and writes its
	// compressed representation into out. It is the responsibility of the
	// implementation to add all framing information required by the matching
	// Decompressor so that the decompressor knows when to stop reading from
	// the input stream during reconstruction.
	//
	// buffersInput is positioned at the start of the data to compress; on
	// successful return the entire stream has been consumed.
	Compress(buffersInput store.ByteBuffersDataInput, out store.DataOutput) error

	// Close releases any resources held by the compressor. Calling Close
	// more than once must be safe and a no-op after the first call.
	Close() error
}
