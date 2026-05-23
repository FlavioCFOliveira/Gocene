// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// DiffIt generates patch commands from an already prepared stemmer table.
//
// This is the Go port of org.egothor.stemmer.DiffIt (Lucene 10.4.0).
// In Lucene this is a command-line tool; in Gocene it is exposed as a
// runnable function so that it can be driven programmatically.
type DiffIt struct{}

// get extracts the digit at position i from s, returning 1 on any error.
func diffItGet(i int, s string) int {
	if i >= len(s) {
		return 1
	}
	n, err := strconv.Atoi(string(s[i]))
	if err != nil {
		return 1
	}
	return n
}

// Run reads a stemmer table from r (one line per word, whitespace-separated,
// stem first, inflected forms following) and writes the resulting patch
// commands to w. The costs string encodes ins/del/rep/noop as four digits
// (e.g. "1110").
func (DiffIt) Run(costs string, r io.Reader, w io.Writer) error {
	ins := diffItGet(0, costs)
	del := diffItGet(1, costs)
	rep := diffItGet(2, costs)
	nop := diffItGet(3, costs)
	diff := NewDiffWithCosts(ins, del, rep, nop)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		stem := fields[0]
		fmt.Fprintf(w, "%s -a\n", stem)
		for _, token := range fields[1:] {
			// map to lowercase runes for comparison
			stemRunes := []rune(stem)
			tokenRunes := []rune(token)
			if !runeSliceEqual(stemRunes, tokenRunes) {
				patch := diff.Exec(token, stem)
				fmt.Fprintf(w, "%s %s\n", stem, patch)
			}
		}
	}
	return scanner.Err()
}

func runeSliceEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i, r := range a {
		if unicode.ToLower(r) != unicode.ToLower(b[i]) {
			return false
		}
	}
	return true
}
