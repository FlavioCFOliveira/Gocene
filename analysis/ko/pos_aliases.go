// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import "github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"

// POSType is an alias for dict.POSType so that ko-package code can use the
// bare name without qualification.
type POSType = dict.POSType

// POSTag is an alias for dict.POSTag so that ko-package code can use the
// bare name without qualification.
type POSTag = dict.POSTag

// POS type aliases.
const (
	POSTypeMorpheme    = dict.POSTypeMorpheme
	POSTypeCompound    = dict.POSTypeCompound
	POSTypeInflect     = dict.POSTypeInflect
	POSTypePreanalysis = dict.POSTypePreanalysis
)

// POS tag aliases — only the tags referenced within the ko package.
const (
	POSTagUNKNOWN = dict.POSTagUNKNOWN
	POSTagNNG     = dict.POSTagNNG
)
