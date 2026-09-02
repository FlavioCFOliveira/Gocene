// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "time"

// QueryTimeoutImpl is the default deadline-based QueryTimeout implementation.
// Mirrors org.apache.lucene.index.QueryTimeoutImpl from Apache Lucene 10.4.0.
//
// The implementation captures an absolute deadline at construction time. Once
// the wall clock passes the deadline, ShouldExit returns true.
type QueryTimeoutImpl struct {
	timeoutAt time.Time
}

// NewQueryTimeoutImpl constructs a QueryTimeoutImpl that expires after
// duration from now.
func NewQueryTimeoutImpl(duration time.Duration) *QueryTimeoutImpl {
	return &QueryTimeoutImpl{timeoutAt: time.Now().Add(duration)}
}

// NewQueryTimeoutImplAt constructs a QueryTimeoutImpl that expires at the
// given absolute time.
func NewQueryTimeoutImplAt(deadline time.Time) *QueryTimeoutImpl {
	return &QueryTimeoutImpl{timeoutAt: deadline}
}

// ShouldExit returns true once the deadline has passed.
func (q *QueryTimeoutImpl) ShouldExit() bool {
	return !time.Now().Before(q.timeoutAt)
}

// GetTimeoutAt returns the absolute deadline.
func (q *QueryTimeoutImpl) GetTimeoutAt() time.Time { return q.timeoutAt }

// Reset resets the timeout to expire after duration from now.
func (q *QueryTimeoutImpl) Reset(duration time.Duration) {
	q.timeoutAt = time.Now().Add(duration)
}
