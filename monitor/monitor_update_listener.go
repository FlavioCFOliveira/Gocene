// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

// MonitorUpdateListener is notified of lifecycle events on the Monitor's query index.
//
// Port of org.apache.lucene.monitor.MonitorUpdateListener.
type MonitorUpdateListener interface {
	// AfterUpdate is called after a set of queries have been added to the query index.
	AfterUpdate(updates []*MonitorQuery)

	// AfterDelete is called after a set of queries have been deleted from the query index.
	AfterDelete(queryIDs []string)

	// AfterClear is called after all queries have been removed from the query index.
	AfterClear()

	// OnPurge is called after the Monitor's query cache has been purged of deleted queries.
	OnPurge()

	// OnPurgeError is called if there was an error purging the query cache.
	OnPurgeError(err error)
}

// NoopMonitorUpdateListener is a MonitorUpdateListener that does nothing.
// Embed it to get default (no-op) implementations.
type NoopMonitorUpdateListener struct{}

func (NoopMonitorUpdateListener) AfterUpdate(_ []*MonitorQuery) {}
func (NoopMonitorUpdateListener) AfterDelete(_ []string)        {}
func (NoopMonitorUpdateListener) AfterClear()                   {}
func (NoopMonitorUpdateListener) OnPurge()                      {}
func (NoopMonitorUpdateListener) OnPurgeError(_ error)          {}
