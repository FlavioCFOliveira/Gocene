// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

/*
Copyright (c) 2001, Dr Martin Porter
Copyright (c) 2004,2005, Richard Boulton
Copyright (c) 2013, Yoshiki Shibukawa
Copyright (c) 2006,2007,2009,2010,2011,2014-2019, Olly Betts
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions
are met:

  1. Redistributions of source code must retain the above copyright notice,
     this list of conditions and the following disclaimer.
  2. Redistributions in binary form must reproduce the above copyright notice,
     this list of conditions and the following disclaimer in the documentation
     and/or other materials provided with the distribution.
  3. Neither the name of the Snowball project nor the names of its contributors
     may be used to endorse or promote products derived from this software
     without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// Package snowball provides the core Snowball stemming infrastructure:
// Among, SnowballProgram, and SnowballStemmer.
//
// This is the Go port of org.tartarus.snowball from Apache Lucene 10.4.0.
package snowball

// Among mirrors org.tartarus.snowball.Among.
//
// It holds one entry in a keyword-table used by the binary-search lookup
// routines in SnowballProgram (find_among / find_among_b). Each entry
// carries the search string, an index to the next shorter matching prefix
// (substring_i), the integer result code, and an optional method callback.
//
// Go deviation: Java uses java.lang.invoke.MethodHandle for the optional
// method; Go uses a plain func(*SnowballProgram) bool instead.
type Among struct {
	// S is the search string as a rune slice (mirrors char[] s in Java).
	S []rune
	// SubstringI is the index to the longest matching substring (-1 if none).
	SubstringI int
	// Result is the integer result returned when this entry matches.
	Result int
	// Method is the optional callback invoked when the string matches.
	// nil means no method (equivalent to Java's null MethodHandle).
	Method func(*SnowballProgram) bool
}

// NewAmong creates an Among with no method callback.
//
// Mirrors: Among(String s, int substring_i, int result)
func NewAmong(s string, substringI int, result int) *Among {
	return &Among{
		S:          []rune(s),
		SubstringI: substringI,
		Result:     result,
	}
}

// NewAmongMethod creates an Among with a method callback.
//
// Mirrors: Among(String s, int substring_i, int result, String methodname,
//
//	MethodHandles.Lookup methodobject)
//
// Go deviation: instead of reflective lookup by name, the caller passes the
// method function directly.
func NewAmongMethod(s string, substringI int, result int, method func(*SnowballProgram) bool) *Among {
	return &Among{
		S:          []rune(s),
		SubstringI: substringI,
		Result:     result,
		Method:     method,
	}
}
