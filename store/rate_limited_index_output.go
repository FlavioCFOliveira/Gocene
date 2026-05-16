// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// RateLimitedIndexOutput is an IndexOutput that throttles its writes using a
// RateLimiter.
//
// This is the Go port of org.apache.lucene.store.RateLimitedIndexOutput from
// Apache Lucene 10.4.0. It is a FilterIndexOutput that tracks the number of
// bytes written since the last rate-limiter pause and invokes
// rateLimiter.Pause once the accumulated count crosses the limiter's
// MinPauseCheckBytes threshold.
//
// Per Lucene's contract, byte slices passed to WriteBytes are forwarded to
// the wrapped output in one shot without intermediate pauses, so the
// instantaneous write rate may briefly exceed the configured limit; the
// average rate over multiple calls stays within bound.
type RateLimitedIndexOutput struct {
	*FilterIndexOutput
	rateLimiter             RateLimiter
	bytesSinceLastPause     int64
	currentMinPauseCheckBytes int64
}

// NewRateLimitedIndexOutput wraps the given IndexOutput so that writes are
// paced by rateLimiter.
func NewRateLimitedIndexOutput(rateLimiter RateLimiter, out IndexOutput) *RateLimitedIndexOutput {
	return &RateLimitedIndexOutput{
		FilterIndexOutput:         NewFilterIndexOutput("RateLimitedIndexOutput("+out.GetName()+")", out.GetName(), out),
		rateLimiter:               rateLimiter,
		currentMinPauseCheckBytes: rateLimiter.GetMinPauseCheckBytes(),
	}
}

// WriteByte writes a single byte and may pause to respect the rate limit.
func (o *RateLimitedIndexOutput) WriteByte(b byte) error {
	o.bytesSinceLastPause++
	o.checkRate()
	return o.FilterIndexOutput.WriteByte(b)
}

// WriteBytes writes all bytes from b, pausing first if the accumulated byte
// count has crossed the limiter's threshold.
func (o *RateLimitedIndexOutput) WriteBytes(b []byte) error {
	o.bytesSinceLastPause += int64(len(b))
	o.checkRate()
	return o.FilterIndexOutput.WriteBytes(b)
}

// WriteBytesN writes the first n bytes from b.
func (o *RateLimitedIndexOutput) WriteBytesN(b []byte, n int) error {
	o.bytesSinceLastPause += int64(n)
	o.checkRate()
	return o.FilterIndexOutput.WriteBytesN(b, n)
}

// WriteShort writes a 16-bit value.
func (o *RateLimitedIndexOutput) WriteShort(v int16) error {
	o.bytesSinceLastPause += 2
	o.checkRate()
	return o.FilterIndexOutput.WriteShort(v)
}

// WriteInt writes a 32-bit value.
func (o *RateLimitedIndexOutput) WriteInt(v int32) error {
	o.bytesSinceLastPause += 4
	o.checkRate()
	return o.FilterIndexOutput.WriteInt(v)
}

// WriteLong writes a 64-bit value.
func (o *RateLimitedIndexOutput) WriteLong(v int64) error {
	o.bytesSinceLastPause += 8
	o.checkRate()
	return o.FilterIndexOutput.WriteLong(v)
}

// checkRate triggers a rate-limiter pause if enough bytes have accumulated.
func (o *RateLimitedIndexOutput) checkRate() {
	if o.bytesSinceLastPause > o.currentMinPauseCheckBytes {
		o.rateLimiter.Pause(o.bytesSinceLastPause)
		o.bytesSinceLastPause = 0
		o.currentMinPauseCheckBytes = o.rateLimiter.GetMinPauseCheckBytes()
	}
}

// Compile-time assertion that RateLimitedIndexOutput satisfies IndexOutput.
var _ IndexOutput = (*RateLimitedIndexOutput)(nil)
