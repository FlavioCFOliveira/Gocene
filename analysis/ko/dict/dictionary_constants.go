// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// Dictionary constants for the Korean (Nori) binary dictionary format.
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.DictionaryConstants
// from Apache Lucene 10.4.0.
const (
	// DictHeader is the codec header of the dictionary file.
	DictHeader = "ko_dict"

	// TargetMapHeader is the codec header of the dictionary mapping file.
	TargetMapHeader = "ko_dict_map"

	// PosDictHeader is the codec header of the POS dictionary file.
	PosDictHeader = "ko_dict_pos"

	// ConnCostsHeader is the codec header of the connection costs file.
	ConnCostsHeader = "ko_cc"

	// CharDefHeader is the codec header of the character definition file.
	CharDefHeader = "ko_cd"

	// Version is the codec version of the binary dictionary.
	Version = 1
)
