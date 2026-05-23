// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import "github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"

// Token is a morphological token produced by JapaneseTokenizer.
// It is defined in the dict sub-package to break the import cycle between
// the kuromoji and tokenattributes packages.
//
// This alias allows callers to refer to the type via the kuromoji package.
type Token = dict.Token

// NewToken creates a new Token. See dict.NewToken for parameter details.
var NewToken = dict.NewToken
