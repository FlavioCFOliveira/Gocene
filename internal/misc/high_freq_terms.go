package misc

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/internal/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const DefaultNumTerms = 100

type TermStatsComparator func(a, b *codecs.TermStats) int

func DocFreqComparator(a, b *codecs.TermStats) int {
	if a.DocFreq != b.DocFreq {
		if a.DocFreq < b.DocFreq {
			return -1
		}
		return 1
	}
	if a.Field != b.Field {
		if a.Field < b.Field {
			return -1
		}
		return 1
	}
	if string(a.TermText) != string(b.TermText) {
		if string(a.TermText) < string(b.TermText) {
			return -1
		}
		return 1
	}
	return 0
}

func TotalTermFreqComparator(a, b *codecs.TermStats) int {
	if a.TotalTermFreq != b.TotalTermFreq {
		if a.TotalTermFreq < b.TotalTermFreq {
			return -1
		}
		return 1
	}
	if a.Field != b.Field {
		if a.Field < b.Field {
			return -1
		}
		return 1
	}
	if string(a.TermText) != string(b.TermText) {
		if string(a.TermText) < string(b.TermText) {
			return -1
		}
		return 1
	}
	return 0
}

func GetHighFreqTerms(reader *index.IndexReader, numTerms int, field string, comparator TermStatsComparator) ([]*codecs.TermStats, error) {
	var allTerms []*codecs.TermStats

	if field != "" {
		terms, err := reader.GetTerms(field)
		if err != nil {
			return nil, err
		}
		if terms == nil {
			return nil, fmt.Errorf("field %s not found", field)
		}
		iter := terms.Iterator()
		for {
			term := iter.Next()
			if term == nil {
				break
			}
			allTerms = append(allTerms, &codecs.TermStats{
				Field:        field,
				TermText:     term.Text,
				DocFreq:      iter.DocFreq(),
				TotalTermFreq: iter.TotalTermFreq(),
			})
		}
	} else {
		fields, err := reader.GetIndexedFields()
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 {
			return nil, fmt.Errorf("no fields found for this index")
		}
		for _, fieldName := range fields {
			terms, err := reader.GetTerms(fieldName)
			if err != nil || terms == nil {
				continue
			}
			iter := terms.Iterator()
			for {
				term := iter.Next()
				if term == nil {
					break
				}
				allTerms = append(allTerms, &codecs.TermStats{
					Field:        fieldName,
					TermText:     term.Text,
					DocFreq:      iter.DocFreq(),
					TotalTermFreq: iter.TotalTermFreq(),
				})
			}
		}
	}

	sort.Slice(allTerms, func(i, j int) bool {
		return comparator(allTerms[i], allTerms[j]) > 0
	})

	if len(allTerms) > numTerms {
		return allTerms[:numTerms], nil
	}
	return allTerms, nil
}

func RunHighFreqTerms(args []string) error {
	if len(args) == 0 || len(args) > 4 {
		fmt.Println("\n\njava org.apache.lucene.misc.HighFreqTerms <index dir> [-t] [number_terms] [field]\n\t -t: order by totalTermFreq\n\n")
		return fmt.Errorf("invalid arguments")
	}

	dirPath := args[0]
	var field string
	numTerms := DefaultNumTerms
	comparator := DocFreqComparator

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-t" {
			comparator = TotalTermFreqComparator
		} else {
			if n, err := strconv.Atoi(arg); err == nil {
				numTerms = n
			} else {
				field = arg
			}
		}
	}

	dir, err := store.FSDirectoryOpen(dirPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	reader, err := index.DirectoryReaderOpen(dir)
	if err != nil {
		return err
	}
	defer reader.Close()

	terms, err := GetHighFreqTerms(reader, numTerms, field, comparator)
	if err != nil {
		return err
	}

	for _, t := range terms {
		fmt.Printf("%s:%s \t totalTF = %d \t docFreq = %d \n",
			t.Field, string(t.TermText), t.TotalTermFreq, t.DocFreq)
	}

	return nil
}
