// Code in this file mirrors org.apache.lucene.geo.SimpleWKTShapeParser
// from Apache Lucene 10.4.0. The Java reference uses
// java.io.StreamTokenizer; the Go port hand-rolls an equivalent
// tokenizer scoped to the WKT subset Lucene actually parses.

package geo

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// WKT lexical tokens reproduced as exported constants so callers and
// tests can refer to them without re-declaring strings.
const (
	wktEmpty  = "EMPTY"
	wktLParen = "("
	wktRParen = ")"
	wktComma  = ","
	wktNAN    = "NaN"
)

// ShapeType is the WKT geometry type tag accepted by ParseWKT /
// ParseWKTExpected. Mirrors SimpleWKTShapeParser.ShapeType.
type ShapeType int

const (
	// ShapePoint corresponds to WKT "POINT".
	ShapePoint ShapeType = iota
	// ShapeMultiPoint corresponds to WKT "MULTIPOINT".
	ShapeMultiPoint
	// ShapeLineString corresponds to WKT "LINESTRING".
	ShapeLineString
	// ShapeMultiLineString corresponds to WKT "MULTILINESTRING".
	ShapeMultiLineString
	// ShapePolygon corresponds to WKT "POLYGON".
	ShapePolygon
	// ShapeMultiPolygon corresponds to WKT "MULTIPOLYGON".
	ShapeMultiPolygon
	// ShapeGeometryCollection corresponds to WKT "GEOMETRYCOLLECTION".
	ShapeGeometryCollection
	// ShapeEnvelope corresponds to WKT "BBOX" (also accepted as
	// "ENVELOPE" — matches Java's two-entry shapeTypeMap).
	ShapeEnvelope
)

// WKTName returns the canonical WKT name of the shape type. ENVELOPE
// renders as "BBOX" matching Java's wktName().
func (s ShapeType) WKTName() string {
	switch s {
	case ShapePoint:
		return "point"
	case ShapeMultiPoint:
		return "multipoint"
	case ShapeLineString:
		return "linestring"
	case ShapeMultiLineString:
		return "multilinestring"
	case ShapePolygon:
		return "polygon"
	case ShapeMultiPolygon:
		return "multipolygon"
	case ShapeGeometryCollection:
		return "geometrycollection"
	case ShapeEnvelope:
		return "BBOX"
	default:
		return "unknown"
	}
}

// String returns the canonical Java enum name (uppercased typename),
// used in error messages.
func (s ShapeType) String() string {
	switch s {
	case ShapePoint:
		return "POINT"
	case ShapeMultiPoint:
		return "MULTIPOINT"
	case ShapeLineString:
		return "LINESTRING"
	case ShapeMultiLineString:
		return "MULTILINESTRING"
	case ShapePolygon:
		return "POLYGON"
	case ShapeMultiPolygon:
		return "MULTIPOLYGON"
	case ShapeGeometryCollection:
		return "GEOMETRYCOLLECTION"
	case ShapeEnvelope:
		return "ENVELOPE"
	default:
		return "UNKNOWN"
	}
}

// ShapeTypeForName resolves a WKT type name to the corresponding
// ShapeType. Matches Java's ShapeType.forName: case-insensitive
// lookup over both the canonical type names and the "BBOX" alias for
// ENVELOPE.
func ShapeTypeForName(name string) (ShapeType, error) {
	lower := strings.ToLower(name)
	switch lower {
	case "point":
		return ShapePoint, nil
	case "multipoint":
		return ShapeMultiPoint, nil
	case "linestring":
		return ShapeLineString, nil
	case "multilinestring":
		return ShapeMultiLineString, nil
	case "polygon":
		return ShapePolygon, nil
	case "multipolygon":
		return ShapeMultiPolygon, nil
	case "geometrycollection":
		return ShapeGeometryCollection, nil
	case "envelope", "bbox":
		return ShapeEnvelope, nil
	default:
		return 0, fmt.Errorf("geo: unknown geo_shape [%s]", name)
	}
}

// ErrWKTParse is the sentinel wrapped by every parse error so callers
// can errors.Is to detect WKT-specific failures.
var ErrWKTParse = errors.New("geo: WKT parse error")

// wktParseError carries the line number (1-indexed in Java; the Go
// port keeps it at 1 since the parser does not track newlines, which
// the WKT subset Lucene parses cannot contain).
type wktParseError struct {
	msg  string
	line int
}

func (e *wktParseError) Error() string { return e.msg }
func (e *wktParseError) Unwrap() error { return ErrWKTParse }

func newParseError(line int, format string, args ...any) error {
	return &wktParseError{msg: fmt.Sprintf(format, args...), line: line}
}

// ParseWKT parses an arbitrary WKT shape and returns one of:
//
//	*PointCoord (for POINT)
//	[]*PointCoord (for MULTIPOINT)
//	Line / []Line (for LINESTRING / MULTILINESTRING)
//	Polygon / []Polygon (for POLYGON / MULTIPOLYGON)
//	Rectangle (for BBOX / ENVELOPE)
//	[]any (for GEOMETRYCOLLECTION)
//
// The Java reference returns Object; the Go port returns `any` for
// the same dynamic-typing semantics. Callers can use a type switch
// to dispatch on the concrete shape.
func ParseWKT(wkt string) (any, error) {
	return ParseWKTExpected(wkt, -1)
}

// ParseWKTExpected parses a WKT string and additionally enforces the
// top-level shape type. Pass -1 to skip the type check (matching
// Java's `null` argument).
func ParseWKTExpected(wkt string, expected ShapeType) (any, error) {
	tok := newWKTTokenizer(wkt)
	g, err := parseGeometry(tok, expected)
	if err != nil {
		return nil, err
	}
	if err := tok.expectEOF(); err != nil {
		return nil, err
	}
	return g, nil
}

// PointCoord represents a single (lon, lat) coordinate produced by
// the WKT parser for POINT / MULTIPOINT shapes. It matches Java's
// `double[]` of length 2 in shape (lon, lat order).
type PointCoord struct {
	Lon float64
	Lat float64
}

// ----- Parser core -----

func parseGeometry(tok *wktTokenizer, expected ShapeType) (any, error) {
	name, err := tok.nextWord()
	if err != nil {
		return nil, err
	}
	typ, err := ShapeTypeForName(name)
	if err != nil {
		return nil, newParseError(tok.line, "%s", err.Error())
	}
	if expected != -1 && expected != ShapeGeometryCollection {
		if typ.WKTName() != expected.WKTName() {
			return nil, newParseError(tok.line,
				"Expected geometry type: [%s], but found: [%s]", expected, typ)
		}
	}
	switch typ {
	case ShapePoint:
		return parseWKTPoint(tok)
	case ShapeMultiPoint:
		return parseWKTMultiPoint(tok)
	case ShapeLineString:
		return parseWKTLine(tok)
	case ShapeMultiLineString:
		return parseWKTMultiLine(tok)
	case ShapePolygon:
		return parseWKTPolygon(tok)
	case ShapeMultiPolygon:
		return parseWKTMultiPolygon(tok)
	case ShapeEnvelope:
		return parseWKTBBox(tok)
	case ShapeGeometryCollection:
		return parseWKTGeometryCollection(tok)
	default:
		return nil, newParseError(tok.line, "Unknown geometry type: %v", typ)
	}
}

func parseWKTPoint(tok *wktTokenizer) (*PointCoord, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	lon, err := tok.nextNumber()
	if err != nil {
		return nil, err
	}
	lat, err := tok.nextNumber()
	if err != nil {
		return nil, err
	}
	// Optional third dimension (Z) silently consumed.
	if hasNumber, err := tok.isNumberNext(); err != nil {
		return nil, err
	} else if hasNumber {
		if _, err := tok.nextNumber(); err != nil {
			return nil, err
		}
	}
	if _, err := tok.nextCloser(); err != nil {
		return nil, err
	}
	return &PointCoord{Lon: lon, Lat: lat}, nil
}

func parseWKTMultiPoint(tok *wktTokenizer) ([]*PointCoord, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	lats, lons, err := parseCoordinates(tok)
	if err != nil {
		return nil, err
	}
	out := make([]*PointCoord, len(lats))
	for i := range lats {
		out[i] = &PointCoord{Lon: lons[i], Lat: lats[i]}
	}
	return out, nil
}

func parseWKTLine(tok *wktTokenizer) (*Line, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	lats, lons, err := parseCoordinates(tok)
	if err != nil {
		return nil, err
	}
	l, err := NewLine(lats, lons)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func parseWKTMultiLine(tok *wktTokenizer) ([]*Line, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	first, err := parseWKTLine(tok)
	if err != nil {
		return nil, err
	}
	out := []*Line{first}
	for {
		next, err := tok.nextCloserOrComma()
		if err != nil {
			return nil, err
		}
		if next == wktRParen {
			break
		}
		l, err := parseWKTLine(tok)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func parseWKTPolygonHole(tok *wktTokenizer) (*Polygon, error) {
	lats, lons, err := parseCoordinates(tok)
	if err != nil {
		return nil, err
	}
	p, err := NewPolygon(lats, lons)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func parseWKTPolygon(tok *wktTokenizer) (*Polygon, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	if _, err := tok.nextOpener(); err != nil {
		return nil, err
	}
	lats, lons, err := parseCoordinates(tok)
	if err != nil {
		return nil, err
	}
	var holes []Polygon
	for {
		next, err := tok.nextCloserOrComma()
		if err != nil {
			return nil, err
		}
		if next == wktRParen {
			break
		}
		h, err := parseWKTPolygonHole(tok)
		if err != nil {
			return nil, err
		}
		holes = append(holes, *h)
	}
	p, err := NewPolygon(lats, lons, holes...)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func parseWKTMultiPolygon(tok *wktTokenizer) ([]*Polygon, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	first, err := parseWKTPolygon(tok)
	if err != nil {
		return nil, err
	}
	out := []*Polygon{first}
	for {
		next, err := tok.nextCloserOrComma()
		if err != nil {
			return nil, err
		}
		if next == wktRParen {
			break
		}
		p, err := parseWKTPolygon(tok)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func parseWKTBBox(tok *wktTokenizer) (*Rectangle, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	minLon, err := tok.nextNumber()
	if err != nil {
		return nil, err
	}
	if _, err := tok.nextComma(); err != nil {
		return nil, err
	}
	maxLon, err := tok.nextNumber()
	if err != nil {
		return nil, err
	}
	if _, err := tok.nextComma(); err != nil {
		return nil, err
	}
	maxLat, err := tok.nextNumber()
	if err != nil {
		return nil, err
	}
	if _, err := tok.nextComma(); err != nil {
		return nil, err
	}
	minLat, err := tok.nextNumber()
	if err != nil {
		return nil, err
	}
	if _, err := tok.nextCloser(); err != nil {
		return nil, err
	}
	r, err := NewRectangle(minLat, maxLat, minLon, maxLon)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func parseWKTGeometryCollection(tok *wktTokenizer) ([]any, error) {
	w, err := tok.nextEmptyOrOpen()
	if err != nil {
		return nil, err
	}
	if w == wktEmpty {
		return nil, nil
	}
	first, err := parseGeometry(tok, ShapeGeometryCollection)
	if err != nil {
		return nil, err
	}
	out := []any{first}
	for {
		next, err := tok.nextCloserOrComma()
		if err != nil {
			return nil, err
		}
		if next == wktRParen {
			break
		}
		g, err := parseGeometry(tok, -1)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// parseCoordinates fills lats/lons with the comma-separated
// coordinates between the current "(" and the matching ")". Handles
// both bare and parenthesised coordinates (Java's WKT subset accepts
// both "(1 2, 3 4)" and "((1 2), (3 4))"). The implementation
// mirrors Java's stateful isOpenParen tracking precisely so the
// matching ")" for a leading "(" is deferred to the post-loop check,
// not consumed eagerly after the first coordinate.
func parseCoordinates(tok *wktTokenizer) ([]float64, []float64, error) {
	var lats, lons []float64
	isOpenParen := false

	// First coordinate or "(".
	if isNum, err := tok.isNumberNext(); err != nil {
		return nil, nil, err
	} else if isNum {
		if err := parseCoordinate(tok, &lats, &lons); err != nil {
			return nil, nil, err
		}
	} else {
		w, err := tok.nextWord()
		if err != nil {
			return nil, nil, err
		}
		if w != wktLParen {
			return nil, nil, newParseError(tok.line,
				"expected: [(] or number but found: [%s]", w)
		}
		isOpenParen = true
		if err := parseCoordinate(tok, &lats, &lons); err != nil {
			return nil, nil, err
		}
	}

	for {
		next, err := tok.nextCloserOrComma()
		if err != nil {
			return nil, nil, err
		}
		if next == wktRParen {
			break
		}
		// COMMA case.
		isOpenParen = false
		if isNum, err := tok.isNumberNext(); err != nil {
			return nil, nil, err
		} else if isNum {
			if err := parseCoordinate(tok, &lats, &lons); err != nil {
				return nil, nil, err
			}
		} else {
			w, err := tok.nextWord()
			if err != nil {
				return nil, nil, err
			}
			if w != wktLParen {
				return nil, nil, newParseError(tok.line,
					"expected: [(] or number but found: [%s]", w)
			}
			isOpenParen = true
			if err := parseCoordinate(tok, &lats, &lons); err != nil {
				return nil, nil, err
			}
		}
		if isOpenParen {
			if _, err := tok.nextCloser(); err != nil {
				return nil, nil, err
			}
		}
	}

	// Post-loop: if the last iteration left isOpenParen=true (single
	// per-coord paren case) we still need to consume the matching
	// ")". Mirrors Java's trailing if-block.
	if isOpenParen {
		if _, err := tok.nextCloser(); err != nil {
			return nil, nil, err
		}
	}
	return lats, lons, nil
}

func parseCoordinate(tok *wktTokenizer, lats, lons *[]float64) error {
	lon, err := tok.nextNumber()
	if err != nil {
		return err
	}
	lat, err := tok.nextNumber()
	if err != nil {
		return err
	}
	*lons = append(*lons, lon)
	*lats = append(*lats, lat)
	// Optional third dimension.
	if hasNum, err := tok.isNumberNext(); err != nil {
		return err
	} else if hasNum {
		if _, err := tok.nextNumber(); err != nil {
			return err
		}
	}
	return nil
}

// ----- Tokenizer -----

// wktTokenizer is a tiny hand-rolled tokenizer that recognises words
// (sequences of letters / digits / sign / dot), parentheses, commas,
// and whitespace. It matches the StreamTokenizer configuration the
// Java reference uses for WKT.
type wktTokenizer struct {
	src      string
	pos      int
	line     int
	pushedTT int    // pushback for isNumberNext
	pushedSV string // pushback word value
	hasPush  bool
}

const (
	ttWord  = 1
	ttRunep = 2 // punctuation: (, ), ,
	ttEOF   = -1
)

func newWKTTokenizer(s string) *wktTokenizer {
	return &wktTokenizer{src: s, line: 1}
}

// nextToken returns the next token type and its string value. For
// punctuation the string holds the single character.
func (t *wktTokenizer) nextToken() (int, string, error) {
	if t.hasPush {
		t.hasPush = false
		return t.pushedTT, t.pushedSV, nil
	}
	// Skip whitespace (Java treats 0..space as whitespace) and
	// '#' comments to end-of-line.
	for t.pos < len(t.src) {
		c := t.src[t.pos]
		if c <= ' ' {
			if c == '\n' {
				t.line++
			}
			t.pos++
			continue
		}
		if c == '#' {
			// Skip to newline.
			for t.pos < len(t.src) && t.src[t.pos] != '\n' {
				t.pos++
			}
			continue
		}
		break
	}
	if t.pos >= len(t.src) {
		return ttEOF, "", nil
	}
	c := t.src[t.pos]
	switch c {
	case '(', ')', ',':
		t.pos++
		return ttRunep, string(c), nil
	}
	// Word: letters / digits / sign / dot / 128..255 (StreamTokenizer
	// includes wordChars(128+32, 255) which covers high-byte chars
	// often present in Latin-1 input).
	start := t.pos
	for t.pos < len(t.src) {
		c := t.src[t.pos]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '+' || c == '.':
		case c >= 0xA0:
		default:
			goto end
		}
		t.pos++
	}
end:
	if t.pos == start {
		// Should be unreachable because the switch above handles
		// every non-whitespace ASCII char.
		return ttEOF, "", newParseError(t.line, "unexpected character: %q", c)
	}
	return ttWord, t.src[start:t.pos], nil
}

// pushBack stores the most recently returned token so the next call
// to nextToken re-emits it. Mirrors StreamTokenizer.pushBack.
func (t *wktTokenizer) pushBack(tt int, sv string) {
	t.pushedTT = tt
	t.pushedSV = sv
	t.hasPush = true
}

func (t *wktTokenizer) nextWord() (string, error) {
	tt, sv, err := t.nextToken()
	if err != nil {
		return "", err
	}
	switch tt {
	case ttWord:
		if strings.EqualFold(sv, wktEmpty) {
			return wktEmpty, nil
		}
		return sv, nil
	case ttRunep:
		return sv, nil
	case ttEOF:
		return "", newParseError(t.line, "expected word but found: END-OF-STREAM")
	}
	return "", newParseError(t.line, "expected word but found unknown token")
}

func (t *wktTokenizer) nextNumber() (float64, error) {
	tt, sv, err := t.nextToken()
	if err != nil {
		return 0, err
	}
	if tt != ttWord {
		return 0, newParseError(t.line, "expected number but found: %s", tokenDisplay(tt, sv))
	}
	if strings.EqualFold(sv, wktNAN) {
		return math.NaN(), nil
	}
	v, err := strconv.ParseFloat(sv, 64)
	if err != nil {
		return 0, newParseError(t.line, "invalid number found: %s", sv)
	}
	return v, nil
}

func (t *wktTokenizer) isNumberNext() (bool, error) {
	tt, sv, err := t.nextToken()
	if err != nil {
		return false, err
	}
	t.pushBack(tt, sv)
	return tt == ttWord, nil
}

func (t *wktTokenizer) nextEmptyOrOpen() (string, error) {
	w, err := t.nextWord()
	if err != nil {
		return "", err
	}
	if w == wktEmpty || w == wktLParen {
		return w, nil
	}
	return "", newParseError(t.line, "expected EMPTY or ( but found: %s", w)
}

func (t *wktTokenizer) nextCloser() (string, error) {
	w, err := t.nextWord()
	if err != nil {
		return "", err
	}
	if w == wktRParen {
		return wktRParen, nil
	}
	return "", newParseError(t.line, "expected ) but found: %s", w)
}

func (t *wktTokenizer) nextComma() (string, error) {
	w, err := t.nextWord()
	if err != nil {
		return "", err
	}
	if w == wktComma {
		return wktComma, nil
	}
	return "", newParseError(t.line, "expected , but found: %s", w)
}

func (t *wktTokenizer) nextOpener() (string, error) {
	w, err := t.nextWord()
	if err != nil {
		return "", err
	}
	if w == wktLParen {
		return wktLParen, nil
	}
	return "", newParseError(t.line, "expected ( but found: %s", w)
}

func (t *wktTokenizer) nextCloserOrComma() (string, error) {
	w, err := t.nextWord()
	if err != nil {
		return "", err
	}
	if w == wktComma || w == wktRParen {
		return w, nil
	}
	return "", newParseError(t.line, "expected , or ) but found: %s", w)
}

func (t *wktTokenizer) expectEOF() error {
	tt, sv, err := t.nextToken()
	if err != nil {
		return err
	}
	if tt != ttEOF {
		return newParseError(t.line,
			"expected end of WKT string but found additional text: %s", tokenDisplay(tt, sv))
	}
	return nil
}

func tokenDisplay(tt int, sv string) string {
	switch tt {
	case ttWord:
		return sv
	case ttRunep:
		return fmt.Sprintf("'%s'", sv)
	case ttEOF:
		return "END-OF-STREAM"
	}
	return "<UNKNOWN>"
}
