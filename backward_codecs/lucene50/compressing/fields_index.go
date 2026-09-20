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
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/lucene50/compressing/FieldsIndex.java

package compressing

// FieldsIndex is the fields index of a compressing stored-fields or
// term-vectors file: it maps a document ID to the start pointer of the chunk
// that holds it.
//
// Mirrors {@code abstract class FieldsIndex implements Cloneable, Closeable}
// (Lucene 10.5.0, package-private). Go has no abstract classes, so the type is
// an interface; Cloneable contributes Clone and Closeable contributes Close.
type FieldsIndex interface {
	// GetStartPointer gets the start pointer for the block that contains the
	// given docID.
	//
	// Mirrors {@code abstract long getStartPointer(int docID)}. Java's
	// signature declares no checked exception and reports an out-of-range
	// docID with IndexOutOfBoundsException; the Go rendering returns it as an
	// error.
	GetStartPointer(docID int) (int64, error)

	// CheckIntegrity checks the integrity of the index.
	//
	// Mirrors {@code abstract void checkIntegrity() throws IOException}.
	CheckIntegrity() error

	// Clone returns an independent view of this index.
	//
	// Mirrors {@code public abstract FieldsIndex clone()}.
	Clone() FieldsIndex

	// Close releases the resources held by this index.
	//
	// Mirrors java.io.Closeable#close, which FieldsIndex implements.
	Close() error
}
