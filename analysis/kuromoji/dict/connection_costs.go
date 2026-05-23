// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "github.com/FlavioCFOliveira/Gocene/analysis/morph"

// ConnectionCosts is the kuromoji-specific connection cost matrix. It wraps
// the morph base and adds a singleton loader pattern for the built-in binary
// resource.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.ConnectionCosts from Apache Lucene
// 10.4.0.
//
// Deviation: the Java original loads a pre-built binary resource from the JAR
// classpath. The Go port exposes the morph base directly; loading from
// embedded resources is deferred to the codec sprint.
type ConnectionCosts struct {
	morph.ConnectionCosts
}

// NewConnectionCosts creates a ConnectionCosts wrapping the given morph base.
func NewConnectionCosts(base morph.ConnectionCosts) *ConnectionCosts {
	return &ConnectionCosts{ConnectionCosts: base}
}
