// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package histograms implements histogram collection over numeric index
// fields. It is a port of the
// org.apache.lucene.sandbox.facet.plain.histograms Java package from
// Apache Lucene 10.4.0.
//
// The package provides three main types:
//
//   - HistogramCollectorManager — a CollectorManager that configures and
//     reduces histogram collectors for use with IndexSearcher.
//   - HistogramCollector — a per-search Collector that accumulates per-bucket
//     document counts using doc values.
//   - PointTreeBulkCollector — a bulk-collection optimisation that traverses
//     the BKD point tree directly when the field is indexed as points and the
//     point density is higher than the bucket count.
package histograms
