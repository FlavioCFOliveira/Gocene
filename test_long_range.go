package main

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/document"
)

func main() {
	min := []int64{1, 2}
	max := []int64{3, 4}
	f, err := document.NewLongRangeDocValuesField("test", min, max)
	if err != nil {
		panic(err)
	}
	fmt.Println("Created field:", f.FieldName())
	vMin, _ := f.GetMin(0)
	fmt.Println("Min 0:", vMin)
	vMax, _ := f.GetMax(1)
	fmt.Println("Max 1:", vMax)
}
