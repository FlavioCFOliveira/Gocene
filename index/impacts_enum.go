// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// ImpactsEnum is the PostingsEnum extension that also exposes ImpactsSource.
// Mirrors org.apache.lucene.index.ImpactsEnum from Apache Lucene 10.5.0.
//
// Lucene defines an abstract class; Gocene uses an interface to align with
// how PostingsEnum and ImpactsSource are modelled. Implementations must
// satisfy both PostingsEnum and ImpactsSource.
type ImpactsEnum interface {
	PostingsEnum
	ImpactsSource
}
