// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/internal/hppc"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// ─── Constants ──────────────────────────────────────────────────────────────

const (
	// maxPrologueScanWindow is the maximum number of bytes scanned in the first
	// pass of the affix file to detect encoding/flag settings.
	maxPrologueScanWindow = 30 * 1024

	// DictionaryFlagUnset is the zero/unset flag value.
	DictionaryFlagUnset = flagUnset

	// DictionaryDefaultFlags is the maximum value of a user-visible flag.
	DictionaryDefaultFlags = 65510

	// DictionaryHiddenFlag is the internal flag used for title-cased hidden
	// entries ("ONLYUPCASEFLAG" in Hunspell).
	DictionaryHiddenFlag = rune(65511)

	// affixFlag, affixStripOrd, affixCondition, affixAppend are offsets inside
	// the 4-element per-affix slot in affixData.
	AffixFlag      = 0
	AffixStripOrd  = 1
	affixCondition = 2
	AffixAppend    = 3
)

var noflags = []rune{}

// ─── Flag-separator constants ────────────────────────────────────────────────

const (
	flagSeparator  = rune(0x1f)
	morphSeparator = rune(0x1e)
)

// ─── Breaks ──────────────────────────────────────────────────────────────────

// Breaks holds the BREAK directives from the affix file.
//
// This is the Go port of Dictionary.Breaks in Apache Lucene 10.4.0.
type Breaks struct {
	Starting []string
	Ending   []string
	Middle   []string
}

var defaultBreaks = &Breaks{
	Starting: []string{"-"},
	Ending:   []string{"-"},
	Middle:   []string{"-"},
}

// IsNotEmpty reports whether any break patterns are defined.
func (b *Breaks) IsNotEmpty() bool {
	return len(b.Middle) > 0 || len(b.Starting) > 0 || len(b.Ending) > 0
}

// ─── FlagParsingStrategy ────────────────────────────────────────────────────

// FlagParsingStrategy abstracts flag parsing for the different flag formats
// supported by Hunspell (ASCII, UTF-8, numeric, double-ASCII).
//
// This is the Go port of Dictionary.FlagParsingStrategy from Apache Lucene 10.4.0.
type FlagParsingStrategy interface {
	// ParseFlag parses a single flag from rawFlag.
	ParseFlag(rawFlag string) rune
	// ParseFlags parses multiple flags from rawFlags.
	ParseFlags(rawFlags string) []rune
	// PrintFlag formats a single encoded flag for display.
	PrintFlag(flag rune) string
	// PrintFlags formats a sorted set of flags for display.
	PrintFlags(flags []rune) string
	// ParseUtfFlags parses flags from a string produced by PrintFlags.
	ParseUtfFlags(flagsInUtf string) []rune
}

// simpleFlagParsingStrategy treats each rune in the input as a separate flag.
type simpleFlagParsingStrategy struct{}

func (s *simpleFlagParsingStrategy) ParseFlag(rawFlag string) rune {
	flags := s.ParseFlags(rawFlag)
	if len(flags) == 0 {
		return 0
	}
	return flags[0]
}

func (s *simpleFlagParsingStrategy) ParseFlags(rawFlags string) []rune {
	return []rune(rawFlags)
}

func (s *simpleFlagParsingStrategy) PrintFlag(flag rune) string {
	return string(flag)
}

func (s *simpleFlagParsingStrategy) PrintFlags(flags []rune) string {
	if len(flags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(flags))
	for _, f := range flags {
		if f < DictionaryDefaultFlags {
			parts = append(parts, s.PrintFlag(f))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "")
}

func (s *simpleFlagParsingStrategy) ParseUtfFlags(flagsInUtf string) []rune {
	return s.ParseFlags(flagsInUtf)
}

// defaultAsUTF8FlagParsingStrategy decodes flags as UTF-8 even when the
// file uses a different 8-bit encoding.
type defaultAsUTF8FlagParsingStrategy struct{}

func (d *defaultAsUTF8FlagParsingStrategy) ParseFlag(rawFlag string) rune {
	flags := d.ParseFlags(rawFlag)
	if len(flags) == 0 {
		return 0
	}
	return flags[0]
}

// ParseFlags re-encodes rawFlags from Latin-1 to UTF-8, then splits into runes.
func (d *defaultAsUTF8FlagParsingStrategy) ParseFlags(rawFlags string) []rune {
	// rawFlags is a Go string (UTF-8), but the bytes in it represent Latin-1.
	// Re-interpret: treat each byte as a Latin-1 code point.
	latin1 := make([]byte, len(rawFlags))
	for i := 0; i < len(rawFlags); i++ {
		latin1[i] = rawFlags[i]
	}
	// Now decode from Latin-1 to runes.
	runes := make([]rune, 0, len(latin1))
	for _, b := range latin1 {
		runes = append(runes, rune(b))
	}
	return runes
}

func (d *defaultAsUTF8FlagParsingStrategy) PrintFlag(flag rune) string {
	return string(flag)
}

func (d *defaultAsUTF8FlagParsingStrategy) PrintFlags(flags []rune) string {
	parts := make([]string, 0, len(flags))
	for _, f := range flags {
		if f < DictionaryDefaultFlags {
			parts = append(parts, d.PrintFlag(f))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "")
}

func (d *defaultAsUTF8FlagParsingStrategy) ParseUtfFlags(flagsInUtf string) []rune {
	return []rune(flagsInUtf)
}

// numFlagParsingStrategy parses numeric flags (comma-separated integers).
type numFlagParsingStrategy struct{}

func (n *numFlagParsingStrategy) ParseFlag(rawFlag string) rune {
	flags := n.ParseFlags(rawFlag)
	if len(flags) == 0 {
		return 0
	}
	return flags[0]
}

func (n *numFlagParsingStrategy) ParseFlags(rawFlags string) []rune {
	var result []rune
	var group strings.Builder
	for i := 0; i <= len(rawFlags); i++ {
		if i == len(rawFlags) || rawFlags[i] == ',' {
			if group.Len() > 0 {
				flag, err := strconv.Atoi(group.String())
				if err == nil {
					if flag >= DictionaryDefaultFlags {
						panic(fmt.Sprintf("hunspell: num flag out of range: %d", flag))
					}
					result = append(result, rune(flag))
				}
				group.Reset()
			}
		} else if rawFlags[i] >= '0' && rawFlags[i] <= '9' {
			group.WriteByte(rawFlags[i])
		}
	}
	return result
}

func (n *numFlagParsingStrategy) PrintFlag(flag rune) string {
	return strconv.Itoa(int(flag))
}

func (n *numFlagParsingStrategy) PrintFlags(flags []rune) string {
	parts := make([]string, 0, len(flags))
	for _, f := range flags {
		if f < DictionaryDefaultFlags {
			parts = append(parts, n.PrintFlag(f))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (n *numFlagParsingStrategy) ParseUtfFlags(flagsInUtf string) []rune {
	return n.ParseFlags(flagsInUtf)
}

// doubleASCIIFlagParsingStrategy encodes two ASCII chars per flag.
type doubleASCIIFlagParsingStrategy struct{}

func (da *doubleASCIIFlagParsingStrategy) ParseFlag(rawFlag string) rune {
	flags := da.ParseFlags(rawFlag)
	if len(flags) == 0 {
		return 0
	}
	return flags[0]
}

func (da *doubleASCIIFlagParsingStrategy) ParseFlags(rawFlags string) []rune {
	r := []rune(rawFlags)
	flags := make([]rune, len(r)/2)
	for i := range flags {
		f1, f2 := r[i*2], r[i*2+1]
		flags[i] = rune(int(f1)<<8 | int(f2))
	}
	return flags
}

func (da *doubleASCIIFlagParsingStrategy) PrintFlag(flag rune) string {
	return string([]rune{(flag & 0xff00) >> 8, flag & 0xff})
}

func (da *doubleASCIIFlagParsingStrategy) PrintFlags(flags []rune) string {
	parts := make([]string, 0, len(flags))
	for _, f := range flags {
		if f < DictionaryDefaultFlags {
			parts = append(parts, da.PrintFlag(f))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "")
}

func (da *doubleASCIIFlagParsingStrategy) ParseUtfFlags(flagsInUtf string) []rune {
	return da.ParseFlags(flagsInUtf)
}

// GetFlagParsingStrategy returns the FlagParsingStrategy indicated by a FLAG
// directive line.
func GetFlagParsingStrategy(flagLine string, encoding string) (FlagParsingStrategy, error) {
	parts := strings.Fields(flagLine)
	if len(parts) != 2 {
		return nil, fmt.Errorf("hunspell: illegal FLAG specification: %q", flagLine)
	}
	flagType := parts[1]
	switch flagType {
	case "num":
		return &numFlagParsingStrategy{}, nil
	case "UTF-8":
		// If the file is in the default (Latin-1) charset, flags need UTF-8 re-encoding.
		if encoding == "" || strings.EqualFold(encoding, "ISO-8859-1") ||
			strings.EqualFold(encoding, "ISO8859-1") {
			return &defaultAsUTF8FlagParsingStrategy{}, nil
		}
		return &simpleFlagParsingStrategy{}, nil
	case "long":
		return &doubleASCIIFlagParsingStrategy{}, nil
	default:
		return nil, fmt.Errorf("hunspell: unknown flag type: %s", flagType)
	}
}

// ─── Dictionary ──────────────────────────────────────────────────────────────

// Dictionary holds the in-memory representation of a Hunspell .dic + .aff
// file pair.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.Dictionary from Apache Lucene 10.4.0.
//
// Deviation: The Java constructor takes Directory + prefix for offline
// sorting; this Go port takes a SortingStrategy directly, defaulting to
// inMemory when NewDictionary is called without one.
// Deviation: The Java class is public with package-private fields.  The Go
// port exports the struct but keeps implementation details unexported.
type Dictionary struct {
	// prefixes / suffixes FSTs (IntsRef output = affix-id list).
	prefixes *gfst.FST[*util.IntsRef]
	suffixes *gfst.FST[*util.IntsRef]

	// Breaks directive.
	breaks *Breaks

	// AffixCondition objects (deduplicated).
	patterns []*AffixCondition

	// Word storage.
	words *WordStorage

	// Frozen flag lookup table.
	flagLookup *FlagLookup

	// Strip data: a single flat array of chars plus offsets per strip id.
	stripData    []rune
	stripOffsets []int

	// wordChars: extra characters considered part of a word.
	wordChars string

	// Per-affix data: 4 runes per affix in the order
	//   [AffixFlag, AffixStripOrd, affixCondition, AffixAppend]
	affixData    []rune
	currentAffix int

	// Flag parsing.
	flagParsingStrategy FlagParsingStrategy

	// AF (affix alias) entries.
	aliases    []string
	aliasCount int

	// AM (morph alias) entries.
	morphAliases    []string
	morphAliasCount int

	// Morphological data strings (index 0 = empty).
	morphData []string

	// Whether custom morphological data is present.
	hasCustomMorphData bool

	IgnoreCase      bool
	CheckSharpS     bool
	complexPrefixes bool

	// Second-stage affix flag sets (for 2-level affix stripping).
	secondStagePrefixFlags []rune
	secondStageSuffixFlags []rune

	circumfix      rune
	keepcase       rune
	forceUCase     rune
	needaffix      rune
	forbiddenword  rune
	onlyincompound rune
	compoundBegin  rune
	compoundMiddle rune
	compoundEnd    rune
	compoundFlag   rune
	compoundPermit rune
	compoundForbid rune

	checkCompoundCase     bool
	checkCompoundDup      bool
	checkCompoundRep      bool
	checkCompoundTriple   bool
	simplifiedTriple      bool
	compoundMin           int
	compoundMax           int
	compoundRules         []*CompoundRule
	checkCompoundPatterns []*CheckCompoundPattern

	// Ignored characters (sorted for binary search).
	ignore []rune

	tryChars               string
	neighborKeyGroups      []string
	enableSplitSuggestions bool
	repTable               []*RepEntry
	mapTable               [][]string
	maxDiff                int
	maxNGramSuggestions    int
	onlyMaxDiff            bool
	noSuggest              rune
	subStandard            rune
	iconv                  *ConvTable
	oconv                  *ConvTable

	fullStrip       bool
	language        string
	alternateCasing bool
}

// NewDictionary parses the given affix stream and dictionary streams using the
// in-memory SortingStrategy.
//
// affixStream and dictStreams are read but not closed by this function.
func NewDictionary(affixStream io.Reader, dictStreams []io.Reader, ignoreCase bool) (*Dictionary, error) {
	return newDictionary(affixStream, dictStreams, ignoreCase, nil)
}

func newDictionary(affixStream io.Reader, dictStreams []io.Reader, ignoreCase bool, strategy SortingStrategy) (*Dictionary, error) {
	d := &Dictionary{
		IgnoreCase:             ignoreCase,
		breaks:                 defaultBreaks,
		flagParsingStrategy:    &simpleFlagParsingStrategy{},
		compoundMin:            3,
		compoundMax:            int(^uint(0) >> 1),
		morphData:              []string{""},
		enableSplitSuggestions: true,
		maxDiff:                5,
		maxNGramSuggestions:    4,
		neighborKeyGroups:      []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"},
		affixData:              make([]rune, 32),
	}
	// zero condition → ord 0
	d.patterns = []*AffixCondition{nil}

	// 1. Probe encoding from the prologue.
	buf, err := probeEncoding(affixStream)
	if err != nil {
		return nil, err
	}
	encoding := buf.encoding
	if encoding == "" {
		encoding = "ISO-8859-1"
	}
	if st, err2 := GetFlagParsingStrategy("FLAG "+buf.flagType, encoding); err2 == nil {
		d.flagParsingStrategy = st
	}

	// 2. Parse the affix file.
	flagEnum := NewFlagEnumerator()
	if err := d.readAffixFile(buf.rawBytes, encoding, flagEnum); err != nil {
		return nil, err
	}

	// 3. Parse dictionary entries.
	if strategy == nil {
		strategy = InMemorySortingStrategy()
	}
	acc, err := strategy.Start()
	if err != nil {
		return nil, err
	}
	decoder := makeDecoder(encoding)
	if err := d.mergeDictionaries(dictStreams, decoder, acc); err != nil {
		return nil, err
	}
	sorted, err := acc.FinishAndSort()
	if err != nil {
		return nil, err
	}
	defer sorted.Close() //nolint:errcheck
	d.words, err = d.readSortedDictionaries(flagEnum, sorted)
	if err != nil {
		return nil, err
	}
	d.flagLookup = flagEnum.Finish()
	d.aliases = nil
	d.morphAliases = nil
	return d, nil
}

// ─── Prologue probe ──────────────────────────────────────────────────────────

type prologueBuf struct {
	rawBytes []byte
	encoding string
	flagType string
}

func probeEncoding(r io.Reader) (*prologueBuf, error) {
	// Read up to maxPrologueScanWindow bytes.
	// io.ReadFull fills the entire buffer or returns io.ErrUnexpectedEOF when the
	// reader is shorter than the buffer — both outcomes are acceptable here.
	prologue := make([]byte, maxPrologueScanWindow)
	n, err := io.ReadFull(r, prologue)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	prologue = prologue[:n]

	// Strip UTF-8 BOM if present.
	if bytes.HasPrefix(prologue, []byte{0xef, 0xbb, 0xbf}) {
		prologue = prologue[3:]
	}

	buf := &prologueBuf{rawBytes: prologue}
	sc := bufio.NewScanner(bytes.NewReader(prologue))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "SET":
			if len(fields) >= 2 {
				buf.encoding = fields[1]
			}
		case "FLAG":
			if len(fields) >= 2 {
				buf.flagType = fields[1]
			}
		}
		if buf.encoding != "" && buf.flagType != "" {
			break
		}
	}
	return buf, nil
}

// ─── Decoder ────────────────────────────────────────────────────────────────

// dictDecoder converts raw bytes from a .dic/.aff stream into Go strings.
type dictDecoder func([]byte) string

func makeDecoder(encoding string) dictDecoder {
	norm := strings.ToUpper(strings.ReplaceAll(encoding, "-", ""))
	switch norm {
	case "ISO885914":
		return DecodeISO8859_14
	case "ISO88591", "ISO8859", "LATIN1", "LATIN-1":
		// ISO-8859-1: bytes 0x00-0xFF map 1:1 to Unicode codepoints U+0000-U+00FF.
		return DecodeISO8859_1
	default:
		return func(b []byte) string { return string(b) }
	}
}

// DecodeISO8859_1 converts ISO-8859-1 (Latin-1) bytes to a UTF-8 string.
// Each byte value maps directly to the Unicode codepoint with the same value.
func DecodeISO8859_1(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

// ─── Affix file parsing ──────────────────────────────────────────────────────

type affixParseContext struct {
	prefixes        map[string][]int32 // string → list of affix ids
	suffixes        map[string][]int32
	prefixConts     hppc.CharHashSet
	suffixConts     hppc.CharHashSet
	seenPatterns    map[string]int
	seenStrips      map[string]int // insertion-ordered
	seenStripsOrder []string
}

func (d *Dictionary) readAffixFile(raw []byte, encoding string, flags *FlagEnumerator) error {
	affixDecoder := makeDecoder(encoding)
	ctx := &affixParseContext{
		prefixes:        make(map[string][]int32),
		suffixes:        make(map[string][]int32),
		prefixConts:     make(hppc.CharHashSet),
		suffixConts:     make(hppc.CharHashSet),
		seenPatterns:    map[string]int{alwaysTrueKey: 0},
		seenStrips:      map[string]int{"": 0},
		seenStripsOrder: []string{""},
	}

	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 256*1024), 256*1024)

	lineNo := 0
	var sb strings.Builder

	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if lineNo == 1 && len(line) >= 3 &&
			line[0] == '\xef' && line[1] == '\xbb' && line[2] == '\xbf' {
			line = line[3:]
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "AF":
			if err := d.parseAlias(fields); err != nil {
				return err
			}
		case "AM":
			if err := d.parseMorphAlias(line); err != nil {
				return err
			}
		case "PFX":
			if err := d.parseAffix(ctx, line, AffixKindPrefix, flags, sc, &sb, lineNo); err != nil {
				return err
			}
		case "SFX":
			if err := d.parseAffix(ctx, line, AffixKindSuffix, flags, sc, &sb, lineNo); err != nil {
				return err
			}
		case "COMPLEXPREFIXES":
			d.complexPrefixes = true
		case "CIRCUMFIX":
			if len(fields) >= 2 {
				d.circumfix = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "KEEPCASE":
			if len(fields) >= 2 {
				d.keepcase = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "FORCEUCASE":
			if len(fields) >= 2 {
				d.forceUCase = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "NEEDAFFIX", "PSEUDOROOT":
			if len(fields) >= 2 {
				d.needaffix = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "ONLYINCOMPOUND":
			if len(fields) >= 2 {
				d.onlyincompound = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "CHECKSHARPS":
			d.CheckSharpS = true
		case "IGNORE":
			if len(fields) >= 2 {
				d.ignore = []rune(fields[1])
				sort.Slice(d.ignore, func(i, j int) bool { return d.ignore[i] < d.ignore[j] })
			}
		case "ICONV":
			if len(fields) >= 2 {
				num, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid ICONV count %q: %w", fields[1], err)
				}
				ct, err := d.parseConversions(sc, num, &lineNo)
				if err != nil {
					return err
				}
				d.iconv = ct
			}
		case "OCONV":
			if len(fields) >= 2 {
				num, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid OCONV count %q: %w", fields[1], err)
				}
				ct, err := d.parseConversions(sc, num, &lineNo)
				if err != nil {
					return err
				}
				d.oconv = ct
			}
		case "FULLSTRIP":
			d.fullStrip = true
		case "LANG":
			if len(fields) >= 2 {
				d.language = fields[1]
				d.alternateCasing = d.hasLanguage("tr", "az")
			}
		case "BREAK":
			if len(fields) >= 2 {
				num, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid BREAK count %q: %w", fields[1], err)
				}
				b, err := d.parseBreaks(sc, num, &lineNo)
				if err != nil {
					return err
				}
				d.breaks = b
			}
		case "WORDCHARS":
			if len(fields) >= 2 {
				d.wordChars = fields[1]
			}
		case "TRY":
			if len(fields) >= 2 {
				d.tryChars = fields[1]
			}
		case "REP":
			if len(fields) >= 3 {
				// REP N or REP pattern replacement
				if num, err := strconv.Atoi(fields[1]); err == nil && len(fields) == 2 {
					// count line; parse subsequent lines
					for i := 0; i < num; i++ {
						if sc.Scan() {
							lineNo++
							lineStr := affixDecoder(sc.Bytes())
							rfields := strings.Fields(lineStr)
							if len(rfields) >= 3 {
								d.repTable = append(d.repTable, NewRepEntry(rfields[1], rfields[2]))
							}
						}
					}
				} else {
					pat := affixDecoder([]byte(fields[1]))
					rep := affixDecoder([]byte(fields[2]))
					d.repTable = append(d.repTable, NewRepEntry(pat, rep))
				}
			} else if len(fields) == 2 {
				// count-only form: no-op (subsequent lines parsed inline)
			}
		case "MAP":
			if len(fields) >= 2 {
				num, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid MAP count %q: %w", fields[1], err)
				}
				for i := 0; i < num; i++ {
					if sc.Scan() {
						lineNo++
						entry := d.parseMapEntry(sc.Text())
						d.mapTable = append(d.mapTable, entry)
					}
				}
			}
		case "KEY":
			if len(fields) >= 2 {
				d.neighborKeyGroups = strings.Split(fields[1], "|")
			}
		case "NOSPLITSUGS":
			d.enableSplitSuggestions = false
		case "MAXNGRAMSUGS":
			if len(fields) >= 2 {
				v, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid MAXNGRAMSUGS value %q: %w", fields[1], err)
				}
				d.maxNGramSuggestions = v
			}
		case "MAXDIFF":
			if len(fields) >= 2 {
				v, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid MAXDIFF value %q: %w", fields[1], err)
				}
				d.maxDiff = v
			}
		case "ONLYMAXDIFF":
			d.onlyMaxDiff = true
		case "FORBIDDENWORD":
			if len(fields) >= 2 {
				d.forbiddenword = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "NOSUGGEST":
			if len(fields) >= 2 {
				d.noSuggest = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "SUBSTANDARD":
			if len(fields) >= 2 {
				d.subStandard = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "COMPOUNDMIN":
			if len(fields) >= 2 {
				v, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid COMPOUNDMIN value %q: %w", fields[1], err)
				}
				if v < 1 {
					v = 1
				}
				d.compoundMin = v
			}
		case "COMPOUNDWORDMAX":
			if len(fields) >= 2 {
				v, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid COMPOUNDWORDMAX value %q: %w", fields[1], err)
				}
				if v < 1 {
					v = 1
				}
				d.compoundMax = v
			}
		case "COMPOUNDRULE":
			if len(fields) >= 2 {
				num, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid COMPOUNDRULE count %q: %w", fields[1], err)
				}
				rules, err := d.parseCompoundRules(sc, num, &lineNo)
				if err != nil {
					return err
				}
				d.compoundRules = rules
			}
		case "COMPOUNDFLAG":
			if len(fields) >= 2 {
				d.compoundFlag = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "COMPOUNDBEGIN":
			if len(fields) >= 2 {
				d.compoundBegin = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "COMPOUNDMIDDLE":
			if len(fields) >= 2 {
				d.compoundMiddle = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "COMPOUNDEND":
			if len(fields) >= 2 {
				d.compoundEnd = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "COMPOUNDPERMITFLAG":
			if len(fields) >= 2 {
				d.compoundPermit = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "COMPOUNDFORBIDFLAG":
			if len(fields) >= 2 {
				d.compoundForbid = d.flagParsingStrategy.ParseFlag(fields[1])
			}
		case "CHECKCOMPOUNDCASE":
			d.checkCompoundCase = true
		case "CHECKCOMPOUNDDUP":
			d.checkCompoundDup = true
		case "CHECKCOMPOUNDREP":
			d.checkCompoundRep = true
		case "CHECKCOMPOUNDTRIPLE":
			d.checkCompoundTriple = true
		case "SIMPLIFIEDTRIPLE":
			d.simplifiedTriple = true
		case "CHECKCOMPOUNDPATTERN":
			if len(fields) >= 2 {
				num, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("hunspell: invalid CHECKCOMPOUNDPATTERN count %q: %w", fields[1], err)
				}
				for i := 0; i < num; i++ {
					if sc.Scan() {
						lineNo++
						pat, err := NewCheckCompoundPattern(strings.TrimSpace(sc.Text()), d.flagParsingStrategy, d)
						if err != nil {
							return err
						}
						d.checkCompoundPatterns = append(d.checkCompoundPatterns, pat)
					}
				}
			}
		// SET and FLAG are handled in the prologue probe; re-encountering them is OK.
		case "SET", "FLAG":
			// already handled
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("hunspell: affix scan error: %w", err)
	}

	// Build affix FSTs.
	var err error
	d.prefixes, err = d.buildAffixFST(ctx.prefixes)
	if err != nil {
		return err
	}
	d.suffixes, err = d.buildAffixFST(ctx.suffixes)
	if err != nil {
		return err
	}
	d.secondStagePrefixFlags = setToSortedRunes(ctx.prefixConts)
	d.secondStageSuffixFlags = setToSortedRunes(ctx.suffixConts)

	// Build strip data.
	totalChars := 0
	for _, strip := range ctx.seenStripsOrder {
		totalChars += len([]rune(strip))
	}
	d.stripData = make([]rune, totalChars)
	d.stripOffsets = make([]int, len(ctx.seenStripsOrder)+1)
	cur := 0
	for idx, strip := range ctx.seenStripsOrder {
		d.stripOffsets[idx] = cur
		runes := []rune(strip)
		copy(d.stripData[cur:], runes)
		cur += len(runes)
	}
	d.stripOffsets[len(ctx.seenStripsOrder)] = cur

	return nil
}

func setToSortedRunes(set hppc.CharHashSet) []rune {
	runes := make([]rune, 0, len(set))
	for r := range set {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	return runes
}

// buildAffixFST builds an FST[*util.IntsRef] from the given affix map.
// Keys are the affix strings (reversed for suffixes), values are the affix-id lists.
func (d *Dictionary) buildAffixFST(affixes map[string][]int32) (*gfst.FST[*util.IntsRef], error) {
	// Collect and sort keys.
	keys := make([]string, 0, len(affixes))
	for k := range affixes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	outputs := gfst.IntSequenceOutputs()
	builder := gfst.NewFSTCompilerBuilder[*util.IntsRef](gfst.InputTypeByte4, outputs).Build()
	scratch := util.NewIntsRefBuilder()

	for _, key := range keys {
		// toUTF32: encode each rune as a single int.
		runes := []rune(key)
		scratch.Clear()
		for _, r := range runes {
			scratch.Append(int(r))
		}
		ids := affixes[key]
		ints := make([]int, len(ids))
		for i, id := range ids {
			ints[i] = int(id)
		}
		out := util.NewIntsRef(ints)
		if err := builder.Add(scratch.Get(), out); err != nil {
			return nil, err
		}
	}

	meta, err := builder.Compile()
	if err != nil {
		return nil, err
	}
	return gfst.FromFSTReader[*util.IntsRef](meta, builder.GetFSTReader())
}

// ─── Affix rule parsing ──────────────────────────────────────────────────────

func (d *Dictionary) parseAffix(
	ctx *affixParseContext,
	header string,
	kind AffixKind,
	flags *FlagEnumerator,
	sc *bufio.Scanner,
	sb *strings.Builder,
	lineNo int,
) error {
	args := strings.Fields(header)
	if len(args) < 4 {
		return fmt.Errorf("hunspell: bad affix header at line %d: %q", lineNo, header)
	}

	crossProduct := args[2] == "Y"
	numLines, err := strconv.Atoi(args[3])
	if err != nil {
		return nil // tolerate count mismatches
	}

	// Grow affixData if needed.
	needed := (d.currentAffix + numLines) * 4
	for needed > len(d.affixData) {
		d.affixData = append(d.affixData, make([]rune, len(d.affixData))...)
	}

	for i := 0; i < numLines; i++ {
		if !sc.Scan() {
			break
		}
		lineNo++
		line := strings.TrimSpace(sc.Text())
		ruleArgs := strings.Fields(line)
		if len(ruleArgs) < 4 {
			continue
		}
		if ruleArgs[1] != args[1] {
			continue // count mismatch: skip
		}

		flag := d.flagParsingStrategy.ParseFlag(ruleArgs[1])
		var strip string
		if ruleArgs[2] != "0" {
			strip = ruleArgs[2]
		}
		affixArg := ruleArgs[3]

		var appendFlags []rune
		if sep := strings.LastIndex(affixArg, "/"); sep >= 0 {
			flagPart := affixArg[sep+1:]
			affixArg = affixArg[:sep]
			if d.aliasCount > 0 {
				idx, err2 := strconv.Atoi(flagPart)
				if err2 == nil {
					flagPart = d.getAliasValue(idx)
				}
			}
			appendFlags = d.flagParsingStrategy.ParseFlags(flagPart)
			for _, af := range appendFlags {
				if kind == AffixKindPrefix {
					ctx.prefixConts[af] = struct{}{}
				} else {
					ctx.suffixConts[af] = struct{}{}
				}
			}
		}
		if affixArg == "0" {
			affixArg = ""
		}

		var condition string
		if len(ruleArgs) > 4 {
			condition = ruleArgs[4]
		} else {
			condition = "."
		}

		condKey := AffixConditionUniqueKey(kind, strip, condition)
		patternIndex, ok := ctx.seenPatterns[condKey]
		if !ok {
			patternIndex = len(d.patterns)
			ctx.seenPatterns[condKey] = patternIndex
			compiled := CompileAffixCondition(kind, strip, condition, line)
			d.patterns = append(d.patterns, compiled)
		}

		stripOrd, ok := ctx.seenStrips[strip]
		if !ok {
			stripOrd = len(ctx.seenStrips)
			ctx.seenStrips[strip] = stripOrd
			ctx.seenStripsOrder = append(ctx.seenStripsOrder, strip)
		}

		if appendFlags == nil {
			appendFlags = noflags
		}
		appendFlagsOrd := flags.Add(appendFlags)

		dataStart := d.currentAffix * 4
		d.affixData[dataStart+AffixFlag] = flag
		d.affixData[dataStart+AffixStripOrd] = rune(stripOrd)
		patternOrd := patternIndex << 1
		if crossProduct {
			patternOrd |= 1
		}
		d.affixData[dataStart+affixCondition] = rune(patternOrd)
		d.affixData[dataStart+AffixAppend] = rune(appendFlagsOrd)

		if d.needsInputCleaning(affixArg) {
			sb.Reset()
			affixArg = d.cleanInput(affixArg, sb)
		}
		if kind == AffixKindSuffix {
			runes := []rune(affixArg)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			affixArg = string(runes)
		}

		if kind == AffixKindPrefix {
			ctx.prefixes[affixArg] = append(ctx.prefixes[affixArg], int32(d.currentAffix))
		} else {
			ctx.suffixes[affixArg] = append(ctx.suffixes[affixArg], int32(d.currentAffix))
		}
		d.currentAffix++
	}
	return nil
}

// ─── AffixData accessors ─────────────────────────────────────────────────────

// AffixData returns the slot at offset for the given affix id.
func (d *Dictionary) AffixData(affixIndex, offset int) rune {
	return d.affixData[affixIndex*4+offset]
}

// IsCrossProduct reports whether the given affix has cross-product enabled.
func (d *Dictionary) IsCrossProduct(affix int) bool {
	return (d.AffixData(affix, affixCondition) & 1) == 1
}

// GetAffixCondition returns the pattern index for the given affix.
func (d *Dictionary) GetAffixCondition(affix int) int {
	return int(d.AffixData(affix, affixCondition)) >> 1
}

// ─── Compound rules ──────────────────────────────────────────────────────────

func (d *Dictionary) parseCompoundRules(sc *bufio.Scanner, num int, lineNo *int) ([]*CompoundRule, error) {
	rules := make([]*CompoundRule, 0, num)
	for i := 0; i < num; i++ {
		if !sc.Scan() {
			break
		}
		*lineNo++
		line := strings.TrimSpace(sc.Text())
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		r, err := NewCompoundRule(fields[1], d.flagParsingStrategy, d)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// ─── BREAK directive ─────────────────────────────────────────────────────────

func (d *Dictionary) parseBreaks(sc *bufio.Scanner, num int, lineNo *int) (*Breaks, error) {
	starting := map[string]struct{}{}
	ending := map[string]struct{}{}
	middle := map[string]struct{}{}
	for i := 0; i < num; i++ {
		if !sc.Scan() {
			break
		}
		*lineNo++
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		breakStr := fields[1]
		if strings.HasPrefix(breakStr, "^") {
			starting[breakStr[1:]] = struct{}{}
		} else if strings.HasSuffix(breakStr, "$") {
			ending[breakStr[:len(breakStr)-1]] = struct{}{}
		} else {
			middle[breakStr] = struct{}{}
		}
	}
	b := &Breaks{
		Starting: mapKeys(starting),
		Ending:   mapKeys(ending),
		Middle:   mapKeys(middle),
	}
	return b, nil
}

func mapKeys(m map[string]struct{}) []string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return s
}

// ─── Conversion tables ───────────────────────────────────────────────────────

func (d *Dictionary) parseConversions(sc *bufio.Scanner, num int, lineNo *int) (*ConvTable, error) {
	mappings := make(map[string]string, num)
	for i := 0; i < num; i++ {
		if !sc.Scan() {
			break
		}
		*lineNo++
		fields := strings.Fields(sc.Text())
		if len(fields) >= 3 {
			mappings[fields[1]] = fields[2]
		}
	}
	return NewConvTable(mappings)
}

// ─── Map entries ─────────────────────────────────────────────────────────────

func (d *Dictionary) parseMapEntry(line string) []string {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil
	}
	unparsed := fields[1]
	var entry []string
	i := 0
	for i < len(unparsed) {
		if unparsed[i] == '(' {
			j := strings.IndexByte(unparsed[i:], ')')
			if j < 0 {
				break
			}
			entry = append(entry, unparsed[i+1:i+j])
			i += j + 1
		} else {
			entry = append(entry, string(unparsed[i]))
			i++
		}
	}
	return entry
}

// ─── Alias parsing ───────────────────────────────────────────────────────────

func (d *Dictionary) parseAlias(fields []string) error {
	if d.aliases == nil {
		if len(fields) >= 2 {
			count, err := strconv.Atoi(fields[1])
			if err != nil {
				return fmt.Errorf("hunspell: invalid AF alias count %q: %w", fields[1], err)
			}
			d.aliases = make([]string, count)
		}
		return nil
	}
	v := ""
	if len(fields) >= 2 {
		v = fields[1]
	}
	if d.aliasCount < len(d.aliases) {
		d.aliases[d.aliasCount] = v
		d.aliasCount++
	}
	return nil
}

func (d *Dictionary) parseMorphAlias(line string) error {
	if d.morphAliases == nil {
		trimmed := strings.TrimPrefix(line, "AM")
		trimmed = strings.TrimSpace(trimmed)
		count, err := strconv.Atoi(trimmed)
		if err != nil {
			return fmt.Errorf("hunspell: invalid AM morph alias count %q: %w", trimmed, err)
		}
		d.morphAliases = make([]string, count)
		return nil
	}
	arg := line[2:] // leave the space
	if d.morphAliasCount < len(d.morphAliases) {
		d.morphAliases[d.morphAliasCount] = arg
		d.morphAliasCount++
	}
	return nil
}

func (d *Dictionary) getAliasValue(id int) string {
	if id < 1 || id > len(d.aliases) {
		return ""
	}
	return d.aliases[id-1]
}

// ─── Dictionary entry parsing ────────────────────────────────────────────────

func (d *Dictionary) mergeDictionaries(dicts []io.Reader, decode dictDecoder, acc EntryAccumulator) error {
	var sb strings.Builder
	for _, dict := range dicts {
		sc := bufio.NewScanner(dict)
		sc.Buffer(make([]byte, 256*1024), 256*1024)
		first := true
		for sc.Scan() {
			raw := sc.Bytes()
			line := decode(raw)
			if first {
				first = false
				continue // first line is approximate entry count
			}
			if len(line) == 0 || line[0] == '#' || line[0] == '\t' {
				continue
			}
			line = d.unescapeEntry(line)
			if !d.hasCustomMorphData {
				morphStart := strings.IndexRune(line, morphSeparator)
				if morphStart >= 0 {
					data := line[morphStart+1:]
					for _, s := range d.splitMorphData(data) {
						if !strings.HasPrefix(s, "ph:") {
							d.hasCustomMorphData = true
							break
						}
					}
				}
			}
			if err := d.writeNormalizedWordEntry(&sb, line, acc); err != nil {
				return err
			}
		}
		if err := sc.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dictionary) unescapeEntry(entry string) string {
	var sb strings.Builder
	end := morphBoundary(entry)
	for i := 0; i < end; {
		r, size := rune(entry[i]), 1
		if r < 128 {
			// ASCII fast path: single byte
		} else {
			r, size = decodeRune(entry[i:])
		}
		i += size
		if r == '\\' && i < end {
			// Escaped character — consume the next rune literally.
			nr, nsize := decodeRune(entry[i:])
			sb.WriteRune(nr)
			i += nsize
		} else if r == '/' && i > 1 {
			sb.WriteRune(flagSeparator)
		} else if r != flagSeparator && r != morphSeparator {
			sb.WriteRune(r)
		}
	}
	sb.WriteRune(morphSeparator)
	if end < len(entry) {
		for _, r := range entry[end:] {
			if r != flagSeparator && r != morphSeparator {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}

// decodeRune decodes the first rune from s and returns it with its byte size.
// Uses range iteration which naturally handles multi-byte UTF-8 sequences.
func decodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	for _, r := range s {
		// The first iteration gives us the first rune.
		// The byte size is len(s) - len(s with first rune removed).
		size := len(string(r))
		return r, size
	}
	return 0, 0
}

func morphBoundary(line string) int {
	end := indexOfSpaceOrTab(line, 0)
	if end < 0 {
		return len(line)
	}
	for end >= 0 && end < len(line) {
		if line[end] == '\t' ||
			(end > 0 && end+3 < len(line) &&
				unicode.IsLetter(rune(line[end+1])) &&
				unicode.IsLetter(rune(line[end+2])) &&
				line[end+3] == ':') {
			break
		}
		end = indexOfSpaceOrTab(line, end+1)
	}
	if end < 0 {
		return len(line)
	}
	return end
}

func indexOfSpaceOrTab(text string, start int) int {
	pos1 := strings.Index(text[start:], "\t")
	pos2 := strings.Index(text[start:], " ")
	if pos1 >= 0 {
		pos1 += start
	}
	if pos2 >= 0 {
		pos2 += start
	}
	if pos1 >= 0 && pos2 >= 0 {
		if pos1 < pos2 {
			return pos1
		}
		return pos2
	}
	if pos1 >= 0 {
		return pos1
	}
	return pos2
}

func (d *Dictionary) writeNormalizedWordEntry(reuse *strings.Builder, line string, acc EntryAccumulator) error {
	flagSepIdx := strings.IndexRune(line, flagSeparator)
	morphSepIdx := strings.IndexRune(line, morphSeparator)
	if morphSepIdx < 0 {
		return nil
	}
	sep := morphSepIdx
	if flagSepIdx >= 0 && flagSepIdx < morphSepIdx {
		sep = flagSepIdx
	}
	if sep == 0 {
		return nil
	}

	beforeSep := line[:sep]
	var toWrite string
	if d.needsInputCleaning(beforeSep) {
		reuse.Reset()
		cleaned := d.cleanInput(beforeSep, reuse)
		toWrite = cleaned + line[sep:]
	} else {
		toWrite = line
	}

	// Re-compute sep for toWrite.
	sep2 := len(toWrite) - (len(line) - sep)
	if err := acc.AddEntry(toWrite); err != nil {
		return err
	}

	wordCase := CaseOfString(toWrite[:sep2])
	if wordCase == WordCaseMixed || (wordCase == WordCaseUpper && flagSepIdx > 0) {
		reuse.Reset()
		if err := d.addHiddenCapitalizedWord(reuse, acc, toWrite[:sep2], toWrite[sep2:]); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dictionary) addHiddenCapitalizedWord(reuse *strings.Builder, acc EntryAccumulator, word, afterSep string) error {
	reuse.Reset()
	rw := []rune(word)
	reuse.WriteRune(unicode.ToUpper(rw[0]))
	for i := 1; i < len(rw); i++ {
		reuse.WriteRune(d.caseFold(rw[i]))
	}
	reuse.WriteRune(flagSeparator)
	reuse.WriteRune(DictionaryHiddenFlag)
	start := 0
	if len(afterSep) > 0 && rune(afterSep[0]) == flagSeparator {
		start = 1
	}
	reuse.WriteString(afterSep[start:])
	return acc.AddEntry(reuse.String())
}

// ─── Sorted dictionary loading ────────────────────────────────────────────────

func (d *Dictionary) readSortedDictionaries(flags *FlagEnumerator, sorted EntrySupplier) (*WordStorage, error) {
	morphIndices := make(map[string]int)

	nonSuggestFlags := d.allNonSuggestibleFlags()
	builder := newWordStorageBuilder(sorted.WordCount(), 1.0, d.hasCustomMorphData, flags, nonSuggestFlags)

	for {
		line, err := sorted.Next()
		if err != nil {
			return nil, err
		}
		if line == "" {
			break
		}

		var entry string
		var wordForm []rune
		var end int

		flagSepIdx := strings.IndexRune(line, flagSeparator)
		morphSepIdx := strings.IndexRune(line, morphSeparator)
		if morphSepIdx < 0 {
			continue
		}

		if flagSepIdx < 0 {
			wordForm = noflags
			end = morphSepIdx
			entry = line[:end]
		} else {
			end = morphSepIdx
			hidden := rune(line[flagSepIdx+1]) == DictionaryHiddenFlag
			flagStart := flagSepIdx + 1
			if hidden {
				flagStart++
			}
			flagPart := strings.TrimSpace(line[flagStart:end])
			if d.aliasCount > 0 && flagPart != "" {
				if idx, err2 := strconv.Atoi(flagPart); err2 == nil {
					flagPart = d.getAliasValue(idx)
				}
			}
			wordForm = d.flagParsingStrategy.ParseFlags(flagPart)
			if hidden {
				wordForm = append(wordForm, DictionaryHiddenFlag)
			}
			entry = line[:flagSepIdx]
		}

		if entry == "" {
			continue
		}

		morphDataID := 0
		if end+1 < len(line) {
			morphFields := d.readMorphFields(entry, line[end+1:])
			if len(morphFields) > 0 {
				sort.Strings(morphFields)
				key := strings.Join(morphFields, " ")
				if id, ok := morphIndices[key]; ok {
					morphDataID = id
				} else {
					morphDataID = len(d.morphData)
					morphIndices[key] = morphDataID
					d.morphData = append(d.morphData, key)
				}
			}
		}

		if err := builder.add(entry, wordForm, morphDataID); err != nil {
			return nil, err
		}
	}

	return builder.build(func(r rune) rune { return d.caseFold(r) }), nil
}

func (d *Dictionary) readMorphFields(word, unparsed string) []string {
	var morphFields []string
	for _, datum := range d.splitMorphData(unparsed) {
		if strings.HasPrefix(datum, "ph:") {
			d.addPhoneticRepEntries(word, datum[3:])
		} else {
			morphFields = append(morphFields, datum)
		}
	}
	return morphFields
}

func (d *Dictionary) splitMorphData(morphData string) []string {
	if d.morphAliasCount > 0 {
		if idx, err := strconv.Atoi(strings.TrimSpace(morphData)); err == nil {
			if idx >= 1 && idx <= len(d.morphAliases) {
				morphData = d.morphAliases[idx-1]
			}
		}
	}
	if strings.TrimSpace(morphData) == "" {
		return nil
	}
	var result []string
	fields := strings.Fields(morphData)
	for _, f := range fields {
		if len(f) > 3 && unicode.IsLetter(rune(f[0])) && unicode.IsLetter(rune(f[1])) && f[2] == ':' {
			result = append(result, f)
		}
	}
	return result
}

func (d *Dictionary) addPhoneticRepEntries(word, ph string) {
	arrow := strings.Index(ph, "->")
	var pattern, replacement string
	if arrow > 0 {
		pattern = ph[:arrow]
		replacement = ph[arrow+2:]
	} else {
		pattern = ph
		replacement = word
	}
	if strings.HasSuffix(pattern, "*") && len(pattern) > 2 && len(replacement) > 1 {
		pattern = pattern[:len(pattern)-2]
		replacement = replacement[:len(replacement)-1]
	}
	if CaseOfString(word) == WordCaseTitle && CaseOfString(pattern) == WordCaseLower {
		if d.hasLanguage("de", "hu") {
			d.repTable = append(d.repTable, NewRepEntry(pattern, d.toLowerCase(replacement)))
		}
		d.repTable = append(d.repTable, NewRepEntry(d.toTitleCase(pattern), replacement))
	}
	d.repTable = append(d.repTable, NewRepEntry(pattern, replacement))
}

// ─── FST lookup ──────────────────────────────────────────────────────────────

// LookupWord returns the forms data for word[offset:offset+length], or nil.
func (d *Dictionary) LookupWord(word []rune, offset, length int) *util.IntsRef {
	if d.words == nil {
		return nil
	}
	return d.words.LookupWord(word, offset, length)
}

// LookupPrefix looks up prefixes matching word (for testing).
func (d *Dictionary) LookupPrefix(word []rune) *util.IntsRef {
	return d.fstLookup(d.prefixes, word)
}

// LookupSuffix looks up suffixes matching word (for testing).
func (d *Dictionary) LookupSuffix(word []rune) *util.IntsRef {
	return d.fstLookup(d.suffixes, word)
}

func (d *Dictionary) fstLookup(fst *gfst.FST[*util.IntsRef], word []rune) *util.IntsRef {
	if fst == nil {
		return nil
	}
	br := fst.GetBytesReader()
	arc := fst.GetFirstArc(new(gfst.Arc[*util.IntsRef]))
	output := fst.Outputs().GetNoOutput()
	for _, r := range word {
		next, err := fst.FindTargetArc(int(r), arc, arc, br)
		if err != nil || next == nil {
			return nil
		}
		output = fst.Outputs().Add(output, arc.Output())
	}
	next, err := fst.FindTargetArc(gfst.END_LABEL, arc, arc, br)
	if err != nil || next == nil {
		return nil
	}
	return fst.Outputs().Add(output, arc.Output())
}

// ─── Flag testing ─────────────────────────────────────────────────────────────

// HasFlag reports whether the entry identified by entryID has the given flag.
func (d *Dictionary) HasFlag(entryID int, flag rune) bool {
	return d.flagLookup.HasFlag(entryID, flag)
}

// HasFlagInForms reports whether any of the forms for a word has the flag.
func (d *Dictionary) HasFlagInForms(forms *util.IntsRef, flag rune) bool {
	step := d.FormStep()
	for i := 0; i < forms.Length; i += step {
		if d.HasFlag(forms.Ints[forms.Offset+i], flag) {
			return true
		}
	}
	return false
}

// IsFlagAppendedByAffix reports whether the affix appends the given flag.
func (d *Dictionary) IsFlagAppendedByAffix(affixID int, flag rune) bool {
	if affixID < 0 || flag == flagUnset {
		return false
	}
	appendID := int(d.AffixData(affixID, AffixAppend))
	return d.HasFlag(appendID, flag)
}

// ─── Case helpers ─────────────────────────────────────────────────────────────

// FormStep returns 1 normally, 2 when custom morphological data is present.
func (d *Dictionary) FormStep() int {
	if d.hasCustomMorphData {
		return 2
	}
	return 1
}

func (d *Dictionary) caseFold(r rune) rune {
	if d.alternateCasing {
		switch r {
		case 'I':
			return 'ı'
		case 'İ':
			return 'i'
		default:
			return unicode.ToLower(r)
		}
	}
	return unicode.ToLower(r)
}

func (d *Dictionary) toLowerCase(word string) string {
	runes := []rune(word)
	for i, r := range runes {
		runes[i] = d.caseFold(r)
	}
	return string(runes)
}

func (d *Dictionary) toTitleCase(word string) string {
	runes := []rune(word)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = d.caseFold(runes[i])
	}
	return string(runes)
}

func (d *Dictionary) IsDotICaseChangeDisallowed(word []rune) bool {
	return len(word) > 0 && word[0] == 'İ' && !d.alternateCasing
}

// ─── Input cleaning ───────────────────────────────────────────────────────────

// MayNeedInputCleaning reports whether any cleaning (case, ignore, iconv) is
// configured.
func (d *Dictionary) MayNeedInputCleaning() bool {
	return d.IgnoreCase || len(d.ignore) > 0 || d.iconv != nil
}

// needsInputCleaning reports whether the specific input string needs cleaning.
func (d *Dictionary) needsInputCleaning(input string) bool {
	if d.MayNeedInputCleaning() {
		for _, ch := range input {
			if len(d.ignore) > 0 && runeInSortedSlice(ch, d.ignore) {
				return true
			}
			if d.IgnoreCase && d.caseFold(ch) != ch {
				return true
			}
			if d.iconv != nil && d.iconv.MightReplaceChar(ch) {
				return true
			}
		}
	}
	return false
}

func runeInSortedSlice(r rune, sorted []rune) bool {
	i := sort.Search(len(sorted), func(i int) bool { return sorted[i] >= r })
	return i < len(sorted) && sorted[i] == r
}

// cleanInput removes ignored characters, applies case folding and iconv
// mappings to input, writing into reuse and returning the resulting string.
func (d *Dictionary) cleanInput(input string, reuse *strings.Builder) string {
	reuse.Reset()
	for _, ch := range input {
		if len(d.ignore) > 0 && runeInSortedSlice(ch, d.ignore) {
			continue
		}
		if d.IgnoreCase && d.iconv == nil {
			ch = d.caseFold(ch)
		}
		reuse.WriteRune(ch)
	}
	if d.iconv != nil {
		sb := &strings.Builder{}
		sb.WriteString(reuse.String())
		d.iconv.ApplyMappings(sb)
		if d.IgnoreCase {
			result := []rune(sb.String())
			for i, r := range result {
				result[i] = d.caseFold(r)
			}
			return string(result)
		}
		return sb.String()
	}
	return reuse.String()
}

// ─── allNonSuggestibleFlags ──────────────────────────────────────────────────

func (d *Dictionary) allNonSuggestibleFlags() []rune {
	set := make(map[rune]struct{})
	set[DictionaryHiddenFlag] = struct{}{}
	for _, c := range []rune{d.noSuggest, d.forbiddenword, d.onlyincompound, d.subStandard} {
		if c != flagUnset {
			set[c] = struct{}{}
		}
	}
	out := make([]rune, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ─── Second-stage flag tests ──────────────────────────────────────────────────

// IsSecondStagePrefix reports whether flag is a second-stage prefix flag.
func (d *Dictionary) IsSecondStagePrefix(flag rune) bool {
	return runeInSortedSlice(flag, d.secondStagePrefixFlags)
}

// IsSecondStageSuffix reports whether flag is a second-stage suffix flag.
func (d *Dictionary) IsSecondStageSuffix(flag rune) bool {
	return runeInSortedSlice(flag, d.secondStageSuffixFlags)
}

// ─── Language ────────────────────────────────────────────────────────────────

func (d *Dictionary) hasLanguage(codes ...string) bool {
	if d.language == "" {
		return false
	}
	langCode := extractLanguageCode(d.language)
	for _, code := range codes {
		if langCode == code {
			return true
		}
	}
	return false
}

func extractLanguageCode(isoCode string) string {
	if idx := strings.IndexByte(isoCode, '_'); idx >= 0 {
		return isoCode[:idx]
	}
	return isoCode
}

// ─── DictEntry / DictEntries ─────────────────────────────────────────────────

// LookupEntries returns the DictEntries for the given root, or nil.
func (d *Dictionary) LookupEntries(root string) DictEntries {
	forms := d.LookupWord([]rune(root), 0, len([]rune(root)))
	if forms == nil {
		return nil
	}
	step := d.FormStep()
	count := forms.Length / step
	entries := make([]DictEntry, count)
	for i := 0; i < count; i++ {
		flagID := forms.Ints[forms.Offset+i*step]
		morphID := 0
		if d.hasCustomMorphData && step == 2 {
			morphID = forms.Ints[forms.Offset+i*2+1]
		}
		entries[i] = d.DictEntryAt(root, flagID, morphID)
	}
	return entries
}

// DictEntryAt returns a DictEntry for the given stem, flag-id and morph-id.
func (d *Dictionary) DictEntryAt(stem string, flagID, morphID int) DictEntry {
	flags := d.flagLookup.GetFlags(flagID)
	morphStr := ""
	if morphID > 0 && morphID < len(d.morphData) {
		morphStr = d.morphData[morphID]
	}
	return NewDictEntryFromData(stem, d.flagParsingStrategy.PrintFlags(flags), morphStr)
}

// ─── Strip data ───────────────────────────────────────────────────────────────

// GetStrip returns the strip data for the given affix (as a []rune slice).
func (d *Dictionary) GetStrip(affixID int) []rune {
	stripOrd := int(d.AffixData(affixID, AffixStripOrd))
	start := d.stripOffsets[stripOrd]
	end := d.stripOffsets[stripOrd+1]
	return d.stripData[start:end]
}

// ─── GetIgnoreCase ────────────────────────────────────────────────────────────

// GetIgnoreCase returns whether the dictionary was built with ignoreCase.
func (d *Dictionary) GetIgnoreCase() bool {
	return d.IgnoreCase
}

// ─── Stemmer-facing accessors ────────────────────────────────────────────────

// Prefixes returns the prefixes FST.
func (d *Dictionary) Prefixes() *gfst.FST[*util.IntsRef] { return d.prefixes }

// Suffixes returns the suffixes FST.
func (d *Dictionary) Suffixes() *gfst.FST[*util.IntsRef] { return d.suffixes }

// Patterns returns the compiled affix conditions.
func (d *Dictionary) Patterns() []*AffixCondition { return d.patterns }

// StripData returns the raw strip-characters array.
func (d *Dictionary) StripData() []rune { return d.stripData }

// StripOffsets returns the strip offset table.
func (d *Dictionary) StripOffsets() []int { return d.stripOffsets }

// MorphData returns the morphological data strings.
func (d *Dictionary) MorphData() []string { return d.morphData }

// HasCustomMorphData reports whether custom morphological data is present.
func (d *Dictionary) HasCustomMorphData() bool { return d.hasCustomMorphData }

// Circumfix returns the circumfix flag.
func (d *Dictionary) Circumfix() rune { return d.circumfix }

// Needaffix returns the needaffix flag.
func (d *Dictionary) Needaffix() rune { return d.needaffix }

// Onlyincompound returns the onlyincompound flag.
func (d *Dictionary) Onlyincompound() rune { return d.onlyincompound }

// CompoundFlag returns the compoundFlag flag.
func (d *Dictionary) CompoundFlag() rune { return d.compoundFlag }

// CompoundForbid returns the compoundForbid flag.
func (d *Dictionary) CompoundForbid() rune { return d.compoundForbid }

// CompoundPermit returns the compoundPermit flag.
func (d *Dictionary) CompoundPermit() rune { return d.compoundPermit }

// FullStrip reports whether FULLSTRIP is set.
func (d *Dictionary) FullStrip() bool { return d.fullStrip }

// CheckSharpS reports whether CHECKSHARPS is set.
func (d *Dictionary) CheckSharpSFlag() bool { return d.CheckSharpS }

// AlternateCasing reports whether alternate casing (tr/az) is in use.
func (d *Dictionary) AlternateCasing() bool { return d.alternateCasing }

// ComplexPrefixes reports whether COMPLEXPREFIXES is set.
func (d *Dictionary) ComplexPrefixes() bool { return d.complexPrefixes }

// Oconv returns the output conversion table, or nil.
func (d *Dictionary) Oconv() *ConvTable { return d.oconv }

// NeedsInputCleaning reports whether the specific input string needs cleaning.
func (d *Dictionary) NeedsInputCleaning(input string) bool {
	return d.needsInputCleaning(input)
}

// CleanInputString cleans the input string writing to reuse, returning the result.
func (d *Dictionary) CleanInputString(input string, reuse *strings.Builder) string {
	return d.cleanInput(input, reuse)
}

// CaseFoldRune folds a rune according to the dictionary's language settings.
func (d *Dictionary) CaseFoldRune(r rune) rune { return d.caseFold(r) }

// NextArc advances the FST arc by character ch, returning the accumulated
// output or nil if there is no arc for ch.
//
// Mirrors Dictionary.nextArc (static) in Apache Lucene 10.4.0.
func NextArc(fst *gfst.FST[*util.IntsRef], arc *gfst.Arc[*util.IntsRef], reader gfst.BytesReader, output *util.IntsRef, ch int) *util.IntsRef {
	next, err := fst.FindTargetArc(ch, arc, arc, reader)
	if err != nil || next == nil {
		return nil
	}
	return fst.Outputs().Add(output, arc.Output())
}
