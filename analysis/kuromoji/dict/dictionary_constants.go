// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// DictionaryConstants holds codec header names and version constants for the
// Kuromoji binary dictionary format.
//
// This is the Go port of org.apache.lucene.analysis.ja.dict.DictionaryConstants
// from Apache Lucene 10.4.0.
//
// Note: these constants are package-internal in the Java reference; they are
// unexported here for the same reason.
const (
	// DictHeader is the codec header of the dictionary file.
	DictHeader = "kuromoji_dict"

	// TargetMapHeader is the codec header of the dictionary mapping file.
	TargetMapHeader = "kuromoji_dict_map"

	// PosDictHeader is the codec header of the POS dictionary file.
	PosDictHeader = "kuromoji_dict_pos"

	// ConnCostsHeader is the codec header of the connection costs file.
	ConnCostsHeader = "kuromoji_cc"

	// CharDefHeader is the codec header of the character definition file.
	CharDefHeader = "kuromoji_cd"

	// Version is the codec version of the binary dictionary.
	Version = 1
)
