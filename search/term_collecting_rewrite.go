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

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/TermCollectingRewrite.java

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermCollector is the Go equivalent of TermCollectingRewrite.TermCollector.
// Implementations enumerate matched terms during multi-term query rewriting.
//
// Mirrors org.apache.lucene.search.TermCollectingRewrite.TermCollector.
type TermCollector interface {
	// SetReaderContext is called once per leaf before term iteration begins.
	SetReaderContext(topCtx index.IndexReaderContext, leafCtx *index.LeafReaderContext)
	// SetNextEnum is called with the filtered TermsEnum for the current leaf.
	SetNextEnum(termsEnum index.TermsEnum) error
	// Collect is called for each matched term. Returning false stops collection
	// across all remaining leaves.
	Collect(term *index.Term) (bool, error)
}

// BaseTermCollector holds the reader context fields shared by all TermCollector
// implementations. Embed it in a concrete collector struct and supply
// SetNextEnum and Collect methods.
//
// Mirrors the protected readerContext / topReaderContext fields on
// TermCollectingRewrite.TermCollector (Lucene 10.4.0).
type BaseTermCollector struct {
	// ReaderContext is the current leaf context (updated by SetReaderContext).
	ReaderContext *index.LeafReaderContext
	// TopReaderContext is the top-level context (updated by SetReaderContext).
	TopReaderContext index.IndexReaderContext
}

// SetReaderContext records the current leaf and top-level contexts.
func (b *BaseTermCollector) SetReaderContext(topCtx index.IndexReaderContext, leafCtx *index.LeafReaderContext) {
	b.TopReaderContext = topCtx
	b.ReaderContext = leafCtx
}

// CollectTerms iterates every leaf of reader, obtains a filtered TermsEnum for
// query's field from each leaf, and calls collector for each matched term.
//
// Mirrors TermCollectingRewrite.collectTerms (Lucene 10.4.0).
//
// Deviations from Java:
//   - The reader parameter is index.IndexReaderInterface (not the minimal
//     search.IndexReader) because leaf access requires GetContext / Leaves,
//     which are only defined on the full index reader.
//   - AttributeSource is omitted: Gocene's GetTermsEnum does not yet accept
//     an AttributeSource.
//   - If query does not implement MultiTermQueryTermsEnumProvider, the field's
//     full TermsEnum is used instead (no term filtering).
func CollectTerms(reader index.IndexReaderInterface, query *MultiTermQuery, collector TermCollector) error {
	topCtx, err := index.GetReaderContext(reader)
	if err != nil {
		return err
	}
	leaves := index.GetLeafReaderContexts(topCtx)

	provider, hasProvider := any(query).(MultiTermQueryTermsEnumProvider)

	for _, leafCtx := range leaves {
		leafReader := leafCtx.LeafReader()
		if leafReader == nil {
			continue
		}

		terms, err := leafReader.Terms(query.field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}

		var termsEnum index.TermsEnum
		if hasProvider {
			termsEnum, err = provider.GetTermsEnum(terms)
		} else {
			termsEnum, err = terms.GetIterator()
		}
		if err != nil {
			return err
		}
		if termsEnum == nil {
			continue
		}
		// Skip the EMPTY sentinel (mirrors "if (termsEnum == TermsEnum.EMPTY) continue").
		if _, empty := termsEnum.(*index.EmptyTermsEnum); empty {
			continue
		}

		collector.SetReaderContext(topCtx, leafCtx)
		if err := collector.SetNextEnum(termsEnum); err != nil {
			return err
		}

		for {
			term, err := termsEnum.Next()
			if err != nil {
				return err
			}
			if term == nil {
				break
			}
			ok, err := collector.Collect(term)
			if err != nil {
				return err
			}
			if !ok {
				return nil // caller requested early stop
			}
		}
	}
	return nil
}
