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
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/compressing/Compressor.java

package compressing

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Compressor is a data compressor.
//
// Mirrors the abstract class
// org.apache.lucene.backward_codecs.compressing.Compressor, declared as
// {@code public abstract class Compressor implements Closeable}. Go has no
// abstract classes, so the type is an interface; Closeable contributes Close.
//
// NOTE: this is the backward-codecs Compressor, whose compress() takes a
// byte[] window. The core org.apache.lucene.codecs.compressing.Compressor
// (ported at codecs/compressing) takes a ByteBuffersDataInput instead; the
// two are separate Lucene classes and the signatures differ on purpose.
type Compressor interface {
	// Compress compresses bytes[off:off+len] into out.
	//
	// It is the responsibility of the compressor to add all necessary
	// information so that a Decompressor will know when to stop decompressing
	// bytes from the stream.
	//
	// Mirrors {@code public abstract void compress(byte[] bytes, int off,
	// int len, DataOutput out) throws IOException}.
	Compress(bytes []byte, off, length int, out store.DataOutput) error

	// Close releases the resources held by this compressor.
	//
	// Mirrors java.io.Closeable#close, which Compressor implements.
	Close() error
}
