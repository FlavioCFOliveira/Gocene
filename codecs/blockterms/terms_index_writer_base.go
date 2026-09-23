// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package blockterms

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsIndexWriterBase is the Go port of
// org.apache.lucene.codecs.blockterms.TermsIndexWriterBase (Apache Lucene
// 10.5.0): the base of the terms index implementations that plug into
// [BlockTermsWriter].
//
// Java's abstract class implements Closeable; the Go port declares Close as
// part of the interface. The IOException of the Java methods is returned as an
// error.
//
// @lucene.experimental
type TermsIndexWriterBase interface {
	// AddField starts the terms index of fieldInfo, whose terms begin at
	// termsFilePointer in the terms dictionary file.
	//
	// Port of TermsIndexWriterBase.addField(FieldInfo, long).
	AddField(fieldInfo *index.FieldInfo, termsFilePointer int64) (FieldWriter, error)
	// Close finishes and releases the terms index.
	//
	// Port of Closeable.close().
	Close() error
}

// FieldWriter is the Go port of the inner abstract class
// TermsIndexWriterBase.FieldWriter: the terms index API for a single field.
type FieldWriter interface {
	// CheckIndexTerm reports whether text must be an index term.
	//
	// Port of FieldWriter.checkIndexTerm(BytesRef, TermStats).
	CheckIndexTerm(text *util.BytesRef, stats codecs.TermStats) (bool, error)
	// Add records the index term text, whose block starts at
	// termsFilePointer.
	//
	// Port of FieldWriter.add(BytesRef, TermStats, long).
	Add(text *util.BytesRef, stats codecs.TermStats, termsFilePointer int64) error
	// Finish ends the field; termsFilePointer is the end of its terms.
	//
	// Port of FieldWriter.finish(long).
	Finish(termsFilePointer int64) error
}
