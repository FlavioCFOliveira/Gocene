// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package highlight is the Sprint 114 T14 (rmp 4622) binary-compatibility
// harness for Gocene's highlight/* surface against artefacts produced by
// Apache Lucene 10.4.0. Per-file tests cite their audit rows verbatim.
//
// The corpora are produced by two new Lucene-side scenarios shipped
// alongside this package:
//
//	highlight-offset-corpus           -> highlights.tsv     (UH snippets)
//	fast-vector-highlight-phrases     -> fvh-phrases.tsv    (FVH phrases)
//
// The package itself carries no build tag; only the per-file tests are
// gated by //go:build compat so the production module never picks up a
// runtime dependency on the Java harness jar.
package highlight

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by Sprint 114 acceptance
// criteria. Tests iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 second canary (decimal 912559).
}

// Scenario / TSV names registered by the Java harness for T14.
const (
	ScenarioHighlightOffsetCorpus = "highlight-offset-corpus"
	ScenarioFvhPhrases            = "fast-vector-highlight-phrases"
	ScenarioTermVectors           = "term-vectors-format"

	tsvHighlights = "highlights.tsv"
	tsvFvhPhrases = "fvh-phrases.tsv"
)

// snippetRow mirrors HighlightOffsetCorpusScenario.Row on the Java side.
type snippetRow struct {
	queryID      string
	docID        string
	snippetIndex int
	snippetText  string
}

// phraseRow mirrors FastVectorHighlightPhrasesScenario.Row on the Java side.
type phraseRow struct {
	queryID     string
	docID       string
	phraseIndex int
	phraseText  string
	startOffset int
	endOffset   int
}

// requireHarness skips when the Java fixture harness jar is unreachable.
func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

// generate runs `gen` into a fresh t.TempDir() and returns the path.
func generate(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	requireHarness(t)
	dir := t.TempDir()
	if err := gcompat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

// runHarness invokes the jar with args and returns stdout. Non-zero exits
// surface as Go errors with stderr attached.
func runHarness(t *testing.T, args ...string) (string, error) {
	t.Helper()
	jar, err := gcompat.Locate()
	if err != nil {
		return "", err
	}
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return stdout.String(), &harnessError{args: args, err: runErr, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

type harnessError struct {
	args   []string
	err    error
	stderr string
}

func (e *harnessError) Error() string {
	return "java -jar lucene-fixtures.jar " + strings.Join(e.args, " ") +
		": " + e.err.Error() + " (stderr: " + strings.TrimSpace(e.stderr) + ")"
}

func (e *harnessError) Unwrap() error { return e.err }

func readFileBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s/%s: %v", dir, name, err)
	}
	return b
}

// unescapeTSV reverses the four-character escape sequence emitted by the
// Java side (\\ \t \n \r).
func unescapeTSV(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			n := s[i+1]
			i++
			switch n {
			case '\\':
				sb.WriteByte('\\')
			case 't':
				sb.WriteByte('\t')
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(n)
			}
		} else {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// openTSV opens dir/name and returns a scanner ready to iterate.
func openTSV(t *testing.T, dir, name string) (*os.File, *bufio.Scanner) {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	return f, sc
}

// readHighlightsTSV parses dir/highlights.tsv. Comment lines (#) and
// empty lines are skipped.
func readHighlightsTSV(t *testing.T, dir string) []snippetRow {
	t.Helper()
	f, sc := openTSV(t, dir, tsvHighlights)
	defer f.Close()
	var out []snippetRow
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			t.Fatalf("%s: malformed row %q", tsvHighlights, line)
		}
		idx, err := strconv.Atoi(cols[2])
		if err != nil {
			t.Fatalf("%s: parse snippet_index %q: %v", tsvHighlights, cols[2], err)
		}
		out = append(out, snippetRow{
			queryID: cols[0], docID: cols[1],
			snippetIndex: idx, snippetText: unescapeTSV(cols[3]),
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", tsvHighlights, err)
	}
	return out
}

// readFvhPhrasesTSV parses dir/fvh-phrases.tsv.
func readFvhPhrasesTSV(t *testing.T, dir string) []phraseRow {
	t.Helper()
	f, sc := openTSV(t, dir, tsvFvhPhrases)
	defer f.Close()
	var out []phraseRow
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 6 {
			t.Fatalf("%s: malformed row %q", tsvFvhPhrases, line)
		}
		idx, err := strconv.Atoi(cols[2])
		if err != nil {
			t.Fatalf("%s: parse phrase_index %q: %v", tsvFvhPhrases, cols[2], err)
		}
		so, err := strconv.Atoi(cols[4])
		if err != nil {
			t.Fatalf("%s: parse start_offset %q: %v", tsvFvhPhrases, cols[4], err)
		}
		eo, err := strconv.Atoi(cols[5])
		if err != nil {
			t.Fatalf("%s: parse end_offset %q: %v", tsvFvhPhrases, cols[5], err)
		}
		out = append(out, phraseRow{
			queryID: cols[0], docID: cols[1],
			phraseIndex: idx, phraseText: unescapeTSV(cols[3]),
			startOffset: so, endOffset: eo,
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: scan: %v", tsvFvhPhrases, err)
	}
	return out
}
