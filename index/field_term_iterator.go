// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/util"

// fieldTermIterator iterates over terms across multiple fields. The caller
// must check field after each call to Next to detect a field change; the
// iterator guarantees that the same string value is returned for a given
// field, so == comparison is sufficient.
//
// Port of org.apache.lucene.index.FieldTermIterator from Apache Lucene
// 10.4.0. The Java type is package-private and abstract; in Go it is
// modelled as an unexported interface that embeds util.BytesRefIterator
// (the Java parent) plus the two extra accessors field and delGen.
type fieldTermIterator interface {
	util.BytesRefIterator

	// field returns the current field name. It must not be called after
	// the iteration is exhausted. The same string instance is returned for
	// consecutive terms belonging to the same field, allowing == based
	// change detection.
	field() string

	// delGen returns the deletion generation associated with the current
	// term. This is technically a per-iterator property, but the merged
	// iterator (MergedPrefixCodedTermsIterator) needs to know which
	// underlying iterator the current term came from.
	delGen() int64
}
