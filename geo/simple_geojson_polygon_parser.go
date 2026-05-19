// Code in this file mirrors org.apache.lucene.geo.SimpleGeoJSONPolygonParser
// from Apache Lucene 10.4.0. The Java reference is a hand-rolled
// minimal JSON scanner specialised for extracting (Multi)Polygon
// geometry; this port reproduces its parsing surface and error
// messages byte-for-byte where they are observable to callers.
//
// The parser accepts either:
//
//   - a top-level type: Polygon or MultiPolygon object;
//   - a top-level type: Feature whose geometry is a Polygon /
//     MultiPolygon;
//   - a top-level type: FeatureCollection holding exactly one
//     Feature whose geometry is a Polygon / MultiPolygon.
//
// Anything else is rejected with a ParseException-equivalent error
// carrying the same wording as the Java reference, so existing
// downstream consumers that pattern-match on the message text keep
// working.

package geo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrGeoJSONParse is the sentinel wrapped by every parse error so
// callers can errors.Is to detect GeoJSON-specific failures.
var ErrGeoJSONParse = errors.New("geo: GeoJSON parse error")

// GeoJSONParseError carries the character offset at which parsing
// failed, matching Java's java.text.ParseException.getErrorOffset().
type GeoJSONParseError struct {
	msg    string
	offset int
}

// Error returns the human-readable message; matches the Java format
// "<details> at character offset <offset>; fragment leading to this:\n<fragment>".
func (e *GeoJSONParseError) Error() string { return e.msg }

// Unwrap returns ErrGeoJSONParse so errors.Is detects parse failures.
func (e *GeoJSONParseError) Unwrap() error { return ErrGeoJSONParse }

// ErrorOffset is the byte offset within the input at which the
// parser detected the failure; mirrors ParseException.getErrorOffset().
func (e *GeoJSONParseError) ErrorOffset() int { return e.offset }

// simpleGeoJSONPolygonParser is the Go port of Lucene's
// SimpleGeoJSONPolygonParser. It is intentionally not exported: the
// public entry-point is ParseGeoJSONPolygons (and Polygon.FromGeoJSON
// once that wrapper lands), matching Java where the class itself is
// package-private and the only public surface is
// Polygon.fromGeoJSON(String).
type simpleGeoJSONPolygonParser struct {
	input string
	// upto is the read cursor measured in input string bytes. The
	// Java reference uses java.lang.String.charAt (UTF-16 code units);
	// for the ASCII-only JSON tokens the parser actually inspects
	// (braces, brackets, digits, quotes, alphabetic keywords) byte
	// and code-unit indices coincide. Non-ASCII content can only
	// appear inside string literals, which the parser copies through
	// rune-aware via strings.Builder.
	upto        int
	polyType    string
	coordinates []any
}

// ParseGeoJSONPolygons parses a GeoJSON document and returns its
// (Multi)Polygon geometry as a slice of Polygon. Equivalent to
// Java's Polygon.fromGeoJSON, exposed here so the parser is usable
// without depending on a Polygon-level wrapper.
func ParseGeoJSONPolygons(geoJSON string) ([]Polygon, error) {
	p := &simpleGeoJSONPolygonParser{input: geoJSON}
	return p.parse()
}

// parse is the entry-point: it consumes a single top-level object,
// verifies no trailing garbage, and assembles the captured
// coordinates/polyType into one or more Polygon values.
func (p *simpleGeoJSONPolygonParser) parse() ([]Polygon, error) {
	if err := p.parseObject(""); err != nil {
		return nil, err
	}
	if err := p.readEnd(); err != nil {
		return nil, err
	}

	// JSON object keys may appear in any order, so type/coordinates
	// are reconciled here after the full document is parsed.
	if p.coordinates == nil {
		return nil, p.newParseError("did not see any polygon coordinates")
	}
	if p.polyType == "" {
		return nil, p.newParseError("did not see type: Polygon or MultiPolygon")
	}

	if p.polyType == "Polygon" {
		poly, err := p.parsePolygon(p.coordinates)
		if err != nil {
			return nil, err
		}
		return []Polygon{poly}, nil
	}

	polygons := make([]Polygon, 0, len(p.coordinates))
	for _, o := range p.coordinates {
		inner, ok := o.([]any)
		if !ok {
			return nil, p.newParseError(
				"elements of coordinates array should be an array, but got: %s",
				goTypeName(o))
		}
		poly, err := p.parsePolygon(inner)
		if err != nil {
			return nil, err
		}
		polygons = append(polygons, poly)
	}
	return polygons, nil
}

// parseObject scans { ... } at the current cursor. The path argument
// is the dotted address of the object being parsed (e.g.
// "geometry", "features.[].geometry"); it is used to decide whether
// a nested "type" / "coordinates" key is the polygon geometry we
// care about.
func (p *simpleGeoJSONPolygonParser) parseObject(path string) error {
	if err := p.scanRune('{'); err != nil {
		return err
	}
	first := true
	for {
		ch, err := p.peek()
		if err != nil {
			return err
		}
		if ch == '}' {
			break
		}
		if !first {
			if ch == ',' {
				p.upto++
				ch, err = p.peek()
				if err != nil {
					return err
				}
				if ch == '}' {
					break
				}
			} else {
				return p.newParseError("expected , but got %c", ch)
			}
		}
		first = false

		uptoStart := p.upto
		key, err := p.parseString()
		if err != nil {
			return err
		}

		if path == "crs.properties" && key == "href" {
			p.upto = uptoStart
			return p.newParseError("cannot handle linked crs")
		}

		if err := p.scanRune(':'); err != nil {
			return err
		}

		ch, err = p.peek()
		if err != nil {
			return err
		}
		uptoStart = p.upto

		var o any
		switch {
		case ch == '[':
			newPath := key
			if path != "" {
				newPath = path + "." + key
			}
			arr, err := p.parseArray(newPath)
			if err != nil {
				return err
			}
			o = arr
		case ch == '{':
			newPath := key
			if path != "" {
				newPath = path + "." + key
			}
			if err := p.parseObject(newPath); err != nil {
				return err
			}
			o = nil
		case ch == '"':
			s, err := p.parseString()
			if err != nil {
				return err
			}
			o = s
		case ch == 't':
			if err := p.scanString("true"); err != nil {
				return err
			}
			o = true
		case ch == 'f':
			if err := p.scanString("false"); err != nil {
				return err
			}
			o = false
		case ch == 'n':
			if err := p.scanString("null"); err != nil {
				return err
			}
			o = nil
		case ch == '-' || ch == '.' || (ch >= '0' && ch <= '9'):
			n, err := p.parseNumber()
			if err != nil {
				return err
			}
			o = n
		case ch == '}':
			// Empty object body after a trailing comma; break out of
			// the loop and let the closing scan('}') consume it.
			break
		default:
			return p.newParseError(
				"expected array, object, string or literal value, but got: %c", ch)
		}

		if path == "crs.properties" && key == "name" {
			s, ok := o.(string)
			if !ok {
				p.upto = uptoStart
				return p.newParseError("crs.properties.name should be a string, but saw: %s", javaToString(o))
			}
			if !strings.HasPrefix(s, "urn:ogc:def:crs:OGC") || !strings.HasSuffix(s, ":CRS84") {
				p.upto = uptoStart
				return p.newParseError("crs must be CRS84 from OGC, but saw: %s", javaToString(o))
			}
		}

		if key == "type" && !strings.HasPrefix(path, "crs") {
			s, ok := o.(string)
			if !ok {
				p.upto = uptoStart
				return p.newParseError("type should be a string, but got: %s", javaToString(o))
			}
			switch {
			case s == "Polygon" && isValidGeometryPath(path):
				p.polyType = "Polygon"
			case s == "MultiPolygon" && isValidGeometryPath(path):
				p.polyType = "MultiPolygon"
			case (s == "FeatureCollection" || s == "Feature") &&
				(path == "features.[]" || path == ""):
				// OK, parser recurses into geometry/features.
			default:
				p.upto = uptoStart
				return p.newParseError(
					"can only handle type FeatureCollection (if it has a single polygon geometry), Feature, Polygon or MultiPolygon, but got %s", s)
			}
		} else if key == "coordinates" && isValidGeometryPath(path) {
			arr, ok := o.([]any)
			if !ok {
				p.upto = uptoStart
				return p.newParseError("coordinates should be an array, but got: %s", goTypeName(o))
			}
			if p.coordinates != nil {
				p.upto = uptoStart
				return p.newParseError("only one Polygon or MultiPolygon is supported")
			}
			p.coordinates = arr
		}
	}

	return p.scanRune('}')
}

// isValidGeometryPath reports whether the JSON path is a location at
// which a Polygon/MultiPolygon geometry may legitimately appear:
// either at the document root, inside a Feature's geometry, or
// inside a FeatureCollection feature's geometry.
func isValidGeometryPath(path string) bool {
	return path == "" || path == "geometry" || path == "features.[].geometry"
}

// parsePolygon converts the coordinate array of a single polygon
// (outer ring followed by hole rings) into a Polygon value. The
// expected shape is [[[lon, lat], ...], [[lon, lat], ...], ...]
// where the first inner array is the outer ring.
func (p *simpleGeoJSONPolygonParser) parsePolygon(coords []any) (Polygon, error) {
	if len(coords) == 0 {
		return Polygon{}, p.newParseError(
			"first element of polygon array must be an array [[lat, lon], [lat, lon] ...] but got: null")
	}
	outerAny := coords[0]
	outerList, ok := outerAny.([]any)
	if !ok {
		return Polygon{}, p.newParseError(
			"first element of polygon array must be an array [[lat, lon], [lat, lon] ...] but got: %s",
			javaToString(outerAny))
	}
	outerLats, outerLons, err := p.parsePoints(outerList)
	if err != nil {
		return Polygon{}, err
	}

	holes := make([]Polygon, 0, len(coords)-1)
	for i := 1; i < len(coords); i++ {
		holeAny := coords[i]
		holeList, ok := holeAny.([]any)
		if !ok {
			return Polygon{}, p.newParseError(
				"elements of coordinates array must be an array [[lat, lon], [lat, lon] ...] but got: %s",
				javaToString(holeAny))
		}
		holeLats, holeLons, err := p.parsePoints(holeList)
		if err != nil {
			return Polygon{}, err
		}
		hole, err := NewPolygon(holeLats, holeLons)
		if err != nil {
			return Polygon{}, err
		}
		holes = append(holes, hole)
	}
	return NewPolygon(outerLats, outerLons, holes...)
}

// parsePoints converts a coordinate ring [[lon, lat], ...] into
// parallel lat/lon slices. GeoJSON stores coordinates in lon-lat
// order; the returned slices use Lucene's lat-first ordering to
// match the Polygon constructor.
func (p *simpleGeoJSONPolygonParser) parsePoints(o []any) ([]float64, []float64, error) {
	lats := make([]float64, len(o))
	lons := make([]float64, len(o))
	for i, point := range o {
		pl, ok := point.([]any)
		if !ok {
			return nil, nil, p.newParseError(
				"elements of coordinates array must [lat, lon] array, but got: %s",
				javaToString(point))
		}
		if len(pl) != 2 {
			return nil, nil, p.newParseError(
				"elements of coordinates array must [lat, lon] array, but got wrong element count: %s",
				javaToString(pl))
		}
		lonV, ok := pl[0].(float64)
		if !ok {
			return nil, nil, p.newParseError(
				"elements of coordinates array must [lat, lon] array, but first element is not a Double: %s",
				javaToString(pl[0]))
		}
		latV, ok := pl[1].(float64)
		if !ok {
			return nil, nil, p.newParseError(
				"elements of coordinates array must [lat, lon] array, but second element is not a Double: %s",
				javaToString(pl[1]))
		}
		lons[i] = lonV
		lats[i] = latV
	}
	return lats, lons, nil
}

// parseArray scans a [ ... ] sequence. The path argument is extended
// with ".[]" for each level so isValidGeometryPath can locate
// geometry inside FeatureCollection features.
func (p *simpleGeoJSONPolygonParser) parseArray(path string) ([]any, error) {
	if err := p.scanRune('['); err != nil {
		return nil, err
	}
	result := make([]any, 0)
	for p.upto < len(p.input) {
		ch, err := p.peek()
		if err != nil {
			return nil, err
		}
		if ch == ']' {
			if err := p.scanRune(']'); err != nil {
				return nil, err
			}
			return result, nil
		}
		if len(result) > 0 {
			if ch != ',' {
				return nil, p.newParseError(
					"expected ',' separating list items, but got '%c'", ch)
			}
			p.upto++
			if p.upto == len(p.input) {
				return nil, p.newParseError("hit EOF while parsing array")
			}
			ch, err = p.peek()
			if err != nil {
				return nil, err
			}
		}

		var o any
		switch {
		case ch == '[':
			arr, err := p.parseArray(path + ".[]")
			if err != nil {
				return nil, err
			}
			o = arr
		case ch == '{':
			// Only reached when parsing the "features" array of a
			// FeatureCollection; the nested object is consumed for
			// its side effects (capturing geometry) and represented
			// as nil in the array.
			if err := p.parseObject(path + ".[]"); err != nil {
				return nil, err
			}
			o = nil
		case ch == '-' || ch == '.' || (ch >= '0' && ch <= '9'):
			n, err := p.parseNumber()
			if err != nil {
				return nil, err
			}
			o = n
		case ch == '"':
			s, err := p.parseString()
			if err != nil {
				return nil, err
			}
			o = s
		default:
			return nil, p.newParseError(
				"expected another array or number while parsing array, not '%c'", ch)
		}
		result = append(result, o)
	}
	return nil, p.newParseError("hit EOF while reading array")
}

// parseNumber consumes a JSON number and decodes it as float64. The
// Java reference uses Double.parseDouble; strconv.ParseFloat with
// bitSize 64 matches its accepted input set for the numeric subset
// JSON actually emits.
func (p *simpleGeoJSONPolygonParser) parseNumber() (float64, error) {
	uptoStart := p.upto
	var b strings.Builder
	for p.upto < len(p.input) {
		ch := p.input[p.upto]
		if ch == '-' || ch == '.' || (ch >= '0' && ch <= '9') || ch == 'e' || ch == 'E' {
			b.WriteByte(ch)
			p.upto++
			continue
		}
		break
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		p.upto = uptoStart
		return 0, p.newParseError("could not parse number as double")
	}
	return v, nil
}

// parseString consumes a "..."-quoted string. The Java reference
// supports two escape forms: \\ and \uXXXX (the latter is documented
// as "4 hex digit unicode BMP escape" but in the Java source the
// resulting integer is appended via StringBuilder.append(int), which
// stringifies the number rather than treating it as a codepoint).
// This port preserves that behaviour exactly so error messages and
// captured string values are byte-identical.
func (p *simpleGeoJSONPolygonParser) parseString() (string, error) {
	if err := p.scanRune('"'); err != nil {
		return "", err
	}
	var b strings.Builder
	for p.upto < len(p.input) {
		ch := p.input[p.upto]
		if ch == '"' {
			p.upto++
			return b.String(), nil
		}
		if ch == '\\' {
			p.upto++
			if p.upto == len(p.input) {
				return "", p.newParseError("hit EOF inside string literal")
			}
			esc := p.input[p.upto]
			switch esc {
			case 'u':
				p.upto++
				if p.upto+4 > len(p.input) {
					return "", p.newParseError("hit EOF inside string literal")
				}
				hex := p.input[p.upto : p.upto+4]
				n, err := strconv.ParseInt(hex, 16, 32)
				if err != nil {
					return "", p.newParseError("hit EOF inside string literal")
				}
				// Mirror Java StringBuilder.append(int): the integer
				// is decimal-stringified, not interpreted as a code
				// point.
				b.WriteString(strconv.FormatInt(n, 10))
				p.upto += 4
			case '\\':
				b.WriteByte('\\')
				p.upto++
			default:
				return "", p.newParseError("unsupported string escape character \\%c", esc)
			}
			continue
		}
		b.WriteByte(ch)
		p.upto++
	}
	return "", p.newParseError("hit EOF inside string literal")
}

// peek advances over whitespace and returns the next significant
// byte without consuming it.
func (p *simpleGeoJSONPolygonParser) peek() (byte, error) {
	for p.upto < len(p.input) {
		ch := p.input[p.upto]
		if isJSONWhitespace(ch) {
			p.upto++
			continue
		}
		return ch, nil
	}
	return 0, p.newParseError("unexpected EOF")
}

// scanRune skips whitespace and consumes the expected ASCII byte, or
// returns a parse error.
func (p *simpleGeoJSONPolygonParser) scanRune(expected byte) error {
	for p.upto < len(p.input) {
		ch := p.input[p.upto]
		if isJSONWhitespace(ch) {
			p.upto++
			continue
		}
		if ch != expected {
			return p.newParseError("expected '%c' but got '%c'", expected, ch)
		}
		p.upto++
		return nil
	}
	return p.newParseError("expected '%c' but got EOF", expected)
}

// scanString consumes an exact keyword literal (true / false / null)
// starting at the current cursor.
func (p *simpleGeoJSONPolygonParser) scanString(expected string) error {
	if p.upto+len(expected) > len(p.input) {
		return p.newParseError("expected %q but hit EOF", expected)
	}
	sub := p.input[p.upto : p.upto+len(expected)]
	if sub != expected {
		return p.newParseError("expected %q but got %q", expected, sub)
	}
	p.upto += len(expected)
	return nil
}

// readEnd ensures only whitespace remains after the top-level
// object; any other content is a "trailing garbage" error.
func (p *simpleGeoJSONPolygonParser) readEnd() error {
	for p.upto < len(p.input) {
		ch := p.input[p.upto]
		if !isJSONWhitespace(ch) {
			return p.newParseError("unexpected character '%c' after end of GeoJSON object", ch)
		}
		p.upto++
	}
	return nil
}

// isJSONWhitespace matches the four characters JSON treats as
// whitespace (space, tab, line feed, carriage return).
func isJSONWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// newParseError builds a GeoJSONParseError that includes the same
// "fragment leading to this" suffix as Java's exception message.
func (p *simpleGeoJSONPolygonParser) newParseError(format string, args ...any) error {
	details := fmt.Sprintf(format, args...)
	end := p.upto + 1
	if end > len(p.input) {
		end = len(p.input)
	}
	var fragment string
	if p.upto < 50 {
		fragment = p.input[:end]
	} else {
		fragment = "..." + p.input[p.upto-50:end]
	}
	return &GeoJSONParseError{
		msg:    fmt.Sprintf("%s at character offset %d; fragment leading to this:\n%s", details, p.upto, fragment),
		offset: p.upto,
	}
}

// goTypeName renders the dynamic type of o in a form roughly
// equivalent to Java's Class.toString, used in messages such as
// "elements of coordinates array should be an array, but got: ...".
// The Java reference embeds Class.getClass(); we substitute a
// human-readable Go type so the prefix wording remains identical
// even though the trailing class name differs.
func goTypeName(o any) string {
	if o == nil {
		return "class java.lang.Object"
	}
	switch o.(type) {
	case string:
		return "class java.lang.String"
	case float64:
		return "class java.lang.Double"
	case bool:
		return "class java.lang.Boolean"
	case []any:
		return "class java.util.ArrayList"
	default:
		return fmt.Sprintf("%T", o)
	}
}

// javaToString mirrors Java's String.valueOf(Object): primitives are
// rendered as their literal form, lists as "[a, b, c]", and null as
// the literal "null". It only needs to handle the value types this
// parser ever produces (string, float64, bool, []any, nil).
func javaToString(o any) string {
	if o == nil {
		return "null"
	}
	switch v := o.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return formatJavaDouble(v)
	case []any:
		var b strings.Builder
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(javaToString(e))
		}
		b.WriteByte(']')
		return b.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
