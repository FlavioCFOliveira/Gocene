// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Command gen-unicode-wb generates the analysis/unicode_wordbreak.gen.go
// file from vendored Unicode 12.1 data files.
//
// The generator reads four UCD files from cmd/gen-unicode-wb/testdata/:
//
//   - WordBreakProperty.txt   (auxiliary/WordBreakProperty.txt)
//   - emoji-data.txt          (emoji/12.1/emoji-data.txt)
//   - LineBreak.txt           (LineBreak.txt)
//   - Scripts.txt             (Scripts.txt)
//
// and emits unicode.RangeTable literals for every Word_Break property
// class plus the Emoji property classes, the four CJK scripts
// (Han, Hiragana, Katakana, Hangul), and the LineBreak:Complex_Context
// (SA) class consumed by Apache Lucene's StandardTokenizerImpl.jflex.
//
// Usage:
//
//	go run ./cmd/gen-unicode-wb
//
// The generator is intentionally hermetic: it does not perform any
// network I/O. Refreshing to a newer Unicode version requires updating
// the vendored .txt files first.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"
)

// unicodeVersion is the Unicode standard version captured by the
// vendored .txt files. It is emitted into the generated file's
// header for traceability.
const unicodeVersion = "12.1.0"

// dataDir holds the vendored UCD files. It is resolved relative to
// the generator's own location so the command works regardless of
// the caller's working directory.
const dataDir = "cmd/gen-unicode-wb/testdata"

// outFile is the path of the generated Go source file.
const outFile = "analysis/unicode_wordbreak.gen.go"

// rangeSet collects code points into a sorted, deduplicated set
// suitable for compaction into a unicode.RangeTable.
type rangeSet struct {
	pts map[rune]struct{}
}

func newRangeSet() *rangeSet {
	return &rangeSet{pts: make(map[rune]struct{}, 256)}
}

func (s *rangeSet) addRange(lo, hi rune) {
	for r := lo; r <= hi; r++ {
		s.pts[r] = struct{}{}
	}
}

func (s *rangeSet) sorted() []rune {
	out := make([]rune, 0, len(s.pts))
	for r := range s.pts {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// compact converts the set into a unicode.RangeTable, splitting the
// 16-bit (R16) and 32-bit (R32) halves at the U+FFFF boundary as
// the Go standard library expects.
func (s *rangeSet) compact() *unicode.RangeTable {
	runes := s.sorted()
	rt := &unicode.RangeTable{}
	if len(runes) == 0 {
		return rt
	}

	// Group consecutive code points (stride==1) into contiguous
	// blocks. UCD blocks are already stride-1, so the simple
	// "merge adjacent" rule is sufficient.
	type block struct{ lo, hi rune }
	blocks := make([]block, 0, 64)
	cur := block{lo: runes[0], hi: runes[0]}
	for _, r := range runes[1:] {
		if r == cur.hi+1 {
			cur.hi = r
			continue
		}
		blocks = append(blocks, cur)
		cur = block{lo: r, hi: r}
	}
	blocks = append(blocks, cur)

	for _, b := range blocks {
		if b.hi <= 0xFFFF {
			rt.R16 = append(rt.R16, unicode.Range16{
				Lo:     uint16(b.lo),
				Hi:     uint16(b.hi),
				Stride: 1,
			})
		} else if b.lo > 0xFFFF {
			rt.R32 = append(rt.R32, unicode.Range32{
				Lo:     uint32(b.lo),
				Hi:     uint32(b.hi),
				Stride: 1,
			})
		} else {
			// Block straddles the BMP boundary: split.
			rt.R16 = append(rt.R16, unicode.Range16{
				Lo:     uint16(b.lo),
				Hi:     0xFFFF,
				Stride: 1,
			})
			rt.R32 = append(rt.R32, unicode.Range32{
				Lo:     uint32(0x10000),
				Hi:     uint32(b.hi),
				Stride: 1,
			})
		}
	}

	// Populate LatinOffset so unicode.Is can short-circuit ASCII.
	for i, r := range rt.R16 {
		if r.Hi > unicode.MaxLatin1 {
			rt.LatinOffset = i
			break
		}
		rt.LatinOffset = i + 1
	}

	return rt
}

// parseUCDLine extracts the code point range and the property name
// from a single non-blank UCD line. Comments and trailing whitespace
// are stripped. Returns ok=false for blank or comment-only lines.
func parseUCDLine(line string) (lo, hi rune, prop string, ok bool, err error) {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, 0, "", false, nil
	}
	parts := strings.SplitN(line, ";", 2)
	if len(parts) != 2 {
		return 0, 0, "", false, fmt.Errorf("malformed line: %q", line)
	}
	cp := strings.TrimSpace(parts[0])
	prop = strings.TrimSpace(parts[1])
	if cp == "" || prop == "" {
		return 0, 0, "", false, fmt.Errorf("malformed line (empty field): %q", line)
	}
	if i := strings.Index(cp, ".."); i >= 0 {
		loVal, perr := strconv.ParseUint(cp[:i], 16, 32)
		if perr != nil {
			return 0, 0, "", false, fmt.Errorf("invalid lo %q: %w", cp[:i], perr)
		}
		hiVal, perr := strconv.ParseUint(cp[i+2:], 16, 32)
		if perr != nil {
			return 0, 0, "", false, fmt.Errorf("invalid hi %q: %w", cp[i+2:], perr)
		}
		return rune(loVal), rune(hiVal), prop, true, nil
	}
	v, perr := strconv.ParseUint(cp, 16, 32)
	if perr != nil {
		return 0, 0, "", false, fmt.Errorf("invalid code point %q: %w", cp, perr)
	}
	return rune(v), rune(v), prop, true, nil
}

// loadProperties parses a UCD file and returns one rangeSet per
// distinct property value. Properties listed in `keep` (if non-nil)
// are the only ones retained; pass nil to accept every property.
func loadProperties(path string, keep map[string]bool) (map[string]*rangeSet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]*rangeSet, 32)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		lo, hi, prop, ok, perr := parseUCDLine(sc.Text())
		if perr != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNum, perr)
		}
		if !ok {
			continue
		}
		if keep != nil && !keep[prop] {
			continue
		}
		rs, found := out[prop]
		if !found {
			rs = newRangeSet()
			out[prop] = rs
		}
		rs.addRange(lo, hi)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// resolveDataPath returns an absolute path to a vendored UCD file,
// trying the module-root path first and falling back to a
// command-local lookup so the generator works from either invocation
// pattern (`go run ./cmd/gen-unicode-wb` vs. `go run .` inside the
// command directory).
func resolveDataPath(name string) (string, error) {
	if _, err := os.Stat(filepath.Join(dataDir, name)); err == nil {
		return filepath.Join(dataDir, name), nil
	}
	if _, err := os.Stat(filepath.Join("testdata", name)); err == nil {
		return filepath.Join("testdata", name), nil
	}
	return "", fmt.Errorf("could not locate %q under %q or ./testdata", name, dataDir)
}

// resolveOutPath returns the absolute path of the file to write.
func resolveOutPath() (string, error) {
	if _, err := os.Stat("analysis"); err == nil {
		return outFile, nil
	}
	// Fall back to repo-root resolution when the command is run from
	// its own directory.
	if _, err := os.Stat(filepath.Join("..", "..", "analysis")); err == nil {
		return filepath.Join("..", "..", outFile), nil
	}
	return "", fmt.Errorf("could not locate analysis/ relative to cwd")
}

// renderRangeTable returns the Go source representation of a
// unicode.RangeTable literal.
func renderRangeTable(rt *unicode.RangeTable) string {
	var sb strings.Builder
	sb.WriteString("&unicode.RangeTable{\n")
	if len(rt.R16) > 0 {
		sb.WriteString("\t\tR16: []unicode.Range16{\n")
		for _, r := range rt.R16 {
			fmt.Fprintf(&sb, "\t\t\t{Lo: 0x%04X, Hi: 0x%04X, Stride: %d},\n", r.Lo, r.Hi, r.Stride)
		}
		sb.WriteString("\t\t},\n")
	}
	if len(rt.R32) > 0 {
		sb.WriteString("\t\tR32: []unicode.Range32{\n")
		for _, r := range rt.R32 {
			fmt.Fprintf(&sb, "\t\t\t{Lo: 0x%06X, Hi: 0x%06X, Stride: %d},\n", r.Lo, r.Hi, r.Stride)
		}
		sb.WriteString("\t\t},\n")
	}
	if rt.LatinOffset > 0 {
		fmt.Fprintf(&sb, "\t\tLatinOffset: %d,\n", rt.LatinOffset)
	}
	sb.WriteString("\t}")
	return sb.String()
}

// tableEntry describes one named RangeTable to emit.
type tableEntry struct {
	VarName string
	Doc     string
	Source  string
	Body    string
}

// runeCount returns the total number of code points in rt; used for
// the per-table doc comment.
func runeCount(rt *unicode.RangeTable) int {
	n := 0
	for _, r := range rt.R16 {
		n += int(r.Hi-r.Lo)/int(r.Stride) + 1
	}
	for _, r := range rt.R32 {
		n += int(r.Hi-r.Lo)/int(r.Stride) + 1
	}
	return n
}

// addTable appends one table entry to the output list, preserving
// the deterministic emission order.
func addTable(out *[]tableEntry, varName, source, propName string, rt *unicode.RangeTable) {
	body := renderRangeTable(rt)
	doc := fmt.Sprintf(
		"%s contains all code points whose %s property equals %q (%d code points).",
		varName, source, propName, runeCount(rt),
	)
	*out = append(*out, tableEntry{
		VarName: varName,
		Doc:     doc,
		Source:  source,
		Body:    body,
	})
}

// emit writes the final generated Go source file.
func emit(path string, tables []tableEntry) error {
	tpl := template.Must(template.New("gen").Parse(genTemplate))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data := struct {
		Version   string
		Generated string
		Tables    []tableEntry
	}{
		Version:   unicodeVersion,
		Generated: time.Now().UTC().Format(time.RFC3339),
		Tables:    tables,
	}
	return tpl.Execute(f, data)
}

const genTemplate = `// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Code generated by cmd/gen-unicode-wb; DO NOT EDIT.
//
// Unicode version: {{.Version}}
// Source files (vendored under cmd/gen-unicode-wb/testdata/):
//   - WordBreakProperty.txt  (auxiliary/WordBreakProperty.txt)
//   - emoji-data.txt         (emoji/12.1/emoji-data.txt)
//   - LineBreak.txt          (LineBreak.txt)
//   - Scripts.txt            (Scripts.txt)
//
// Regenerate with:  go run ./cmd/gen-unicode-wb
//
// Generated: {{.Generated}}

package analysis

import "unicode"

var (
{{range .Tables}}	// {{.Doc}}
	{{.VarName}} = {{.Body}}

{{end}})
`

func main() {
	wbProps := map[string]bool{
		"ALetter": true, "CR": true, "Double_Quote": true,
		"Extend": true, "ExtendNumLet": true, "Format": true,
		"Hebrew_Letter": true, "Katakana": true, "LF": true,
		"MidLetter": true, "MidNum": true, "MidNumLet": true,
		"Newline": true, "Numeric": true, "Regional_Indicator": true,
		"Single_Quote": true, "WSegSpace": true, "ZWJ": true,
	}
	wbPath, err := resolveDataPath("WordBreakProperty.txt")
	if err != nil {
		fatal(err)
	}
	wb, err := loadProperties(wbPath, wbProps)
	if err != nil {
		fatal(err)
	}

	emojiProps := map[string]bool{
		"Emoji": true, "Emoji_Component": true,
		"Emoji_Modifier": true, "Emoji_Modifier_Base": true,
		"Emoji_Presentation": true, "Extended_Pictographic": true,
	}
	emojiPath, err := resolveDataPath("emoji-data.txt")
	if err != nil {
		fatal(err)
	}
	emoji, err := loadProperties(emojiPath, emojiProps)
	if err != nil {
		fatal(err)
	}

	scriptProps := map[string]bool{
		"Han": true, "Hiragana": true,
		"Katakana": true, "Hangul": true,
	}
	scriptPath, err := resolveDataPath("Scripts.txt")
	if err != nil {
		fatal(err)
	}
	scripts, err := loadProperties(scriptPath, scriptProps)
	if err != nil {
		fatal(err)
	}

	// LineBreak: only the SA (Complex_Context) class is needed.
	lbProps := map[string]bool{"SA": true}
	lbPath, err := resolveDataPath("LineBreak.txt")
	if err != nil {
		fatal(err)
	}
	lb, err := loadProperties(lbPath, lbProps)
	if err != nil {
		fatal(err)
	}

	tables := make([]tableEntry, 0, 32)

	// Word_Break tables - emitted in a stable, alphabetically
	// sorted order so the diff is reviewable run-to-run.
	wbNames := sortedKeys(wb)
	for _, name := range wbNames {
		addTable(&tables, "wbProp"+normalizeName(name), "Word_Break", name, wb[name].compact())
	}

	// Emoji tables.
	emojiNames := sortedKeys(emoji)
	for _, name := range emojiNames {
		addTable(&tables, "emojiProp"+normalizeName(name), "Emoji", name, emoji[name].compact())
	}

	// Script tables: Han, Hiragana, Katakana, Hangul.
	scriptNames := sortedKeys(scripts)
	for _, name := range scriptNames {
		addTable(&tables, "script"+normalizeName(name), "Script", name, scripts[name].compact())
	}

	// LineBreak:SA (Complex_Context). Single table.
	addTable(&tables, "lbComplexContext", "Line_Break", "SA", lb["SA"].compact())

	outPath, err := resolveOutPath()
	if err != nil {
		fatal(err)
	}
	if err := emit(outPath, tables); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (%d tables)\n", outPath, len(tables))
}

func sortedKeys(m map[string]*rangeSet) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// normalizeName converts a UCD property identifier (with underscores)
// into a Go-friendly identifier in CamelCase.
func normalizeName(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen-unicode-wb:", err)
	os.Exit(1)
}
