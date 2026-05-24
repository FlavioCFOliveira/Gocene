// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

// QueryTimeListener is notified of how long it takes to run individual queries.
//
// Port of org.apache.lucene.monitor.QueryTimeListener.
type QueryTimeListener interface {
	// LogQueryTime records how long (in nanoseconds) it took to run the given query.
	LogQueryTime(queryID string, timeInNanos int64)
}

// QueryTimeListenerFunc is a function-based QueryTimeListener.
type QueryTimeListenerFunc func(queryID string, timeInNanos int64)

// LogQueryTime implements QueryTimeListener.
func (f QueryTimeListenerFunc) LogQueryTime(queryID string, timeInNanos int64) {
	f(queryID, timeInNanos)
}
