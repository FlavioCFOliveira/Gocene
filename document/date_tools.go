// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strconv"
	"time"
)

// DateTools provides utility methods for converting dates to strings and vice versa.
// This is the Go port of Lucene's org.apache.lucene.document.DateTools.
type DateTools struct{}

// Resolution defines the resolution for date/time conversion.
type Resolution int

const (
	// ResolutionYear resolution for year
	ResolutionYear Resolution = iota
	// ResolutionMonth resolution for month
	ResolutionMonth
	// ResolutionDay resolution for day
	ResolutionDay
	// ResolutionHour resolution for hour
	ResolutionHour
	// ResolutionMinute resolution for minute
	ResolutionMinute
	// ResolutionSecond resolution for second
	ResolutionSecond
	// ResolutionMillisecond resolution for millisecond
	ResolutionMillisecond
)

// String returns the string representation of the resolution.
func (r Resolution) String() string {
	switch r {
	case ResolutionYear:
		return "YEAR"
	case ResolutionMonth:
		return "MONTH"
	case ResolutionDay:
		return "DAY"
	case ResolutionHour:
		return "HOUR"
	case ResolutionMinute:
		return "MINUTE"
	case ResolutionSecond:
		return "SECOND"
	case ResolutionMillisecond:
		return "MILLISECOND"
	default:
		return "UNKNOWN"
	}
}

// TimeToString converts a time.Time to a string according to the specified resolution.
func TimeToString(t time.Time, resolution Resolution) string {
	switch resolution {
	case ResolutionYear:
		return t.Format("2006")
	case ResolutionMonth:
		return t.Format("200601")
	case ResolutionDay:
		return t.Format("20060102")
	case ResolutionHour:
		return t.Format("2006010215")
	case ResolutionMinute:
		return t.Format("200601021504")
	case ResolutionSecond:
		return t.Format("20060102150405")
	case ResolutionMillisecond:
		return t.Format("20060102150405.000")
	default:
		return t.Format("20060102")
	}
}

// StringToTime parses a string to time.Time according to the specified resolution.
func StringToTime(s string, resolution Resolution) (time.Time, error) {
	switch resolution {
	case ResolutionYear:
		return time.Parse("2006", s)
	case ResolutionMonth:
		return time.Parse("200601", s)
	case ResolutionDay:
		return time.Parse("20060102", s)
	case ResolutionHour:
		return time.Parse("2006010215", s)
	case ResolutionMinute:
		return time.Parse("200601021504", s)
	case ResolutionSecond:
		return time.Parse("20060102150405", s)
	case ResolutionMillisecond:
		return time.Parse("20060102150405.000", s)
	default:
		return time.Parse("20060102", s)
	}
}

// TimeToLong converts a time.Time to a long value (milliseconds since epoch).
func TimeToLong(t time.Time) int64 {
	return t.UnixMilli()
}

// LongToTime converts a long value (milliseconds since epoch) to time.Time.
func LongToTime(millis int64) time.Time {
	return time.UnixMilli(millis)
}

// Round rounds a time.Time to the specified resolution.
func Round(t time.Time, resolution Resolution) time.Time {
	switch resolution {
	case ResolutionYear:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	case ResolutionMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case ResolutionDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case ResolutionHour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case ResolutionMinute:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
	case ResolutionSecond:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, t.Location())
	case ResolutionMillisecond:
		return t
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

// DateTimeField is a field for indexing date/time values.
type DateTimeField struct {
	Field
	value time.Time
}

// NewDateTimeField creates a new DateTimeField.
func NewDateTimeField(name string, value time.Time, stored bool) *DateTimeField {
	// Store as milliseconds since epoch
	millis := value.UnixMilli()
	ft := NewFieldType()
	ft.SetStored(stored)
	ft.SetIndexed(true)

	field, _ := NewField(name, strconv.FormatInt(millis, 10), ft)
	return &DateTimeField{
		Field: *field,
		value: value,
	}
}

// Value returns the time.Time value.
func (f *DateTimeField) Value() time.Time {
	return f.value
}

// String returns a string representation.
func (f *DateTimeField) String() string {
	return fmt.Sprintf("DateTimeField(name=%s, value=%v)", f.name, f.value)
}
