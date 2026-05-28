// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
)

// TestConnectionCostsBuilder_MalformedFields verifies that a corrupted
// matrix.def field yields a non-nil, descriptive error instead of silently
// substituting zero (which would corrupt the connection-cost matrix and thus
// segmentation).
func TestConnectionCostsBuilder_MalformedFields(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantInErr string
	}{
		{
			name:      "bad forwardSize",
			input:     "x 2\n",
			wantInErr: "forwardSize",
		},
		{
			name:      "bad backwardSize",
			input:     "2 y\n",
			wantInErr: "backwardSize",
		},
		{
			name:      "bad forwardID in body",
			input:     "2 2\nx 0 5\n",
			wantInErr: "forwardID",
		},
		{
			name:      "bad backwardID in body",
			input:     "2 2\n0 y 5\n",
			wantInErr: "backwardID",
		},
		{
			name:      "bad cost in body",
			input:     "2 2\n0 0 z\n",
			wantInErr: "cost",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dict.ConnectionCostsBuilder{}.Build(strings.NewReader(tc.input))
			if err == nil {
				t.Fatalf("Build(%q): got nil error, want a descriptive error", tc.input)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantInErr)
			}
		})
	}
}

// TestConnectionCostsBuilder_WellFormed is the success-path guard: a valid
// matrix.def must build without error.
func TestConnectionCostsBuilder_WellFormed(t *testing.T) {
	const input = "2 2\n0 0 5\n1 1 -3\n"
	cc, err := dict.ConnectionCostsBuilder{}.Build(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	if cc == nil {
		t.Fatal("Build returned nil ConnectionCosts for valid input")
	}
}
