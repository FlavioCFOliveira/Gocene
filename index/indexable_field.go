// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/document"

// IndexableField represents a single field for indexing. IndexWriter consumes
// a sequence of IndexableField as a document.
//
// This is org.apache.lucene.index.IndexableField from Apache Lucene 10.5.0,
// spelled in the Lucene package that declares it. Java's interface sits on
// both sides of an index/document package cycle that Go forbids -- its
// signature names document.StoredValue and document.InvertableType, while
// document.Document names IndexableField -- so the single declaration lives in
// package document and is aliased here. Both spellings name one type with one
// member set; see document.IndexableField for the full rationale.
type IndexableField = document.IndexableField
