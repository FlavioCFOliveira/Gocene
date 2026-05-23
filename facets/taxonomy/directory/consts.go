// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package directory implements the directory-based taxonomy reader and writer
// infrastructure. Mirrors org.apache.lucene.facet.taxonomy.directory.
package directory

// FullPath is the index field that stores the full category path for each
// taxonomy document. Mirrors Consts.FULL.
const FullPath = "$full_path$"

// FieldParentOrdinalNDV is the NumericDocValues field that stores the parent
// ordinal for each taxonomy document. Mirrors Consts.FIELD_PARENT_ORDINAL_NDV.
const FieldParentOrdinalNDV = "$parent_ndv$"
