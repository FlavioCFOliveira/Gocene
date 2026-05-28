// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"io"
	"strings"
	"testing"
)

// minimalDic is a well-formed one-entry dictionary body used to drive the
// affix-file parser to completion in the success-path assertions.
const minimalDic = "1\nword\n"

// readersFor wraps dictionary content in the []io.Reader slice that
// NewDictionary expects.
func readersFor(dic string) []io.Reader {
	return []io.Reader{strings.NewReader(dic)}
}

// TestReadAffixFile_MalformedNumericFields verifies that a corrupted numeric
// field in an affix directive yields a non-nil, descriptive error instead of
// silently parsing the count as zero (which would skip the section and produce
// silently-wrong stemming). The upstream Lucene Dictionary throws
// NumberFormatException at the equivalent sites, which propagates out of
// dictionary construction.
func TestReadAffixFile_MalformedNumericFields(t *testing.T) {
	tests := []struct {
		name      string
		affix     string
		wantInErr string
	}{
		{
			name:      "ICONV count",
			affix:     "ICONV x\n",
			wantInErr: "ICONV",
		},
		{
			name:      "OCONV count",
			affix:     "OCONV x\n",
			wantInErr: "OCONV",
		},
		{
			name:      "BREAK count",
			affix:     "BREAK x\n",
			wantInErr: "BREAK",
		},
		{
			name:      "MAP count",
			affix:     "MAP x\n",
			wantInErr: "MAP",
		},
		{
			name:      "MAXNGRAMSUGS value",
			affix:     "MAXNGRAMSUGS x\n",
			wantInErr: "MAXNGRAMSUGS",
		},
		{
			name:      "MAXDIFF value",
			affix:     "MAXDIFF x\n",
			wantInErr: "MAXDIFF",
		},
		{
			name:      "COMPOUNDMIN value",
			affix:     "COMPOUNDMIN x\n",
			wantInErr: "COMPOUNDMIN",
		},
		{
			name:      "COMPOUNDWORDMAX value",
			affix:     "COMPOUNDWORDMAX x\n",
			wantInErr: "COMPOUNDWORDMAX",
		},
		{
			name:      "COMPOUNDRULE count",
			affix:     "COMPOUNDRULE x\n",
			wantInErr: "COMPOUNDRULE",
		},
		{
			name:      "CHECKCOMPOUNDPATTERN count",
			affix:     "CHECKCOMPOUNDPATTERN x\n",
			wantInErr: "CHECKCOMPOUNDPATTERN",
		},
		{
			name:      "AF alias count",
			affix:     "AF x\n",
			wantInErr: "AF alias",
		},
		{
			name:      "AM morph alias count",
			affix:     "AM x\n",
			wantInErr: "AM morph alias",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDictionary(strings.NewReader(tc.affix), nil, false)
			if err == nil {
				t.Fatalf("NewDictionary with malformed %s: got nil error, want a descriptive error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantInErr)
			}
		})
	}
}

// TestReadAffixFile_WellFormedNumericFields is the success-path guard: each
// directive with a valid numeric field must parse without error, proving the
// fix did not change the success path. The dictionary body is well-formed.
func TestReadAffixFile_WellFormedNumericFields(t *testing.T) {
	tests := []struct {
		name  string
		affix string
	}{
		{name: "ICONV zero", affix: "ICONV 0\n"},
		{name: "OCONV zero", affix: "OCONV 0\n"},
		{name: "MAP zero", affix: "MAP 0\n"},
		{name: "MAXNGRAMSUGS", affix: "MAXNGRAMSUGS 7\n"},
		{name: "MAXDIFF", affix: "MAXDIFF 3\n"},
		{name: "COMPOUNDMIN", affix: "COMPOUNDMIN 2\n"},
		{name: "COMPOUNDWORDMAX", affix: "COMPOUNDWORDMAX 4\n"},
		{name: "COMPOUNDRULE", affix: "COMPOUNDRULE 0\n"},
		{name: "CHECKCOMPOUNDPATTERN", affix: "CHECKCOMPOUNDPATTERN 0\n"},
		{name: "AF alias", affix: "AF 0\n"},
		{name: "AM morph alias", affix: "AM 0\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDictionary(strings.NewReader(tc.affix), readersFor(minimalDic), false)
			if err != nil {
				t.Fatalf("NewDictionary with well-formed %s: unexpected error: %v", tc.name, err)
			}
		})
	}
}

// TestCompoundMinClamp confirms the COMPOUNDMIN/COMPOUNDWORDMAX clamp-to-one
// behaviour is preserved for valid-but-out-of-range values (the success path),
// distinct from the new error path for non-numeric values.
func TestCompoundMinClamp(t *testing.T) {
	d, err := NewDictionary(strings.NewReader("COMPOUNDMIN 0\n"), readersFor(minimalDic), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.compoundMin != 1 {
		t.Fatalf("compoundMin = %d, want 1 (zero must clamp to 1)", d.compoundMin)
	}
}
