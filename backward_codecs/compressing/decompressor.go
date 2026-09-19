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
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/compressing/Decompressor.java

package compressing

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Decompressor is a decompressor.
//
// Mirrors the abstract class
// org.apache.lucene.backward_codecs.compressing.Decompressor, declared as
// {@code public abstract class Decompressor implements Cloneable}. Go has no
// abstract classes, so the type is an interface; Cloneable contributes Clone.
type Decompressor interface {
	// Decompress bytes that were stored between offsets offset and
	// offset+length in the original stream from the compressed stream in to
	// bytes. After returning, the length of bytes (bytes.Length) must be
	// equal to length. Implementations of this method are free to resize
	// bytes depending on their needs.
	//
	//	in            the input that stores the compressed stream
	//	originalLength the length of the original data (before compression)
	//	offset        bytes before this offset do not need to be decompressed
	//	length        bytes after {@code offset + length} do not need to be
	//	              decompressed
	//	bytes         a BytesRef where to store the decompressed data
	//
	// Mirrors {@code public abstract void decompress(DataInput in,
	// int originalLength, int offset, int length, BytesRef bytes) throws
	// IOException}.
	Decompress(in store.DataInput, originalLength, offset, length int, bytes *util.BytesRef) error

	// Clone returns an independent copy of this decompressor.
	//
	// Mirrors {@code public abstract Decompressor clone()}.
	Clone() Decompressor
}
