// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// ImpactsEnum is an extension of PostingsEnum which also provides information about upcoming impacts.
// Mirrors org.apache.lucene.index.ImpactsEnum from Apache Lucene 10.5.0.
type ImpactsEnum interface {
	PostingsEnum
	ImpactsSource
}
