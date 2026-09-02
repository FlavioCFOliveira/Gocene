// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"
	"time"
)

// Reference snapshot: 2026-05-17 12:34:56.789 UTC
var refTime = time.Date(2026, 5, 17, 12, 34, 56, 789_000_000, time.UTC)

func TestTimeToStringLucene(t *testing.T) {
	cases := []struct {
		res  Resolution
		want string
	}{
		{ResolutionYear, "2026"},
		{ResolutionMonth, "202605"},
		{ResolutionDay, "20260517"},
		{ResolutionHour, "2026051712"},
		{ResolutionMinute, "202605171234"},
		{ResolutionSecond, "20260517123456"},
		{ResolutionMillisecond, "20260517123456789"},
	}
	for _, c := range cases {
		got := DateToStringLucene(refTime, c.res)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.res, got, c.want)
		}
	}
}

func TestStringToTimeLucene_RoundTrip(t *testing.T) {
	resolutions := []Resolution{
		ResolutionYear, ResolutionMonth, ResolutionDay,
		ResolutionHour, ResolutionMinute, ResolutionSecond, ResolutionMillisecond,
	}
	for _, r := range resolutions {
		s := DateToStringLucene(refTime, r)
		t1, err := StringToDateLucene(s)
		if err != nil {
			t.Errorf("%s: parse error: %v", r, err)
			continue
		}
		// Round refTime to the same resolution before comparing.
		rounded := RoundLucene(refTime, r)
		if !t1.Equal(rounded) {
			t.Errorf("%s: parsed %v != rounded %v", r, t1, rounded)
		}
	}
}

func TestStringToTimeLucene_InvalidLength(t *testing.T) {
	if _, err := StringToDateLucene("invalid"); err == nil {
		t.Fatalf("expected error for invalid length")
	}
}

func TestRoundLucene(t *testing.T) {
	cases := []struct {
		res  Resolution
		want time.Time
	}{
		{ResolutionYear, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ResolutionMonth, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{ResolutionDay, time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)},
		{ResolutionHour, time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)},
		{ResolutionMinute, time.Date(2026, 5, 17, 12, 34, 0, 0, time.UTC)},
		{ResolutionSecond, time.Date(2026, 5, 17, 12, 34, 56, 0, time.UTC)},
		{ResolutionMillisecond, time.Date(2026, 5, 17, 12, 34, 56, 789_000_000, time.UTC)},
	}
	for _, c := range cases {
		if got := RoundLucene(refTime, c.res); !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.res, got, c.want)
		}
	}
}

func TestRoundMillisLucene(t *testing.T) {
	ms := refTime.UnixMilli()
	if RoundMillisLucene(ms, ResolutionDay) != time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC).UnixMilli() {
		t.Fatalf("RoundMillisLucene day mismatch")
	}
}
