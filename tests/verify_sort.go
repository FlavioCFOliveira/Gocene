package main

import (
	"fmt"
	"math"
	"github.com/FlavioCFOliveira/Gocene/spatial3d"
	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
)

func main() {
	pm := geom.WGS84
	encoder := geom.NewDocValueEncoder(pm)

	p1 := geom.NewGeoPointModel(pm, 0, 0)
	p2 := geom.NewGeoPointModel(pm, 0, 0.005)
	p3 := geom.NewGeoPointModel(pm, 0, 0.015)

	vals := []int64{
		encoder.EncodePoint(p1),
		encoder.EncodePoint(p2),
		encoder.EncodePoint(p3),
	}

	shape, _ := geom.MakeGeoCircle(pm, 0, 0, 1000.0/pm.MeanRadius)
	comp := spatial3d.NewGeo3DPointDistanceComparator("field", pm, shape, 3)
	
	// Manually simulate the comparator's logic for a few docs
	// since we can't easily mock SortedNumericDocValues without the index package
	fmt.Println("Verifying distance computation...")
	d1 := comp.ComputeMinimumDistanceMock(vals[0])
	d2 := comp.ComputeMinimumDistanceMock(vals[1])
	d3 := comp.ComputeMinimumDistanceMock(vals[2])
	fmt.Printf("d1: %g, d2: %g, d3: %g\n", d1, d2, d3)
	if d1 < d2 && d2 < d3 {
		fmt.Println("Success: Distances are monotonic")
	} else {
		fmt.Println("Failure: Distances not monotonic")
	}
}
