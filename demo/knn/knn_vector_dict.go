package knn

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

var spaceRE = regexp.MustCompile(` `)

// KnnVectorDict manages a map from token to numeric vector for use with KnnVector indexing and search.
// The map is stored as an FST: token-to-ordinal plus a dense binary file holding the vectors.
type KnnVectorDict struct {
	fst       *fst.FST[int64]
	vectors   store.IndexInput
	dimension int
}

// NewKnnVectorDict is the sole constructor.
// directory: Lucene directory from which knn directory should be read.
// dictName: the base name of the directory files that store the knn vector dictionary.
// A file with extension '.bin' holds the vectors and the '.fst' maps tokens to offsets in the '.bin' file.
func NewKnnVectorDict(directory store.Directory, dictName string) (*KnnVectorDict, error) {
	var fstIn store.IndexInput
	var err error
	if fstIn, err = directory.OpenInput(dictName+".fst", store.DefaultIOContext); err != nil {
		return nil, err
	}

	// Lucene: fst = new FST<>(readMetadata(fstIn, PositiveIntOutputs.getSingleton()), fstIn);
	metadata, err := fst.ReadMetadata(fstIn, fst.PositiveIntOutputs)
	if err != nil {
		fstIn.Close()
		return nil, err
	}

	f, err := fst.NewFST(metadata, fstIn)
	fstIn.Close()
	if err != nil {
		return nil, err
	}

	vectors, err := directory.OpenInput(dictName+".bin", store.DefaultIOContext)
	if err != nil {
		return nil, err
	}

	size := vectors.Length()
	if size < 4 {
		vectors.Close()
		return nil, errors.New("knn vector bin file too small")
	}

	vectors.Seek(size - 4)
	dimension, err := vectors.ReadInt()
	if err != nil {
		vectors.Close()
		return nil, err
	}

	if (size-4)%(int64(dimension)*4) != 0 {
		vectors.Close()
		return nil, fmt.Errorf("vector file size %d is not consonant with the vector dimension %d", size, dimension)
	}

	return &KnnVectorDict{
		fst:       f,
		vectors:   vectors,
		dimension: dimension,
	}, nil
}

// Get the vector corresponding to the given token.
// NOTE: the returned array is shared and its contents will be overwritten by subsequent calls.
// The caller is responsible to copy the data as needed.
// token: the token to look up.
// output: the array in which to write the corresponding vector. Its length must be GetDimension() * 4.
// It will be filled with zeros if the token is not present in the dictionary.
func (k *KnnVectorDict) Get(token *util.BytesRef, output []byte) error {
	if len(output) != k.dimension*4 {
		return fmt.Errorf("the output array must be of length %d, got %d", k.dimension*4, len(output))
	}

	// Util.get(fst, token) -> GetBytesRef(fst, token)
	ord, found, err := fst.GetBytesRef(k.fst, token)
	if err != nil {
		return err
	}

	if !found {
		for i := range output {
			output[i] = 0
		}
	} else {
		k.vectors.Seek(ord * int64(k.dimension) * 4)
		if _, err := k.vectors.ReadBytes(output); err != nil {
			return err
		}
	}
	return nil
}

// GetDimension returns the dimension of the vectors returned by this.
func (k *KnnVectorDict) GetDimension() int {
	return k.dimension
}

// Close closes the underlying vector input.
func (k *KnnVectorDict) Close() error {
	return k.vectors.Close()
}

// RamBytesUsed returns the size of the dictionary in bytes.
func (k *KnnVectorDict) RamBytesUsed() int64 {
	return k.fst.RamBytesUsed() + k.vectors.Length()
}

// Build converts from a GloVe-formatted dictionary file to a KnnVectorDict file pair.
// gloveInput: the path to the input dictionary. The dictionary is delimited by newlines,
// and each line is space-delimited. The first column has the token, and the remaining columns
// are the vector components, as text. The dictionary must be sorted by its leading tokens.
// directory: a Lucene directory to write the dictionary to.
// dictName: base name for the knn dictionary files.
func Build(gloveInput string, directory store.Directory, dictName string) error {
	b := &builder{
		intsRefBuilder: util.NewIntsRefBuilder(),
		fstCompiler:    fst.NewFSTCompiler(fst.InputTypeByte1, fst.PositiveIntOutputs),
	}
	return b.build(gloveInput, directory, dictName)
}

type builder struct {
	intsRefBuilder *util.IntsRefBuilder
	fstCompiler    *fst.FSTCompiler[int64]
	scratch        []float32
	ordinal        int64
	numFields      int
}

func (b *builder) build(gloveInput string, directory store.Directory, dictName string) error {
	f, err := os.Open(gloveInput)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	binOut, err := directory.CreateOutput(dictName+".bin", store.DefaultIOContext)
	if err != nil {
		return err
	}
	defer binOut.Close()

	fstOut, err := directory.CreateOutput(dictName+".fst", store.DefaultIOContext)
	if err != nil {
		return err
	}
	defer fstOut.Close()

	if err := b.writeFirstLine(reader, binOut); err != nil {
		return err
	}

	for {
		if err := b.addOneLine(reader, binOut); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	// FST.fromFSTReader(fstCompiler.compile(), fstCompiler.getFSTReader()).save(fstOut, fstOut);
	compiledFST := b.fstCompiler.Compile()
	f, err := fst.NewFST(compiledFST.Metadata(), compiledFST.Reader())
	if err != nil {
		return err
	}
	if err := f.Save(fstOut, fstOut); err != nil {
		return err
	}

	return binOut.WriteInt(b.numFields - 1)
}

func (b *builder) writeFirstLine(in *bufio.Reader, out store.IndexOutput) error {
	fields, err := b.readOneLine(in)
	if err != nil {
		return err
	}
	if fields == nil {
		return nil
	}
	b.numFields = len(fields)
	b.scratch = make([]float32, b.numFields-1)
	return b.writeVector(fields, out)
}

func (b *builder) readOneLine(in *bufio.Reader) ([]string, error) {
	line, err := in.ReadString('\n')
	if err != nil && len(line) == 0 {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return nil, nil
	}
	return spaceRE.Split(line, 0), nil
}

func (b *builder) addOneLine(in *bufio.Reader, out store.IndexOutput) error {
	fields, err := b.readOneLine(in)
	if err != nil {
		return err
	}
	if fields == nil {
		return io.EOF
	}
	if len(fields) != b.numFields {
		return fmt.Errorf("different field count at line %d got %d when expecting %d", b.ordinal, len(fields), b.numFields)
	}

	// fstCompiler.add(Util.toIntsRef(new BytesRef(fields[0]), intsRefBuilder), ordinal++);
	token := &util.BytesRef{Bytes: []byte(fields[0])}
	intsRef := fst.ToIntsRef(token, b.intsRefBuilder)
	b.fstCompiler.Add(intsRef, b.ordinal)
	b.ordinal++

	if err := b.writeVector(fields, out); err != nil {
		return err
	}
	return nil
}

func (b *builder) writeVector(fields []string, out store.IndexOutput) error {
	for i := 1; i < len(fields); i++ {
		val, err := strconv.ParseFloat(fields[i], 32)
		if err != nil {
			return err
		}
		b.scratch[i-1] = float32(val)
	}

	util.L2Normalize(b.scratch)

	buf := make([]byte, len(b.scratch)*4)
	for i, v := range b.scratch {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}

	_, err := out.WriteBytes(buf)
	return err
}
