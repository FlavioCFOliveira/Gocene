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
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsIndexEnum is the Go port of
// org.apache.lucene.codecs.blockterms.TermsIndexReaderBase.FieldIndexEnum
// (Apache Lucene 10.5.0).
//
// Similar to TermsEnum, except the only "metadata" it reports for a given
// indexed term is the long fileOffset into the main terms dictionary file.
//
// Java declares seek(BytesRef), next() and seek(long ord) as throwing
// IOException; the Go port returns an explicit error in their place. term()
// and ord() do not throw in Java and therefore return a bare value here.
// Java overloads seek(BytesRef)/seek(long); Go has no overloading, so the
// ordinal form is named SeekOrd.
type TermsIndexEnum interface {
	// Seek moves to the "largest" indexed term that is <= term and returns the
	// file pointer index (into the main terms index file) for that term.
	//
	// Port of FieldIndexEnum.seek(BytesRef).
	Seek(term *util.BytesRef) (int64, error)

	// Next advances to the next indexed term and returns its file pointer, or
	// -1 at end.
	//
	// Port of FieldIndexEnum.next().
	Next() (int64, error)

	// Term returns the current indexed term.
	//
	// Port of FieldIndexEnum.term().
	Term() *util.BytesRef

	// SeekOrd moves to the indexed term covering the supplied ordinal and
	// returns its file pointer. Only implemented if the owning
	// TermsIndexReader reports SupportsOrd() == true.
	//
	// Port of FieldIndexEnum.seek(long).
	SeekOrd(ord int64) (int64, error)

	// Ord returns the ordinal of the current indexed term. Only implemented if
	// the owning TermsIndexReader reports SupportsOrd() == true.
	//
	// Port of FieldIndexEnum.ord().
	Ord() int64
}

// TermsIndexReader is the Go port of
// org.apache.lucene.codecs.blockterms.TermsIndexReaderBase
// (Apache Lucene 10.5.0).
//
// BlockTermsReader interacts with an implementation of this interface to
// manage its terms index. The writer accepts indexed terms (many pairs of
// BytesRef text + long fileOffset), and this reader retrieves the nearest
// index term to a provided term text.
//
// Java's TermsIndexReaderBase implements Closeable; the Go port declares
// Close as part of the interface.
type TermsIndexReader interface {
	// GetFieldEnum returns an enumerator over the indexed terms of fieldInfo,
	// or nil when the field has no terms index.
	//
	// Port of TermsIndexReaderBase.getFieldEnum(FieldInfo).
	GetFieldEnum(fieldInfo *schema.FieldInfo) TermsIndexEnum

	// SupportsOrd reports whether TermsIndexEnum.Ord and
	// TermsIndexEnum.SeekOrd are implemented.
	//
	// Port of TermsIndexReaderBase.supportsOrd().
	SupportsOrd() bool

	// Close releases the resources held by this reader.
	//
	// Port of TermsIndexReaderBase.close().
	Close() error
}
