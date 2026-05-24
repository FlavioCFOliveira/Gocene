// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "time"

// MonitorConfiguration encapsulates configuration settings for a Monitor's query index.
//
// Port of org.apache.lucene.monitor.MonitorConfiguration.
//
// Deviation: Directory / IndexWriter integration is deferred.  The configuration
// retains all tuneable fields and their defaults, but the buildIndexWriter and
// directoryProvider helpers are stubs.
type MonitorConfiguration struct {
	// QueryUpdateBufferSize is how many queries are buffered in memory before
	// being committed to the query index.  Default: 5000.
	QueryUpdateBufferSize int

	// PurgeFrequency is the period between query-cache garbage-collection runs.
	// Default: 5 minutes.
	PurgeFrequency time.Duration

	// QueryDecomposer splits queries into indexable sub-queries.
	QueryDecomposer *QueryDecomposer

	// Serializer converts MonitorQuery objects to/from bytes for persistence.
	// May be nil when persistence is not needed.
	Serializer MonitorQuerySerializer

	// ReadOnly makes the Monitor read-only.
	ReadOnly bool
}

// NewMonitorConfiguration returns a MonitorConfiguration with default values.
func NewMonitorConfiguration() *MonitorConfiguration {
	return &MonitorConfiguration{
		QueryUpdateBufferSize: 5000,
		PurgeFrequency:        5 * time.Minute,
		QueryDecomposer:       NewQueryDecomposer(),
	}
}

// SetQueryUpdateBufferSize sets how many queries are buffered before committing.
func (c *MonitorConfiguration) SetQueryUpdateBufferSize(size int) *MonitorConfiguration {
	c.QueryUpdateBufferSize = size
	return c
}

// SetPurgeFrequency sets the frequency of query-cache garbage collection.
func (c *MonitorConfiguration) SetPurgeFrequency(d time.Duration) *MonitorConfiguration {
	c.PurgeFrequency = d
	return c
}

// SetQueryDecomposer sets the QueryDecomposer.
func (c *MonitorConfiguration) SetQueryDecomposer(qd *QueryDecomposer) *MonitorConfiguration {
	c.QueryDecomposer = qd
	return c
}

// SetSerializer sets the MonitorQuerySerializer for persistence.
func (c *MonitorConfiguration) SetSerializer(s MonitorQuerySerializer) *MonitorConfiguration {
	c.Serializer = s
	return c
}

// SetReadOnly marks the Monitor as read-only.
func (c *MonitorConfiguration) SetReadOnly(readOnly bool) *MonitorConfiguration {
	c.ReadOnly = readOnly
	return c
}
