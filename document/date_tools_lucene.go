// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"time"
)

// This file augments DateTools (defined in date_tools.go) with Lucene
// 10.4.0-compatible date<->string serialization. Two specific divergences
// in the pre-existing helpers required new functions rather than
// modifications to avoid breaking shipped Gocene callers:
//
//   - Lucene always operates in GMT/UTC. The pre-existing TimeToString /
//     StringToTime / Round used the input's local Location, producing
//     non-Lucene-compatible byte strings.
//   - Lucene MILLISECOND format is `yyyyMMddHHmmssSSS` (no dot). The
//     pre-existing helper used `yyyyMMddHHmmss.SSS` (with dot).
//
// The new helpers below produce strings that match Apache Lucene 10.4.0
// byte-for-byte.

// lucene date-format strings (Go reference time layout).
const (
	luceneFormatYear        = "2006"
	luceneFormatMonth       = "200601"
	luceneFormatDay         = "20060102"
	luceneFormatHour        = "2006010215"
	luceneFormatMinute      = "200601021504"
	luceneFormatSecond      = "20060102150405"
	luceneFormatMillisecond = "20060102150405.000" // intermediate format
)

// DateToStringLucene mirrors Lucene's DateTools.dateToString(Date, Resolution).
// The returned string uses UTC and the canonical Lucene format for the
// chosen resolution.
func DateToStringLucene(t time.Time, resolution Resolution) string {
	return TimeToStringLucene(t.UnixMilli(), resolution)
}

// TimeToStringLucene mirrors Lucene's DateTools.timeToString(long, Resolution).
// The long input is interpreted as Unix milliseconds since epoch.
func TimeToStringLucene(millis int64, resolution Resolution) string {
	t := time.UnixMilli(millis).UTC()
	switch resolution {
	case ResolutionYear:
		return t.Format(luceneFormatYear)
	case ResolutionMonth:
		return t.Format(luceneFormatMonth)
	case ResolutionDay:
		return t.Format(luceneFormatDay)
	case ResolutionHour:
		return t.Format(luceneFormatHour)
	case ResolutionMinute:
		return t.Format(luceneFormatMinute)
	case ResolutionSecond:
		return t.Format(luceneFormatSecond)
	case ResolutionMillisecond:
		// `yyyyMMddHHmmssSSS` (no decimal separator)
		return fmt.Sprintf("%s%03d", t.Format(luceneFormatSecond), t.Nanosecond()/1_000_000)
	default:
		return t.Format(luceneFormatDay)
	}
}

// StringToDateLucene parses a Lucene-formatted date string back to a UTC
// time.Time. The resolution is inferred from the string length, matching
// Lucene's DateTools.stringToDate(String).
func StringToDateLucene(s string) (time.Time, error) {
	t, err := stringToTimeLuceneImpl(s)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// StringToTimeLucene parses a Lucene-formatted date string and returns
// Unix milliseconds. Mirrors Lucene's DateTools.stringToTime(String).
func StringToTimeLucene(s string) (int64, error) {
	t, err := stringToTimeLuceneImpl(s)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func stringToTimeLuceneImpl(s string) (time.Time, error) {
	var layout string
	switch len(s) {
	case 4:
		layout = luceneFormatYear
	case 6:
		layout = luceneFormatMonth
	case 8:
		layout = luceneFormatDay
	case 10:
		layout = luceneFormatHour
	case 12:
		layout = luceneFormatMinute
	case 14:
		layout = luceneFormatSecond
	case 17:
		// Lucene MILLISECOND = yyyyMMddHHmmssSSS
		secPart, msPart := s[:14], s[14:]
		t, err := time.ParseInLocation(luceneFormatSecond, secPart, time.UTC)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid Lucene date %q: %w", s, err)
		}
		var ms int
		if _, err := fmt.Sscanf(msPart, "%03d", &ms); err != nil {
			return time.Time{}, fmt.Errorf("invalid Lucene millisecond suffix %q: %w", msPart, err)
		}
		return t.Add(time.Duration(ms) * time.Millisecond), nil
	default:
		return time.Time{}, fmt.Errorf("invalid Lucene date length %d for %q", len(s), s)
	}
	return time.ParseInLocation(layout, s, time.UTC)
}

// RoundLucene rounds a time.Time to the specified resolution in UTC.
// Mirrors Lucene's DateTools.round(Date, Resolution).
func RoundLucene(t time.Time, resolution Resolution) time.Time {
	t = t.UTC()
	switch resolution {
	case ResolutionYear:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	case ResolutionMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	case ResolutionDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case ResolutionHour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
	case ResolutionMinute:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
	case ResolutionSecond:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	case ResolutionMillisecond:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), (t.Nanosecond()/1_000_000)*1_000_000, time.UTC)
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
}

// RoundMillisLucene rounds a Unix milliseconds value to the specified
// resolution, mirroring Lucene's DateTools.round(long, Resolution).
func RoundMillisLucene(millis int64, resolution Resolution) int64 {
	return RoundLucene(time.UnixMilli(millis), resolution).UnixMilli()
}
