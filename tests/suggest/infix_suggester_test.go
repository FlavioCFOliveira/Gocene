// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
)

type mockInputIterator struct {
	data []struct {
		term     []byte
		weight   int64
		payload  []byte
		contexts [][]byte
	}
	pos int
}

func (m *mockInputIterator) Next() ([]byte, int64, []byte, [][]byte, bool, error) {
	if m.pos >= len(m.data) {
		return nil, 0, nil, nil, false, nil
	}
	d := m.data[m.pos]
	m.pos++
	return d.term, d.weight, d.payload, d.contexts, true, nil
}

func (m *mockInputIterator) HasPayloads() bool { return true }
func (m *mockInputIterator) HasContexts() bool { return true }

func TestAnalyzingInfixSuggester(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "infix-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dir, err := store.FSDirectoryOpen(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := analysis.NewStandardAnalyzer()
	suggester, err := NewAnalyzingInfixSuggester(dir, analyzer)
	if err != nil {
		t.Fatal(err)
	}
	defer suggester.Close()

	iter := &mockInputIterator{
		data: []struct {
			term     []byte
			weight   int64
			payload  []byte
			contexts [][]byte
		}{
			{[]byte("apple"), 10, []byte("payload1"), [][]byte{[]byte("fruit")}},
			{[]byte("banana"), 20, []byte("payload2"), [][]byte{[]byte("fruit")}},
			{[]byte("cherry"), 15, []byte("payload3"), [][]byte{[]byte("berry")}},
		},
	}

	if err := suggester.Build(iter); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		key      string
		contexts [][]byte
		num      int
		want     []string
	}{
		{
			name: "prefix match",
			key:  "app",
			num:  10,
			want: []string{"apple"},
		},
		{
			name: "infix match",
			key:  "ana",
			num:  10,
			want: []string{"banana"},
		},
		{
			name: "multiple matches",
			key:  "a",
			num:  10,
			want: []string{"banana", "apple"}, // sorted by weight (20, 10)
		},
		{
			name:     "context filter",
			key:      "a",
			contexts: [][]byte{[]byte("berry")},
			num:      10,
			want:     []string{"cherry"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := suggester.LookupResults(tt.key, tt.contexts, false, tt.num)
			if err != nil {
				t.Fatal(err)
			}

			var got []string
			for _, r := range results {
				got = append(got, r.Key)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LookupResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlendedInfixSuggester(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "blended-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dir, err := store.FSDirectoryOpen(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := analysis.NewStandardAnalyzer()
	suggester, err := NewBlendedInfixSuggester(dir, analyzer)
	if err != nil {
		t.Fatal(err)
	}
	defer suggester.Close()

	iter := &mockInputIterator{
		data: []struct {
			term     []byte
			weight   int64
			payload  []byte
			contexts [][]byte
		}{
			{[]byte("apple"), 10, nil, nil},
			{[]byte("banana"), 20, nil, nil},
		},
	}

	if err := suggester.Build(iter); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		key  string
		num  int
		want []string
	}{
		{
			name: "blended match",
			key:  "ana",
			num:  10,
			want: []string{"banana"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := suggester.LookupResults(tt.key, nil, false, tt.num)
			if err != nil {
				t.Fatal(err)
			}

			var got []string
			for _, r := range results {
				got = append(got, r.Key)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LookupResults() = %v, want %v", got, tt.want)
			}
		})
	}
}
