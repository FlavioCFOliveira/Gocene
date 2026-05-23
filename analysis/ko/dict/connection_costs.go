// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// ConnectionCosts is the Korean-specific connection cost matrix for the Viterbi
// decoder.
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.ConnectionCosts
// from Apache Lucene 10.4.0.
//
// Deviation: the Java original loads a pre-built binary resource from the JAR
// classpath. The Go port provides the struct; loading from embedded resources
// is deferred to the nori codec sprint.
type ConnectionCosts struct {
	matrix      []int16
	forwardSize int
}

// NewConnectionCosts creates a ConnectionCosts with the given matrix.
// matrix must have forwardSize*backwardSize entries in row-major order.
func NewConnectionCosts(matrix []int16, forwardSize int) *ConnectionCosts {
	return &ConnectionCosts{matrix: matrix, forwardSize: forwardSize}
}

// Get returns the connection cost between forwardID and backwardID.
func (c *ConnectionCosts) Get(forwardID, backwardID int) int {
	if c.matrix == nil {
		return 0
	}
	idx := backwardID*c.forwardSize + forwardID
	if idx < 0 || idx >= len(c.matrix) {
		return 0
	}
	return int(c.matrix[idx])
}

// defaultConnectionCosts is the zero-value singleton.
var defaultConnectionCosts = &ConnectionCosts{}

// GetConnectionCostsInstance returns the default ConnectionCosts singleton.
//
// Deviation: the Java original loads binary data from the JAR classpath. The
// Go port returns an empty instance; full binary loading is deferred to the
// nori codec sprint.
func GetConnectionCostsInstance() *ConnectionCosts {
	return defaultConnectionCosts
}
