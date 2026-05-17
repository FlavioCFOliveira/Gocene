// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

func TestSegmentFileName(t *testing.T) {
	cases := []struct {
		seg, suffix, ext, want string
	}{
		{"_0", "", "", "_0"},
		{"_0", "", "si", "_0.si"},
		{"_0", "1", "tip", "_0_1.tip"},
		{"_0", "1", "", "_0_1"},
	}
	for _, c := range cases {
		got := SegmentFileName(c.seg, c.suffix, c.ext)
		if got != c.want {
			t.Errorf("SegmentFileName(%q,%q,%q)=%q want %q", c.seg, c.suffix, c.ext, got, c.want)
		}
	}
}

func TestFileNameFromGeneration(t *testing.T) {
	cases := []struct {
		base, ext string
		gen       int64
		want      string
	}{
		{SegmentsPrefix, "", -1, ""},
		{SegmentsPrefix, "", 0, SegmentsPrefix},
		{SegmentsPrefix, "", 1, "segments_1"},
		{SegmentsPrefix, "", 10, "segments_a"},
		{SegmentsPrefix, "", 36, "segments_10"},
		{"_0", "del", 5, "_0_5.del"},
	}
	for _, c := range cases {
		got := FileNameFromGeneration(c.base, c.ext, c.gen)
		if got != c.want {
			t.Errorf("FileNameFromGeneration(%q,%q,%d)=%q want %q", c.base, c.ext, c.gen, got, c.want)
		}
	}
}

func TestMatchesExtension(t *testing.T) {
	if !MatchesExtension("_0.si", "si") {
		t.Errorf("expected match")
	}
	if MatchesExtension("_0.si", "fnm") {
		t.Errorf("expected no match")
	}
}

func TestStripExtension_GetExtension(t *testing.T) {
	if got := StripExtension("_0.si"); got != "_0" {
		t.Errorf("StripExtension=%q", got)
	}
	if got := StripExtension("_0"); got != "_0" {
		t.Errorf("StripExtension noop=%q", got)
	}
	if got := GetExtension("_0.si"); got != "si" {
		t.Errorf("GetExtension=%q", got)
	}
	if got := GetExtension("_0"); got != "" {
		t.Errorf("GetExtension empty=%q", got)
	}
}

func TestParseSegmentName_ParseGeneration(t *testing.T) {
	// "_0.fnm" -> parseSegmentName="_0"; parseGeneration(no '_' after first idx) => 0
	if got := ParseSegmentName("_0.fnm"); got != "_0" {
		t.Errorf("ParseSegmentName(_0.fnm)=%q", got)
	}
	// "_0_5.del" -> parseSegmentName="_0", parseGeneration=5
	if got := ParseSegmentName("_0_5.del"); got != "_0" {
		t.Errorf("ParseSegmentName(_0_5.del)=%q", got)
	}
	if got := ParseGeneration("_0_5.del"); got != 5 {
		t.Errorf("ParseGeneration(_0_5.del)=%d", got)
	}
	// Generation in base-36: "_0_a.del" -> 10
	if got := ParseGeneration("_0_a.del"); got != 10 {
		t.Errorf("ParseGeneration(_0_a.del)=%d", got)
	}
}

func TestStripSegmentName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"_0.fnm", ".fnm"},
		{"_0_5.del", "_5.del"},
		{"plain", "plain"},
	}
	for _, c := range cases {
		if got := StripSegmentName(c.in); got != c.want {
			t.Errorf("StripSegmentName(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestCodecFilePattern(t *testing.T) {
	good := []string{"_0.si", "_a.fnm", "_0_Lucene90_0.tip"}
	bad := []string{"segments_1", "_0", "random.txt"}
	for _, g := range good {
		if !CodecFilePattern.MatchString(g) {
			t.Errorf("expected match for %q", g)
		}
	}
	for _, b := range bad {
		if CodecFilePattern.MatchString(b) {
			t.Errorf("expected no match for %q", b)
		}
	}
}
