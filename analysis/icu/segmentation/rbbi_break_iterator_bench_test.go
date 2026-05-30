// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import "testing"

// benchInput is a representative mixed-script paragraph (Latin, Myanmar, CJK).
const benchInput = "The quick brown fox jumps over 4,600 lazy dogs. " +
	"သက်ဝင်လှုပ်ရှားစေပြီး ကြောင်းကြောင်း " +
	"中文测试一二三四五六七八九十 hello world 안녕하세요"

// benchAll runs an RBBIBreakIterator over the full input until Done, returning
// the number of boundaries — used to keep the loop from being optimised away.
func benchAll(bi *RBBIBreakIterator, r []rune) int {
	bi.SetText(r, 0, len(r))
	n := 0
	for bi.Next() != Done {
		n++
	}
	return n
}

// BenchmarkRBBIBreakIterator_Default measures forward execution over the
// Default.brk rules for a mixed-script paragraph.
func BenchmarkRBBIBreakIterator_Default(b *testing.B) {
	dict, err := LoadEmbeddedBRK(EmbeddedDefaultBRKName)
	if err != nil {
		b.Fatal(err)
	}
	data, err := dict.RBBIData()
	if err != nil {
		b.Fatal(err)
	}
	bi := newRBBIBreakIterator(data)
	r := []rune(benchInput)

	b.ReportAllocs()
	b.ResetTimer()
	var sink int
	for i := 0; i < b.N; i++ {
		sink += benchAll(bi, r)
	}
	_ = sink
}

// BenchmarkRBBIBreakIterator_Myanmar measures the MyanmarSyllable.brk path.
func BenchmarkRBBIBreakIterator_Myanmar(b *testing.B) {
	dict, err := LoadEmbeddedBRK(EmbeddedMyanmarSyllableBRKName)
	if err != nil {
		b.Fatal(err)
	}
	data, err := dict.RBBIData()
	if err != nil {
		b.Fatal(err)
	}
	bi := newRBBIBreakIterator(data)
	r := []rune("သက်ဝင်လှုပ်ရှားစေပြီး ကြောင်းကြောင်း ကောင်းကောင်း ကျော်ကျော်")

	b.ReportAllocs()
	b.ResetTimer()
	var sink int
	for i := 0; i < b.N; i++ {
		sink += benchAll(bi, r)
	}
	_ = sink
}
