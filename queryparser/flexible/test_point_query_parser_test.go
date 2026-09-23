// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

// Port of
// lucene/queryparser/src/test/org/apache/lucene/queryparser/flexible/standard/TestPointQueryParser.java
// (Apache Lucene 10.5.0).
//
// Blocker: every test method configures
// StandardQueryParser.setPointsConfigMap(Map<String, PointsConfig>) with
// PointsConfig(NumberFormat, Class<? extends Number>) and parses with
// StandardQueryParser.parse(String query, String defaultField). Gocene's
// StandardQueryParser has neither setPointsConfigMap nor the two-argument
// parse, and its PointsConfig carries a PointsType instead of a NumberFormat
// and a Number class, so each test fails naming the missing members.

import "testing"

const pointQueryParserBlocker = "requires org.apache.lucene.queryparser.flexible.standard." +
	"StandardQueryParser.setPointsConfigMap(Map), StandardQueryParser.parse(String, String) and " +
	"PointsConfig(NumberFormat, Class) (not ported)"

func TestPointQueryParser_testIntegers(t *testing.T) {
	t.Fatal(pointQueryParserBlocker)
}

func TestPointQueryParser_testLongs(t *testing.T) {
	t.Fatal(pointQueryParserBlocker)
}

func TestPointQueryParser_testFloats(t *testing.T) {
	t.Fatal(pointQueryParserBlocker)
}

func TestPointQueryParser_testDoubles(t *testing.T) {
	t.Fatal(pointQueryParserBlocker)
}
