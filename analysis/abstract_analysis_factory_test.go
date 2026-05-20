// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/TestAbstractAnalysisFactory.java
//
// Deviation: the Java test exercises AnalysisSPILoader.lookupSPIName and
// XXXFactory.findSPIName, which are Java reflection-based ServiceLoader
// features with no direct Go equivalent. Go tests verify the arg-parsing
// contract of AbstractAnalysisFactory directly.

package analysis

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestAbstractAnalysisFactory_DefaultVersion verifies that omitting
// luceneMatchVersion defaults to util.Latest and isExplicitLuceneMatchVersion
// is false.
func TestAbstractAnalysisFactory_DefaultVersion(t *testing.T) {
	args := map[string]string{"foo": "bar"}
	f, err := NewAbstractAnalysisFactory(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.GetLuceneMatchVersion() != util.Latest {
		t.Fatalf("expected Latest version, got %v", f.GetLuceneMatchVersion())
	}
	if f.IsExplicitLuceneMatchVersion() {
		t.Fatal("expected isExplicitLuceneMatchVersion=false")
	}
}

// TestAbstractAnalysisFactory_ExplicitVersion verifies version parsing and
// the explicit-version flag.
func TestAbstractAnalysisFactory_ExplicitVersion(t *testing.T) {
	args := map[string]string{LuceneMatchVersionParam: "10.4.0"}
	f, err := NewAbstractAnalysisFactory(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v := f.GetLuceneMatchVersion()
	if v.Major != 10 || v.Minor != 4 || v.Bugfix != 0 {
		t.Fatalf("unexpected version: %v", v)
	}
	if !f.IsExplicitLuceneMatchVersion() {
		t.Fatal("expected isExplicitLuceneMatchVersion=true")
	}
}

// TestAbstractAnalysisFactory_InvalidVersion verifies error on unparseable
// version string.
func TestAbstractAnalysisFactory_InvalidVersion(t *testing.T) {
	args := map[string]string{LuceneMatchVersionParam: "not-a-version"}
	_, err := NewAbstractAnalysisFactory(args)
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

// TestAbstractAnalysisFactory_OriginalArgs verifies that getOriginalArgs
// returns a defensive copy with the original (pre-consumption) keys.
func TestAbstractAnalysisFactory_OriginalArgs(t *testing.T) {
	args := map[string]string{
		LuceneMatchVersionParam: "10.4.0",
		"class":                 "MyFactory",
		"name":                  "myFactory",
		"key":                   "value",
	}
	f, err := NewAbstractAnalysisFactory(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	orig := f.GetOriginalArgs()
	if orig[LuceneMatchVersionParam] != "10.4.0" {
		t.Errorf("expected luceneMatchVersion in originalArgs")
	}
	if orig["key"] != "value" {
		t.Errorf("expected key=value in originalArgs")
	}
	// Modifying the returned map must not affect internal state.
	orig["injected"] = "bad"
	if f.GetOriginalArgs()["injected"] != "" {
		t.Error("originalArgs copy is not defensive")
	}
}

// TestAbstractAnalysisFactory_ConsumesClassAndName verifies that the
// constructor removes the "class" and "name" keys from the working args map.
func TestAbstractAnalysisFactory_ConsumesClassAndName(t *testing.T) {
	args := map[string]string{
		"class":  "com.example.MyFactory",
		"name":   "myFactory",
		"actual": "param",
	}
	_, err := NewAbstractAnalysisFactory(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := args["class"]; ok {
		t.Error("constructor must consume 'class' key")
	}
	if _, ok := args["name"]; ok {
		t.Error("constructor must consume 'name' key")
	}
	if args["actual"] != "param" {
		t.Error("non-reserved key must survive constructor")
	}
}

// TestAbstractAnalysisFactory_GetClassArg verifies getClassArg when the
// class arg is and is not present.
func TestAbstractAnalysisFactory_GetClassArg(t *testing.T) {
	f, _ := NewAbstractAnalysisFactory(map[string]string{"class": "com.example.Foo"})
	if f.GetClassArg() != "com.example.Foo" {
		t.Fatalf("expected class arg, got %q", f.GetClassArg())
	}

	f2, _ := NewAbstractAnalysisFactory(map[string]string{})
	if f2.GetClassArg() != "" {
		t.Fatalf("expected empty class arg, got %q", f2.GetClassArg())
	}
}

// TestAbstractAnalysisFactory_SetExplicit verifies the explicit-version setter.
func TestAbstractAnalysisFactory_SetExplicit(t *testing.T) {
	f, _ := NewAbstractAnalysisFactory(map[string]string{})
	f.SetExplicitLuceneMatchVersion(true)
	if !f.IsExplicitLuceneMatchVersion() {
		t.Fatal("expected true after SetExplicitLuceneMatchVersion(true)")
	}
}

// ─── Require / Get helpers ───────────────────────────────────────────────────

func TestRequire_Present(t *testing.T) {
	args := map[string]string{"k": "v"}
	s, err := Require(args, "k")
	if err != nil || s != "v" {
		t.Fatalf("Require: expected v, got %q err=%v", s, err)
	}
	if _, ok := args["k"]; ok {
		t.Error("Require must consume the key")
	}
}

func TestRequire_Missing(t *testing.T) {
	_, err := Require(map[string]string{}, "k")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestRequireWithAllowed_Valid(t *testing.T) {
	args := map[string]string{"k": "B"}
	s, err := RequireWithAllowed(args, "k", []string{"A", "B", "C"}, true)
	if err != nil || s != "B" {
		t.Fatalf("RequireWithAllowed: expected B, got %q err=%v", s, err)
	}
}

func TestRequireWithAllowed_Invalid(t *testing.T) {
	args := map[string]string{"k": "D"}
	_, err := RequireWithAllowed(args, "k", []string{"A", "B", "C"}, true)
	if err == nil {
		t.Fatal("expected error for disallowed value")
	}
}

func TestRequireWithAllowed_CaseInsensitive(t *testing.T) {
	args := map[string]string{"k": "b"}
	s, err := RequireWithAllowed(args, "k", []string{"A", "B", "C"}, false)
	if err != nil || s != "b" {
		t.Fatalf("RequireWithAllowed case-insensitive: expected b, got %q err=%v", s, err)
	}
}

func TestGet_Present(t *testing.T) {
	args := map[string]string{"k": "v"}
	s, ok := Get(args, "k")
	if !ok || s != "v" {
		t.Fatalf("Get: expected v, got %q ok=%v", s, ok)
	}
}

func TestGet_Missing(t *testing.T) {
	_, ok := Get(map[string]string{}, "k")
	if ok {
		t.Fatal("Get: expected ok=false for missing key")
	}
}

func TestGetWithDefault_Present(t *testing.T) {
	args := map[string]string{"k": "found"}
	if got := GetWithDefault(args, "k", "default"); got != "found" {
		t.Fatalf("expected found, got %q", got)
	}
}

func TestGetWithDefault_Missing(t *testing.T) {
	if got := GetWithDefault(map[string]string{}, "k", "default"); got != "default" {
		t.Fatalf("expected default, got %q", got)
	}
}

func TestGetWithAllowed_Valid(t *testing.T) {
	args := map[string]string{"k": "A"}
	s, err := GetWithAllowed(args, "k", []string{"A", "B"}, "X", true)
	if err != nil || s != "A" {
		t.Fatalf("expected A, got %q err=%v", s, err)
	}
}

func TestGetWithAllowed_Missing(t *testing.T) {
	s, err := GetWithAllowed(map[string]string{}, "k", []string{"A", "B"}, "X", true)
	if err != nil || s != "X" {
		t.Fatalf("expected default X, got %q err=%v", s, err)
	}
}

func TestGetWithAllowed_Invalid(t *testing.T) {
	args := map[string]string{"k": "Z"}
	_, err := GetWithAllowed(args, "k", []string{"A", "B"}, "X", true)
	if err == nil {
		t.Fatal("expected error for disallowed value")
	}
}

func TestRequireInt(t *testing.T) {
	args := map[string]string{"n": "42"}
	n, err := RequireInt(args, "n")
	if err != nil || n != 42 {
		t.Fatalf("expected 42, got %d err=%v", n, err)
	}
}

func TestGetInt(t *testing.T) {
	if n, err := GetInt(map[string]string{"n": "7"}, "n", 0); err != nil || n != 7 {
		t.Fatalf("GetInt: expected 7, got %d err=%v", n, err)
	}
	if n, err := GetInt(map[string]string{}, "n", 99); err != nil || n != 99 {
		t.Fatalf("GetInt default: expected 99, got %d err=%v", n, err)
	}
}

func TestRequireBoolean(t *testing.T) {
	args := map[string]string{"b": "true"}
	b, err := RequireBoolean(args, "b")
	if err != nil || !b {
		t.Fatalf("expected true, got %v err=%v", b, err)
	}
}

func TestGetBoolean(t *testing.T) {
	if b, err := GetBoolean(map[string]string{"b": "false"}, "b", true); err != nil || b {
		t.Fatalf("GetBoolean: expected false, got %v err=%v", b, err)
	}
	if b, err := GetBoolean(map[string]string{}, "b", true); err != nil || !b {
		t.Fatalf("GetBoolean default: expected true, got %v err=%v", b, err)
	}
}

func TestRequireFloat(t *testing.T) {
	args := map[string]string{"f": "3.14"}
	f, err := RequireFloat(args, "f")
	if err != nil || f < 3.13 || f > 3.15 {
		t.Fatalf("expected ~3.14, got %f err=%v", f, err)
	}
}

func TestGetFloat(t *testing.T) {
	if f, err := GetFloat(map[string]string{"f": "1.5"}, "f", 0); err != nil || f != 1.5 {
		t.Fatalf("GetFloat: expected 1.5, got %f err=%v", f, err)
	}
	if f, err := GetFloat(map[string]string{}, "f", 2.0); err != nil || f != 2.0 {
		t.Fatalf("GetFloat default: expected 2.0, got %f err=%v", f, err)
	}
}

func TestRequireChar(t *testing.T) {
	args := map[string]string{"c": "|"}
	c, err := RequireChar(args, "c")
	if err != nil || c != '|' {
		t.Fatalf("expected '|', got %c err=%v", c, err)
	}
	_, err = RequireChar(map[string]string{"c": "ab"}, "c")
	if err == nil {
		t.Fatal("expected error for multi-char value")
	}
}

func TestGetChar(t *testing.T) {
	c, err := GetChar(map[string]string{"c": ","}, "c", '|')
	if err != nil || c != ',' {
		t.Fatalf("GetChar: expected ',', got %c err=%v", c, err)
	}
	c, err = GetChar(map[string]string{}, "c", '|')
	if err != nil || c != '|' {
		t.Fatalf("GetChar default: expected '|', got %c err=%v", c, err)
	}
}

func TestGetSet(t *testing.T) {
	set := GetSet(map[string]string{"s": "a, b,c"}, "s")
	if len(set) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(set), set)
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, ok := set[k]; !ok {
			t.Errorf("expected %q in set", k)
		}
	}
}

func TestGetSet_Missing(t *testing.T) {
	if set := GetSet(map[string]string{}, "s"); set != nil {
		t.Fatalf("expected nil for missing key, got %v", set)
	}
}

func TestGetPattern_Valid(t *testing.T) {
	args := map[string]string{"p": `\d+`}
	r, err := GetPattern(args, "p")
	if err != nil || r == nil {
		t.Fatalf("GetPattern: expected compiled regex, err=%v", err)
	}
	if !r.MatchString("123") {
		t.Error("expected pattern to match digits")
	}
}

func TestGetPattern_Invalid(t *testing.T) {
	args := map[string]string{"p": "[invalid"}
	_, err := GetPattern(args, "p")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

// ─── SplitAt / SplitFileNames ─────────────────────────────────────────────────

func TestSplitAt_Basic(t *testing.T) {
	result := SplitAt(',', "a,b,c")
	if len(result) != 3 || result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Fatalf("SplitAt: expected [a b c], got %v", result)
	}
}

func TestSplitAt_EscapedSeparator(t *testing.T) {
	result := SplitAt(',', `a\,b,c`)
	if len(result) != 2 || result[0] != "a,b" || result[1] != "c" {
		t.Fatalf("SplitAt escaped: expected [a,b c], got %v", result)
	}
}

func TestSplitAt_Empty(t *testing.T) {
	if result := SplitAt(',', ""); result != nil {
		t.Fatalf("SplitAt empty: expected nil, got %v", result)
	}
}

func TestSplitFileNames_Basic(t *testing.T) {
	result := SplitFileNames("file1.txt,file2.txt, file3.txt")
	if len(result) != 3 {
		t.Fatalf("SplitFileNames: expected 3, got %v", result)
	}
}
