// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version
//	2.0 (the "License"); you may not use this file except in compliance
//	with the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//	implied. See the License for the specific language governing
//	permissions and limitations under the License.

package compressing

// fieldsIndex is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.FieldsIndex
// (Lucene 10.5.0, FieldsIndex.java, 43 lines):
//
//	abstract class FieldsIndex implements Cloneable, Closeable {
//	  abstract long getBlockID(int docID);
//	  abstract long getBlockStartPointer(long blockID);
//	  abstract long getBlockLength(long blockID);
//	  final long getStartPointer(int docID) {
//	    return getBlockStartPointer(getBlockID(docID));
//	  }
//	  abstract void checkIntegrity() throws IOException;
//	  public abstract FieldsIndex clone();
//	}
//
// The Java class is package-private and abstract with a single concrete
// method, so it renders as an unexported Go interface carrying the abstract
// members; the one final method is fieldsIndexStartPointer below, which no
// implementation may override — exactly the guarantee Java's `final` gives.
//
// Java's I/O errors travel as checked exceptions, so every accessor that can
// fail gains an error return in Go; getBlockID additionally performs
// Objects.checkIndex, which throws rather than returning a sentinel.
type fieldsIndex interface {
	// getBlockID is FieldsIndex#getBlockID: "Get the ID of the block that
	// contains the given docID."
	getBlockID(docID int) (int64, error)

	// getBlockStartPointer is FieldsIndex#getBlockStartPointer: "Get the
	// start pointer of the block with the given ID."
	getBlockStartPointer(blockID int64) (int64, error)

	// getBlockLength is FieldsIndex#getBlockLength: "Get the number of bytes
	// of the block with the given ID."
	getBlockLength(blockID int64) (int64, error)

	// checkIntegrity is FieldsIndex#checkIntegrity: "Check the integrity of
	// the index."
	checkIntegrity() error

	// clone is FieldsIndex#clone. Java declares Cloneable and returns a
	// FieldsIndex; the Go form surfaces the IOException that Java's
	// implementation wraps in an UncheckedIOException.
	clone() (fieldsIndex, error)

	// Close is the java.io.Closeable half of the Java declaration.
	Close() error
}

// fieldsIndexStartPointer is FieldsIndex#getStartPointer(int)
// (FieldsIndex.java:33-35), the class's single final method: "Get the start
// pointer of the block that contains the given docID."
//
//	final long getStartPointer(int docID) {
//	  return getBlockStartPointer(getBlockID(docID));
//	}
//
// It is a package-level function rather than an interface method because Go
// interfaces cannot carry a non-overridable implementation.
func fieldsIndexStartPointer(idx fieldsIndex, docID int) (int64, error) {
	blockID, err := idx.getBlockID(docID)
	if err != nil {
		return 0, err
	}
	return idx.getBlockStartPointer(blockID)
}
