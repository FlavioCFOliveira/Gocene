// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// geo_encoded_points_compat_test.go addresses the geo audit row
// (verbatim from docs/compat-coverage.tsv): "No fixture comparing
// encoded points emitted by Lucene; relies on algorithmic
// equivalence." Scenario "geo-encoded-points" emits a CodecUtil-framed
// blob of (encodeLatitude, encodeLongitude) int32 pairs (BE) so the
// Gocene side can compare its EncodeLatitude / EncodeLongitude output
// against Lucene-emitted bytes.
package spatial

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"

	gcgeo "github.com/FlavioCFOliveira/Gocene/geo"
)

// TestGeoEncodedPoints_ReadFixture (class a) pins the structural shape
// of geo-encoded-points.bin: it exists, starts with the CodecUtil
// IndexHeader magic, and is large enough to hold the header + 32
// tuples (8 bytes each) + footer.
func TestGeoEncodedPoints_ReadFixture(t *testing.T) {
	const lowerBound = 32 + 32*8 + 16
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioGeoEncodedPoints, seed)
			files := listFiles(t, dir)
			if len(files) != 1 || files[0] != fileGeoEncodedPointsBin {
				t.Fatalf("expected exactly %q under fixture dir, got %v",
					fileGeoEncodedPointsBin, files)
			}
			blob := readFileBytes(t, dir, fileGeoEncodedPointsBin)
			if len(blob) < lowerBound {
				t.Fatalf("%s suspiciously small (%d bytes); expected >= %d",
					fileGeoEncodedPointsBin, len(blob), lowerBound)
			}
			if !bytes.HasPrefix(blob, luceneCodecUtilIndexHeaderMagic) {
				t.Errorf("%s does not start with CodecUtil IndexHeader magic %x; got prefix %x",
					fileGeoEncodedPointsBin, luceneCodecUtilIndexHeaderMagic, blob[:4])
			}
		})
	}
}

// TestGeoEncodedPoints_ByteDeterminism (class b, part 1).
func TestGeoEncodedPoints_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioGeoEncodedPoints, seed)
			b := generate(t, ScenarioGeoEncodedPoints, seed)
			ab := readFileBytes(t, a, fileGeoEncodedPointsBin)
			bb := readFileBytes(t, b, fileGeoEncodedPointsBin)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
					fileGeoEncodedPointsBin, seed, len(ab), len(bb))
			}
		})
	}
}

// TestGeoEncodedPoints_VerifySubcommand (class b, part 2) drives the
// harness `verify` subcommand so Java decodes its own output and
// asserts every (lat,lon) tuple matches the seed-derived expectation.
func TestGeoEncodedPoints_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioGeoEncodedPoints, seed)
			out, err := runHarness(t, "verify", ScenarioGeoEncodedPoints,
				strconv.FormatInt(seed, 10), dir)
			if err != nil {
				t.Fatalf("verify %s failed: %v\nstdout:\n%s",
					ScenarioGeoEncodedPoints, err, out)
			}
			if !strings.Contains(out, "ok scenario="+ScenarioGeoEncodedPoints) {
				t.Errorf("expected 'ok scenario=%s' in stdout, got: %s",
					ScenarioGeoEncodedPoints, out)
			}
		})
	}
}

// TestGeoEncodedPoints_RoundTrip (class c) — the geo encoding helpers
// in geo/geo_encoding_utils.go (EncodeLatitude / EncodeLongitude) DO
// allow a partial round-trip leg: we can parse the CodecUtil-framed
// blob and re-encode each seed-derived (lat,lon) tuple with Gocene's
// helpers, asserting byte equality against the Java-emitted int32
// pairs. This is the only T20 scenario where the round-trip leg is
// achievable today because:
//   - the wire format is a simple CodecUtil envelope + plain int32 pairs
//     (no Spatial4j / SerializableObject dependency);
//   - Gocene's EncodeLatitude / EncodeLongitude mirror Lucene's
//     algorithm bit-for-bit (verified by the existing
//     geo/geo_encoding_utils_test.go suite).
//
// The seed-derived (lat,lon) generator is reimplemented in Go from
// GeoEncodedPointsScenario.seedToLat / seedToLon for parity. If this
// drifts, the test will fail with a per-tuple diff message.
func TestGeoEncodedPoints_RoundTrip(t *testing.T) {
	const auditGap = "No fixture comparing encoded points emitted by Lucene; " +
		"relies on algorithmic equivalence."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioGeoEncodedPoints, seed)
			blob := readFileBytes(t, dir, fileGeoEncodedPointsBin)
			// Strip the 49-byte CodecUtil IndexHeader (4+9+4+16+1; the
			// suffix length is a single byte for the empty suffix string).
			// The vInt count for POINT_COUNT=32 fits in a single byte.
			payload, err := stripCodecUtilHeader(blob, "GoceneGeoEncodedPoints")
			if err != nil {
				t.Fatalf("strip header: %v (audit gap_notes: %q)", err, auditGap)
			}
			count, rest, err := readVInt(payload)
			if err != nil {
				t.Fatalf("read vInt count: %v", err)
			}
			if count <= 0 {
				t.Fatalf("non-positive count: %d", count)
			}
			if len(rest) < count*8+16 { // +16 for footer
				t.Fatalf("payload too short: have %d bytes for count=%d", len(rest), count)
			}
			for i := 0; i < count; i++ {
				wantLat := gcgeo.EncodeLatitude(seedToLat(seed, i))
				wantLon := gcgeo.EncodeLongitude(seedToLon(seed, i))
				// IndexOutput.writeInt is LITTLE-ENDIAN in Lucene 10+
				// (see DataOutput.writeInt line 73 of core/store).
				gotLat := int32(binary.LittleEndian.Uint32(rest[i*8:]))
				gotLon := int32(binary.LittleEndian.Uint32(rest[i*8+4:]))
				if gotLat != wantLat || gotLon != wantLon {
					t.Fatalf("tuple[%d] mismatch: got (%d,%d) want (%d,%d) at seed=%d",
						i, gotLat, gotLon, wantLat, wantLon, seed)
				}
			}
		})
	}
}

// saltLat and saltLon mirror the Java constants
// 0xD1B54A32D192ED03L and 0xAAAAAAAA55555555L (signed long values
// after two's-complement reinterpretation). Go's untyped-int multiply
// wraps the same way as Java's long multiply for the seed-mix step.
const (
	saltLat int64 = -3335678366873096957 // 0xD1B54A32D192ED03 reinterpreted
	saltLon int64 = -6148914692668172971 // 0xAAAAAAAA55555555 reinterpreted
)

// seedToLat mirrors GeoEncodedPointsScenario.seedToLat (Java) bit for
// bit. The SplitMix64-style mix is reproduced inline.
func seedToLat(seed int64, i int) float64 {
	u := splitMix64(uint64(seed)^uint64(int64(i)*saltLat)) & 0xFFFFFFFF
	frac := float64(u) / float64(uint64(1)<<32)
	return -85.0 + frac*170.0
}

// seedToLon mirrors GeoEncodedPointsScenario.seedToLon (Java) bit for bit.
func seedToLon(seed int64, i int) float64 {
	u := splitMix64(uint64(seed)^uint64(int64(i)*saltLon)) & 0xFFFFFFFF
	frac := float64(u) / float64(uint64(1)<<32)
	return -179.0 + frac*358.0
}

// splitMix64 mirrors the Java SplitMix64 finaliser used by the
// scenario's mix() helper.
func splitMix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// stripCodecUtilHeader returns the payload that follows the
// IndexHeader for the named codec. The IndexHeader layout is the
// canonical Lucene 10.4 one:
//
//	int32  CODEC_MAGIC = 0x3FD76C17 (BE)
//	String codec       (BE vInt length + UTF-8 bytes)
//	int32  version     (BE)
//	byte[16] id
//	byte   suffixLen
//	byte[suffixLen] suffix
func stripCodecUtilHeader(blob []byte, codec string) ([]byte, error) {
	if !bytes.HasPrefix(blob, luceneCodecUtilIndexHeaderMagic) {
		return nil, errMissingMagic
	}
	rest := blob[4:]
	// codec name: the writeString uses vInt length + bytes (UTF-8).
	nameLen, rest, err := readVInt(rest)
	if err != nil {
		return nil, err
	}
	if nameLen != len(codec) || string(rest[:nameLen]) != codec {
		return nil, errCodecMismatch
	}
	rest = rest[nameLen:]
	if len(rest) < 4+16+1 {
		return nil, errShortHeader
	}
	rest = rest[4:]    // version (int32 BE)
	rest = rest[16:]   // id
	suffixLen := int(rest[0])
	rest = rest[1:]
	if len(rest) < suffixLen {
		return nil, errShortSuffix
	}
	rest = rest[suffixLen:]
	return rest, nil
}

var (
	errMissingMagic  = errString("codecutil header magic missing")
	errCodecMismatch = errString("codecutil header codec name mismatch")
	errShortHeader   = errString("codecutil header truncated")
	errShortSuffix   = errString("codecutil header suffix truncated")
	errBadVInt       = errString("malformed vInt")
)

type errString string

func (e errString) Error() string { return string(e) }

// readVInt decodes a 7-bit-grouped vInt as produced by
// IndexOutput.writeVInt. Returns the decoded value and the remaining
// slice (excluding the consumed bytes).
func readVInt(b []byte) (int, []byte, error) {
	var v uint32
	var shift uint
	for i := 0; i < 5; i++ {
		if i >= len(b) {
			return 0, nil, errBadVInt
		}
		c := b[i]
		v |= uint32(c&0x7F) << shift
		if c&0x80 == 0 {
			return int(v), b[i+1:], nil
		}
		shift += 7
	}
	return 0, nil, errBadVInt
}
