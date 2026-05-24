// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/util"

// MonitorQuerySerializer serializes and deserializes MonitorQuery objects to
// and from byte streams.  Use this for persistent query indexes.
//
// Port of org.apache.lucene.monitor.MonitorQuerySerializer.
//
// Deviation: Gocene's store package exposes InputStreamDataInput /
// OutputStreamDataOutput via a different path than in Java.  The serialization
// format (id, queryString, metadata count, key/value pairs) is preserved but
// the full wire implementation is deferred until store.DataInput round-trip is
// confirmed.  A no-op placeholder is provided to satisfy the interface.
type MonitorQuerySerializer interface {
	// Deserialize reconstructs a MonitorQuery from its byte representation.
	Deserialize(binaryValue *util.BytesRef) (*MonitorQuery, error)

	// Serialize converts a MonitorQuery into its byte representation.
	Serialize(query *MonitorQuery) (*util.BytesRef, error)
}
