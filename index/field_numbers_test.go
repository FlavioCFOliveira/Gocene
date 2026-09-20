package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// TestFieldNumbersAddOrGetAssignsDistinctNumbers is a regression test for a
// defect in FieldNumbers.AddOrGet: the "find a new FieldNumber" branch was
// unreachable because fieldNumber was left at its Go zero value (0) instead of
// -1, so every field that could not reuse its own number was silently assigned
// number 0.
//
// Field numbers are written into the .fnm file and referenced from every other
// per-field structure, so a collision here is a binary-compatibility defect.
//
// Mirrors the contract of org.apache.lucene.index.FieldInfos.FieldNumbers#addOrGet
// of Apache Lucene 10.5.0:
//
//	if (fi.number != -1 && numberToName.containsKey(fi.number) == false) {
//	  fieldNumber = fi.number;
//	} else {
//	  while (numberToName.containsKey(++lowestUnassignedFieldNumber)) {}
//	  fieldNumber = lowestUnassignedFieldNumber;
//	}
func TestFieldNumbersAddOrGetAssignsDistinctNumbers(t *testing.T) {
	t.Run("unnumbered fields get distinct ascending numbers", func(t *testing.T) {
		fn := spi.NewFieldNumbers("", "")
		got := make(map[int]string)
		for _, name := range []string{"a", "b", "c"} {
			num := fn.AddOrGet(spi.NewFieldInfo(name, -1, spi.FieldInfoOptions{}))
			if prev, clash := got[num]; clash {
				t.Fatalf("field %q was assigned number %d, already held by %q", name, num, prev)
			}
			got[num] = name
		}
		for _, want := range []int{0, 1, 2} {
			if _, ok := got[want]; !ok {
				t.Fatalf("expected number %d to be assigned; assignments = %v", want, got)
			}
		}
	})

	t.Run("a taken number is not reused", func(t *testing.T) {
		fn := spi.NewFieldNumbers("", "")
		if n := fn.AddOrGet(spi.NewFieldInfo("first", 0, spi.FieldInfoOptions{})); n != 0 {
			t.Fatalf("first field: got number %d, want 0", n)
		}
		// "second" also declares number 0, which is already taken, so Lucene
		// allocates the next free number instead of colliding.
		n := fn.AddOrGet(spi.NewFieldInfo("second", 0, spi.FieldInfoOptions{}))
		if n == 0 {
			t.Fatalf("second field collided on number 0 with the first field")
		}
		if n != 1 {
			t.Fatalf("second field: got number %d, want 1", n)
		}
	})
}
