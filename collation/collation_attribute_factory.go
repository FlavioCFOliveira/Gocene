// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/java/org/apache/lucene/collation/CollationAttributeFactory.java

package collation

import (
	"github.com/FlavioCFOliveira/Gocene/collation/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CollationAttributeFactory is the Go port of
// org.apache.lucene.collation.CollationAttributeFactory.
//
// It extends util.StaticImplementationAttributeFactory to produce
// CollatedTermAttributeImpl instances for the CharTermAttribute /
// TermToBytesRefAttribute slot, and delegates all other attribute types
// to the wrapped factory.
//
// Converts each token into its collation key bytes, enabling
// locale-sensitive sorting and range queries at the index level.
//
// WARNING: Collation keys are only comparable when produced by the same
// Collator implementation. Ensure you use exactly the same Collator at
// index and query time.
type CollationAttributeFactory struct {
	*util.StaticImplementationAttributeFactory
	collator tokenattributes.Collator
}

// NewCollationAttributeFactory creates a CollationAttributeFactory using
// util.DefaultAttributeFactoryInstance as the delegate and the supplied
// Collator for key generation.
//
// The collator should be safe for concurrent access (the Java side clones
// Collator; Go callers are responsible for concurrency safety of their
// implementation).
func NewCollationAttributeFactory(collator tokenattributes.Collator) *CollationAttributeFactory {
	return NewCollationAttributeFactoryWithDelegate(util.DefaultAttributeFactoryInstance, collator)
}

// NewCollationAttributeFactoryWithDelegate creates a CollationAttributeFactory
// using the supplied delegate AttributeFactory for non-collation attributes.
func NewCollationAttributeFactoryWithDelegate(delegate util.AttributeFactory, collator tokenattributes.Collator) *CollationAttributeFactory {
	f := &CollationAttributeFactory{
		collator: collator,
	}
	f.StaticImplementationAttributeFactory = util.NewStaticImplementationAttributeFactory(
		delegate,
		func() util.AttributeImpl {
			return tokenattributes.NewCollatedTermAttributeImpl(collator)
		},
	)
	return f
}

// CreateInstance returns a new CollatedTermAttributeImpl backed by this
// factory's Collator.
func (f *CollationAttributeFactory) CreateInstance() *tokenattributes.CollatedTermAttributeImpl {
	return tokenattributes.NewCollatedTermAttributeImpl(f.collator)
}
