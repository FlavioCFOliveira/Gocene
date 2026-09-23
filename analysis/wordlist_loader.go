// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// WordlistLoader is a loader for text files that represent a list of
// stopwords.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.WordlistLoader
// (lucene/core/src/java/org/apache/lucene/analysis/WordlistLoader.java). The
// Java class only has static members; they are rendered as package-level
// functions.
//
// Renderings:
//   - java.io.Reader and java.io.InputStream are both rendered as io.Reader.
//     A Reader carries text, which Gocene represents as UTF-8 bytes; an
//     InputStream carries bytes that are decoded with a charset through
//     util.GetDecodingReader (IOUtils.getDecodingReader).
//   - java.nio.charset.Charset is rendered as encoding.Encoding;
//     StandardCharsets.UTF_8 is unicode.UTF8.
//   - Java overloads follow the Gocene convention: the overload with the
//     fewest parameters keeps the base name, "With<Role>" names an added
//     parameter, and "From<Type>" names a distinguishing parameter type.
//   - Each method reads its input through a BufferedReader inside
//     try-with-resources, so the reader is closed afterwards: Gocene closes
//     the reader when it implements io.Closer. A close failure is returned
//     when reading succeeded and is attached to the read error otherwise, as
//     Java adds it as a suppressed exception.
//
// See util.GetDecodingReader to obtain decoding readers.
type WordlistLoader struct{}

// wordlistLoaderInitialCapacity is WordlistLoader.INITIAL_CAPACITY.
const wordlistLoaderInitialCapacity = 16

// GetWordSetWithResult reads lines from reader and adds every non-blank line
// as an entry to result (omitting leading and trailing whitespace). Every line
// of the reader should contain only one word. The words need to be in
// lowercase if you make use of an Analyzer which uses LowerCaseFilter (like
// StandardAnalyzer).
//
// It returns the given result with the reader's words.
//
// Port of WordlistLoader.getWordSet(Reader, CharArraySet).
func GetWordSetWithResult(reader io.Reader, result *CharArraySet) (_ *CharArraySet, err error) {
	br := getBufferedReader(reader)
	defer func() { err = closeWordlistReader(reader, err) }()
	for {
		word, ok, rerr := br.readLine()
		if rerr != nil {
			return nil, rerr
		}
		if !ok {
			break
		}
		word = javaTrim(word)
		// skip blank lines
		if word == "" {
			continue
		}
		result.Add(word)
	}
	return result, nil
}

// GetWordSet reads lines from reader and adds every line as an entry to a
// CharArraySet (omitting leading and trailing whitespace). Every line of the
// reader should contain only one word. The words need to be in lowercase if
// you make use of an Analyzer which uses LowerCaseFilter (like
// StandardAnalyzer).
//
// It returns an unmodifiable CharArraySet with the reader's words.
//
// Port of WordlistLoader.getWordSet(Reader).
func GetWordSet(reader io.Reader) (*UnmodifiableCharArraySet, error) {
	set, err := GetWordSetWithResult(reader, NewCharArraySet(wordlistLoaderInitialCapacity, false))
	if err != nil {
		return nil, err
	}
	return UnmodifiableSet(set), nil
}

// GetWordSetFromInputStream reads lines from stream with the UTF-8 charset
// and adds every line as an entry to a CharArraySet (omitting leading and
// trailing whitespace). Every line should contain only one word. The words
// need to be in lowercase if you make use of an Analyzer which uses
// LowerCaseFilter (like StandardAnalyzer).
//
// It returns an unmodifiable CharArraySet with the stream's words.
//
// Port of WordlistLoader.getWordSet(InputStream).
func GetWordSetFromInputStream(stream io.Reader) (*UnmodifiableCharArraySet, error) {
	return GetWordSetFromInputStreamWithCharset(stream, unicode.UTF8)
}

// GetWordSetFromInputStreamWithCharset reads lines from stream with the given
// charset and adds every line as an entry to a CharArraySet (omitting leading
// and trailing whitespace). Every line should contain only one word. The
// words need to be in lowercase if you make use of an Analyzer which uses
// LowerCaseFilter (like StandardAnalyzer).
//
// It returns an unmodifiable CharArraySet with the stream's words.
//
// Port of WordlistLoader.getWordSet(InputStream, Charset).
func GetWordSetFromInputStreamWithCharset(stream io.Reader, charset encoding.Encoding) (*UnmodifiableCharArraySet, error) {
	return GetWordSet(util.GetDecodingReader(stream, charset))
}

// GetWordSetWithCommentAndResult reads lines from reader and adds every
// non-blank non-comment line as an entry to result (omitting leading and
// trailing whitespace). A line is a comment when it starts with comment
// (tested before trimming); as in Java, an empty comment string marks every
// line as a comment. Every line of the reader should contain only one word.
// The words need to be in lowercase if you make use of an Analyzer which uses
// LowerCaseFilter (like StandardAnalyzer).
//
// It returns the given result with the reader's words.
//
// Port of WordlistLoader.getWordSet(Reader, String, CharArraySet).
func GetWordSetWithCommentAndResult(reader io.Reader, comment string, result *CharArraySet) (_ *CharArraySet, err error) {
	br := getBufferedReader(reader)
	defer func() { err = closeWordlistReader(reader, err) }()
	for {
		word, ok, rerr := br.readLine()
		if rerr != nil {
			return nil, rerr
		}
		if !ok {
			break
		}
		if !strings.HasPrefix(word, comment) {
			word = javaTrim(word)
			// skip blank lines
			if word == "" {
				continue
			}
			result.Add(word)
		}
	}
	return result, nil
}

// GetWordSetWithComment reads lines from reader and adds every non-comment
// line as an entry to a CharArraySet (omitting leading and trailing
// whitespace). Every line of the reader should contain only one word. The
// words need to be in lowercase if you make use of an Analyzer which uses
// LowerCaseFilter (like StandardAnalyzer).
//
// It returns an unmodifiable CharArraySet with the reader's words.
//
// Port of WordlistLoader.getWordSet(Reader, String).
func GetWordSetWithComment(reader io.Reader, comment string) (*UnmodifiableCharArraySet, error) {
	set, err := GetWordSetWithCommentAndResult(reader, comment, NewCharArraySet(wordlistLoaderInitialCapacity, false))
	if err != nil {
		return nil, err
	}
	return UnmodifiableSet(set), nil
}

// GetWordSetFromInputStreamWithComment reads lines from stream with the UTF-8
// charset and adds every non-comment line as an entry to a CharArraySet
// (omitting leading and trailing whitespace). Every line should contain only
// one word. The words need to be in lowercase if you make use of an Analyzer
// which uses LowerCaseFilter (like StandardAnalyzer).
//
// It returns an unmodifiable CharArraySet with the stream's words.
//
// Port of WordlistLoader.getWordSet(InputStream, String).
func GetWordSetFromInputStreamWithComment(stream io.Reader, comment string) (*UnmodifiableCharArraySet, error) {
	return GetWordSetFromInputStreamWithCharsetAndComment(stream, unicode.UTF8, comment)
}

// GetWordSetFromInputStreamWithCharsetAndComment reads lines from stream with
// the given charset and adds every non-comment line as an entry to a
// CharArraySet (omitting leading and trailing whitespace). Every line should
// contain only one word. The words need to be in lowercase if you make use of
// an Analyzer which uses LowerCaseFilter (like StandardAnalyzer).
//
// It returns an unmodifiable CharArraySet with the stream's words.
//
// Port of WordlistLoader.getWordSet(InputStream, Charset, String).
func GetWordSetFromInputStreamWithCharsetAndComment(stream io.Reader, charset encoding.Encoding, comment string) (*UnmodifiableCharArraySet, error) {
	return GetWordSetWithComment(util.GetDecodingReader(stream, charset), comment)
}

// GetSnowballWordSetWithResult reads stopwords from a stopword list in
// Snowball format and adds them to result.
//
// The snowball format is the following:
//   - Lines may contain multiple words separated by whitespace.
//   - The comment character is the vertical line (|).
//   - Lines may contain trailing comments.
//
// It returns the given result with the reader's words.
//
// Port of WordlistLoader.getSnowballWordSet(Reader, CharArraySet).
func GetSnowballWordSetWithResult(reader io.Reader, result *CharArraySet) (_ *CharArraySet, err error) {
	br := getBufferedReader(reader)
	defer func() { err = closeWordlistReader(reader, err) }()
	for {
		line, ok, rerr := br.readLine()
		if rerr != nil {
			return nil, rerr
		}
		if !ok {
			break
		}
		if comment := strings.IndexByte(line, '|'); comment >= 0 {
			line = line[:comment]
		}
		// line.split("\\s+"): Java's \s is the ASCII class [ \t\n\x0B\f\r].
		// Empty strings are skipped by the length check, so FieldsFunc
		// yields the same words.
		for _, word := range strings.FieldsFunc(line, isJavaRegexWhitespace) {
			if len(word) > 0 {
				result.Add(word)
			}
		}
	}
	return result, nil
}

// GetSnowballWordSet reads stopwords from a stopword list in Snowball format.
//
// The snowball format is the following:
//   - Lines may contain multiple words separated by whitespace.
//   - The comment character is the vertical line (|).
//   - Lines may contain trailing comments.
//
// It returns an unmodifiable CharArraySet with the reader's words.
//
// Port of WordlistLoader.getSnowballWordSet(Reader).
func GetSnowballWordSet(reader io.Reader) (*UnmodifiableCharArraySet, error) {
	set, err := GetSnowballWordSetWithResult(reader, NewCharArraySet(wordlistLoaderInitialCapacity, false))
	if err != nil {
		return nil, err
	}
	return UnmodifiableSet(set), nil
}

// GetSnowballWordSetFromInputStream reads stopwords from a stopword list in
// Snowball format, encoded in UTF-8.
//
// The snowball format is the following:
//   - Lines may contain multiple words separated by whitespace.
//   - The comment character is the vertical line (|).
//   - Lines may contain trailing comments.
//
// It returns an unmodifiable CharArraySet with the stream's words.
//
// Port of WordlistLoader.getSnowballWordSet(InputStream).
func GetSnowballWordSetFromInputStream(stream io.Reader) (*UnmodifiableCharArraySet, error) {
	return GetSnowballWordSetFromInputStreamWithCharset(stream, unicode.UTF8)
}

// GetSnowballWordSetFromInputStreamWithCharset reads stopwords from a
// stopword list in Snowball format, encoded in the given charset.
//
// The snowball format is the following:
//   - Lines may contain multiple words separated by whitespace.
//   - The comment character is the vertical line (|).
//   - Lines may contain trailing comments.
//
// It returns an unmodifiable CharArraySet with the stream's words.
//
// Port of WordlistLoader.getSnowballWordSet(InputStream, Charset).
func GetSnowballWordSetFromInputStreamWithCharset(stream io.Reader, charset encoding.Encoding) (*UnmodifiableCharArraySet, error) {
	return GetSnowballWordSet(util.GetDecodingReader(stream, charset))
}

// GetStemDict reads a stem dictionary into result. Each line contains
// "word\tstem" (two tab-separated words); the stem is everything after the
// first tab.
//
// It returns the stem dictionary that overrules the stemming algorithm. As in
// Java, where line.split("\t", 2)[1] throws ArrayIndexOutOfBoundsException, a
// line without a tab fails; the entries of the preceding lines stay in result.
//
// Port of WordlistLoader.getStemDict(Reader, CharArrayMap<String>).
func GetStemDict(reader io.Reader, result *CharArrayMap[string]) (_ *CharArrayMap[string], err error) {
	br := getBufferedReader(reader)
	defer func() { err = closeWordlistReader(reader, err) }()
	for {
		line, ok, rerr := br.readLine()
		if rerr != nil {
			return nil, rerr
		}
		if !ok {
			break
		}
		wordstem := strings.SplitN(line, "\t", 2)
		if len(wordstem) < 2 {
			return nil, fmt.Errorf("ArrayIndexOutOfBoundsException: Index 1 out of bounds for length %d", len(wordstem))
		}
		result.Put(wordstem[0], wordstem[1])
	}
	return result, nil
}

// GetLines reads stream with the given charset and returns the non-blank,
// non-comment lines with leading and trailing whitespace removed. A comment
// line is any line that starts with the character "#". A leading byte order
// mark (U+FEFF) is removed from each line read while no line has been kept
// yet.
//
// Port of WordlistLoader.getLines(InputStream, Charset).
func GetLines(stream io.Reader, charset encoding.Encoding) (_ []string, err error) {
	input := util.GetDecodingReader(stream, charset)
	br := getBufferedReader(input)
	defer func() { err = closeWordlistReader(input, err) }()
	lines := []string{}
	for {
		word, ok, rerr := br.readLine()
		if rerr != nil {
			return nil, rerr
		}
		if !ok {
			break
		}
		// skip initial bom marker
		if len(lines) == 0 && strings.HasPrefix(word, "\uFEFF") {
			word = word[len("\uFEFF"):]
		}
		// skip comments
		if strings.HasPrefix(word, "#") {
			continue
		}
		word = javaTrim(word)
		// skip blank lines
		if len(word) == 0 {
			continue
		}
		lines = append(lines, word)
	}
	return lines, nil
}

// wordlistBufferedReader renders java.io.BufferedReader.readLine over a byte
// reader.
type wordlistBufferedReader struct {
	r      io.ByteReader
	skipLF bool
}

// getBufferedReader renders WordlistLoader.getBufferedReader(Reader): a
// reader that already reads byte by byte from a buffer (as *bufio.Reader and
// the reader of util.GetDecodingReader do, rendering "instanceof
// BufferedReader") is used as is; any other reader is wrapped in a
// *bufio.Reader.
func getBufferedReader(reader io.Reader) *wordlistBufferedReader {
	if br, ok := reader.(io.ByteReader); ok {
		return &wordlistBufferedReader{r: br}
	}
	return &wordlistBufferedReader{r: bufio.NewReader(reader)}
}

// readLine renders BufferedReader.readLine: a line is terminated by "\n",
// "\r" or "\r\n", the terminator is not returned, and ok is false at the end
// of the stream when no character was read.
func (b *wordlistBufferedReader) readLine() (line string, ok bool, err error) {
	var sb []byte
	read := false
	for {
		c, rerr := b.r.ReadByte()
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return string(sb), read, nil
			}
			return "", false, rerr
		}
		if b.skipLF {
			b.skipLF = false
			if c == '\n' {
				continue
			}
		}
		switch c {
		case '\n':
			return string(sb), true, nil
		case '\r':
			b.skipLF = true
			return string(sb), true, nil
		}
		read = true
		sb = append(sb, c)
	}
}

// closeWordlistReader renders the close of Java's try-with-resources: the
// reader is closed when it implements io.Closer; a close failure is returned
// when err is nil and attached to err otherwise.
func closeWordlistReader(reader io.Reader, err error) error {
	c, ok := reader.(io.Closer)
	if !ok {
		return err
	}
	cerr := c.Close()
	switch {
	case err == nil:
		return cerr
	case cerr != nil:
		return fmt.Errorf("%w (suppressed: %w)", err, cerr)
	default:
		return err
	}
}

// javaTrim renders String.trim(): it removes every leading and trailing
// character whose code is at most U+0020. In UTF-8 those are exactly the
// bytes 0x00-0x20.
func javaTrim(s string) string {
	start, end := 0, len(s)
	for start < end && s[start] <= ' ' {
		start++
	}
	for end > start && s[end-1] <= ' ' {
		end--
	}
	return s[start:end]
}

// isJavaRegexWhitespace reports whether r belongs to Java's regex class \s,
// which is [ \t\n\x0B\f\r].
func isJavaRegexWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\x0B', '\f', '\r':
		return true
	}
	return false
}
