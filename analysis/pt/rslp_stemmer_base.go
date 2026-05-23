// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pt

import (
	"bufio"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// RSLPStemmerBase is the Go port of
// org.apache.lucene.analysis.pt.RSLPStemmerBase from Apache Lucene 10.4.0.
//
// It parses an RSLP-format rule file and exposes the named Step objects.
// Language-specific stemmers embed this type and call Parse to load their
// configuration.
//
// Deviation: Java loads the resource via Class.getResourceAsStream. Go uses
// an fs.FS passed explicitly to Parse; embedding packages attach their own
// embed.FS at init time.
//
// Deviation: Java uses char[] (UTF-16). Gocene uses []rune (Unicode code
// points). The Step.Apply / Rule.Matches / Rule.Replace methods operate on
// []rune and an int length pair, mirroring the Java interface exactly.
type RSLPStemmerBase struct{}

// ─── Rule types ──────────────────────────────────────────────────────────────

// Rule is the basic stemming rule: if the word ends with Suffix and the
// candidate stem (word minus suffix) has length ≥ Min, replace the suffix
// with Replacement.
type Rule struct {
	Suffix      []rune
	Replacement []rune
	Min         int
}

// NewRule creates a basic Rule.
func NewRule(suffix string, min int, replacement string) *Rule {
	return &Rule{
		Suffix:      []rune(suffix),
		Replacement: []rune(replacement),
		Min:         min,
	}
}

// Matches reports whether the rule fires on s[:length].
func (r *Rule) Matches(s []rune, length int) bool {
	return length-len(r.Suffix) >= r.Min && runesEndWith(s, length, r.Suffix)
}

// Replace applies the rule to s[:length] and returns the new length.
// The caller must ensure len(s) >= length+len(r.Replacement).
func (r *Rule) Replace(s []rune, length int) int {
	newLen := length - len(r.Suffix) + len(r.Replacement)
	copy(s[length-len(r.Suffix):], r.Replacement)
	return newLen
}

// RuleWithSetExceptions is a Rule that additionally checks a whole-word
// exception set (type=1 in the RSLP file).
type RuleWithSetExceptions struct {
	Rule
	exceptions *analysis.CharArraySet
}

// NewRuleWithSetExceptions creates a rule with whole-word exceptions.
func NewRuleWithSetExceptions(suffix string, min int, replacement string, exceptions []string) *RuleWithSetExceptions {
	return &RuleWithSetExceptions{
		Rule:       *NewRule(suffix, min, replacement),
		exceptions: analysis.NewCharArraySetFromCollection(exceptions, false),
	}
}

// Matches reports whether the rule fires and the word is not in the exception set.
func (r *RuleWithSetExceptions) Matches(s []rune, length int) bool {
	return r.Rule.Matches(s, length) && !r.exceptions.Contains(s, 0, length)
}

// RuleWithSuffixExceptions is a Rule that additionally checks a list of
// exception suffixes (type=0 in the RSLP file).
type RuleWithSuffixExceptions struct {
	Rule
	exceptions [][]rune
}

// NewRuleWithSuffixExceptions creates a rule with suffix exceptions.
func NewRuleWithSuffixExceptions(suffix string, min int, replacement string, exceptions []string) *RuleWithSuffixExceptions {
	excRunes := make([][]rune, len(exceptions))
	for i, e := range exceptions {
		excRunes[i] = []rune(e)
	}
	return &RuleWithSuffixExceptions{
		Rule:       *NewRule(suffix, min, replacement),
		exceptions: excRunes,
	}
}

// Matches reports whether the rule fires and no exception suffix matches.
func (r *RuleWithSuffixExceptions) Matches(s []rune, length int) bool {
	if !r.Rule.Matches(s, length) {
		return false
	}
	for _, exc := range r.exceptions {
		if runesEndWith(s, length, exc) {
			return false
		}
	}
	return true
}

// matcher is satisfied by all three rule types.
type matcher interface {
	Matches(s []rune, length int) bool
	Replace(s []rune, length int) int
}

// Step is an ordered list of rules applied to a word buffer.
type Step struct {
	Name     string
	rules    []matcher
	min      int
	suffixes [][]rune // optional entry conditions
}

// NewStep creates a Step from parsed parts.
func NewStep(name string, rules []matcher, min int, suffixes []string) *Step {
	st := &Step{Name: name, rules: rules}
	if min == 0 {
		min = int(^uint(0) >> 1) // MaxInt
		for _, r := range rules {
			switch rv := r.(type) {
			case *Rule:
				if rv.Min+len(rv.Suffix) < min {
					min = rv.Min + len(rv.Suffix)
				}
			case *RuleWithSetExceptions:
				if rv.Min+len(rv.Suffix) < min {
					min = rv.Min + len(rv.Suffix)
				}
			case *RuleWithSuffixExceptions:
				if rv.Min+len(rv.Suffix) < min {
					min = rv.Min + len(rv.Suffix)
				}
			}
		}
	}
	st.min = min

	for _, suf := range suffixes {
		st.suffixes = append(st.suffixes, []rune(suf))
	}
	return st
}

// Apply applies the step to s[:length] and returns the new length.
// The caller must ensure s is oversized by at least 1 to accommodate
// possible replacement growth.
func (st *Step) Apply(s []rune, length int) int {
	if length < st.min {
		return length
	}

	if len(st.suffixes) > 0 {
		found := false
		for _, suf := range st.suffixes {
			if runesEndWith(s, length, suf) {
				found = true
				break
			}
		}
		if !found {
			return length
		}
	}

	for _, rule := range st.rules {
		if rule.Matches(s, length) {
			return rule.Replace(s, length)
		}
	}
	return length
}

// ─── Parser ──────────────────────────────────────────────────────────────────

var (
	headerRE = regexp.MustCompile(`^\{\s*"([^"]*)"\s*,\s*(\d+)\s*,\s*(0|1)\s*,\s*\{(.*)\}\s*,\s*$`)
	stripRE  = regexp.MustCompile(`^\{\s*"([^"]*)"\s*,\s*(\d+)\s*\}\s*(,|(\}\s*;))$`)
	repRE    = regexp.MustCompile(`^\{\s*"([^"]*)"\s*,\s*(\d+)\s*,\s*"([^"]*)"\s*\}\s*(,|(\}\s*;))$`)
	excRE    = regexp.MustCompile(`^\{\s*"([^"]*)"\s*,\s*(\d+)\s*,\s*"([^"]*)"\s*,\s*\{(.*)\}\s*\}\s*(,|(\}\s*;))$`)
)

// ParseFS parses the named resource from the given fs.FS and returns a map
// from step name to Step.
func ParseFS(fsys fs.FS, name string) (map[string]*Step, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("rslp: open %q: %w", name, err)
	}
	defer f.Close() //nolint:errcheck

	steps := make(map[string]*Step)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var header string
	for {
		header = readLine(scanner)
		if header == "" {
			break
		}
		st, err := parseStep(scanner, header)
		if err != nil {
			return nil, err
		}
		steps[st.Name] = st
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("rslp: scan: %w", err)
	}
	return steps, nil
}

func readLine(scanner *bufio.Scanner) string {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 && line[0] != '#' {
			return line
		}
	}
	return ""
}

func parseStep(scanner *bufio.Scanner, header string) (*Step, error) {
	m := headerRE.FindStringSubmatch(header)
	if m == nil {
		return nil, fmt.Errorf("rslp: illegal step header: %q", header)
	}
	name := m[1]
	min, _ := strconv.Atoi(m[2])
	exType, _ := strconv.Atoi(m[3])
	suffixes := parseList(m[4])

	rules, err := parseRules(scanner, exType)
	if err != nil {
		return nil, err
	}
	return NewStep(name, rules, min, suffixes), nil
}

func parseRules(scanner *bufio.Scanner, exType int) ([]matcher, error) {
	var rules []matcher
	for {
		line := readLine(scanner)
		if line == "" {
			break
		}

		if m := stripRE.FindStringSubmatch(line); m != nil {
			rules = append(rules, NewRule(m[1], mustAtoi(m[2]), ""))
		} else if m := repRE.FindStringSubmatch(line); m != nil {
			rules = append(rules, NewRule(m[1], mustAtoi(m[2]), m[3]))
		} else if m := excRE.FindStringSubmatch(line); m != nil {
			excs := parseList(m[4])
			if exType == 0 {
				rules = append(rules, NewRuleWithSuffixExceptions(m[1], mustAtoi(m[2]), m[3], excs))
			} else {
				rules = append(rules, NewRuleWithSetExceptions(m[1], mustAtoi(m[2]), m[3], excs))
			}
		} else {
			return nil, fmt.Errorf("rslp: illegal rule: %q", line)
		}

		if strings.HasSuffix(line, ";") {
			return rules, nil
		}
	}
	return rules, nil
}

func parseList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && p[0] == '"' {
			p = p[1 : len(p)-1]
		}
		out = append(out, p)
	}
	return out
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// runesEndWith reports whether s[:length] ends with suffix.
func runesEndWith(s []rune, length int, suffix []rune) bool {
	if len(suffix) > length {
		return false
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		if s[length-(len(suffix)-i)] != suffix[i] {
			return false
		}
	}
	return true
}

// AppendUTF8 converts s[:length] []rune to a UTF-8 string, truncated to length.
// This is a helper for stemmers that need to return the result as a string.
func AppendUTF8(s []rune, length int) string {
	buf := make([]byte, 0, length*utf8.UTFMax)
	for i := 0; i < length; i++ {
		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], s[i])
		buf = append(buf, tmp[:n]...)
	}
	return string(buf)
}
