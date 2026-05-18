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

// LongFieldNewDistanceFeatureQuery mirrors the static factory
// org.apache.lucene.document.LongField#newDistanceFeatureQuery from
// Apache Lucene 10.4.0 (lucene/core/src/java/org/apache/lucene/document/
// LongField.java).
//
// The factory constructs a [LongDistanceFeatureQuery] and, when the
// weight differs from 1, wraps it in a [BoostQuery]. Mirrors the Java
// branching exactly:
//
//	Query query = new LongDistanceFeatureQuery(field, origin, pivotDistance);
//	if (weight != 1f) {
//	  query = new BoostQuery(query, weight);
//	}
//
// # Divergence from Lucene
//
// In Lucene this factory is a static method on LongField in the
// document package. In Gocene the factory lives in the search package
// so it can reference LongDistanceFeatureQuery (also in search/)
// without introducing a document→search import cycle: the document
// package already depends on index/util but not on search/. Callers
// migrating from Java should rewrite
//
//	LongField.newDistanceFeatureQuery(field, weight, origin, pivot)
//
// as
//
//	search.LongFieldNewDistanceFeatureQuery(field, weight, origin, pivot)
//
// The behavior, type structure, and BoostQuery wrapping are identical.
func LongFieldNewDistanceFeatureQuery(field string, weight float32, origin, pivotDistance int64) (Query, error) {
	q, err := NewLongDistanceFeatureQuery(field, origin, pivotDistance)
	if err != nil {
		return nil, err
	}
	if weight == 1 {
		return q, nil
	}
	return NewBoostQuery(q, weight), nil
}
