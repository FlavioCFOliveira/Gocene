package hppc

// IntArrayList is an array-backed list of int.
type IntArrayList struct {
	Buffer        []int
	ElementsCount int
}

func NewIntArrayList() *IntArrayList {
	return NewIntArrayListWithCapacity(DefaultExpectedElements)
}

func NewIntArrayListWithCapacity(expectedElements int) *IntArrayList {
	return &IntArrayList{
		Buffer: make([]int, expectedElements),
	}
}

func (l *IntArrayList) Add(e int) {
	l.ensureBufferSpace(1)
	l.Buffer[l.ElementsCount] = e
	l.ElementsCount++
}

func (l *IntArrayList) AddAll(elements []int) {
	l.ensureBufferSpace(len(elements))
	copy(l.Buffer[l.ElementsCount:], elements)
	l.ElementsCount += len(elements)
}

func (l *IntArrayList) Get(index int) int {
	if index < 0 || index >= l.ElementsCount {
		panic("index out of bounds")
	}
	return l.Buffer[index]
}

func (l *IntArrayList) Set(index, e int) int {
	if index < 0 || index >= l.ElementsCount {
		panic("index out of bounds")
	}
	v := l.Buffer[index]
	l.Buffer[index] = e
	return v
}

func (l *IntArrayList) RemoveAt(index int) int {
	if index < 0 || index >= l.ElementsCount {
		panic("index out of bounds")
	}
	v := l.Buffer[index]
	copy(l.Buffer[index:], l.Buffer[index+1:l.ElementsCount])
	l.ElementsCount--
	return v
}

func (l *IntArrayList) RemoveLast() int {
	if l.ElementsCount == 0 {
		panic("list is empty")
	}
	l.ElementsCount--
	return l.Buffer[l.ElementsCount]
}

func (l *IntArrayList) Size() int {
	return l.ElementsCount
}

func (l *IntArrayList) IsEmpty() bool {
	return l.ElementsCount == 0
}

func (l *IntArrayList) Clear() {
	for i := 0; i < l.ElementsCount; i++ {
		l.Buffer[i] = 0
	}
	l.ElementsCount = 0
}

func (l *IntArrayList) ToArray() []int {
	res := make([]int, l.ElementsCount)
	copy(res, l.Buffer[:l.ElementsCount])
	return res
}

func (l *IntArrayList) ensureBufferSpace(expectedAdditions int) {
	if l.ElementsCount+expectedAdditions > len(l.Buffer) {
		newCap := len(l.Buffer) * 2
		if newCap < l.ElementsCount+expectedAdditions {
			newCap = l.ElementsCount + expectedAdditions
		}
		newBuf := make([]int, newCap)
		copy(newBuf, l.Buffer)
		l.Buffer = newBuf
	}
}
