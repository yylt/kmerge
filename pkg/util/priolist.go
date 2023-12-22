package util

import (
	"github.com/emirpasic/gods/trees/binaryheap"
	"github.com/yylt/kmerge/pkg/lock"
)

type PrioList struct {
	*binaryheap.Heap
	mu lock.Mutex
}

func NewPrioStringList() *PrioList {
	return &PrioList{
		Heap: binaryheap.NewWithStringComparator(),
	}
}

func NewPrioIntList() *PrioList {
	return &PrioList{
		Heap: binaryheap.NewWithIntComparator(),
	}
}

func (pl *PrioList) Push(s any) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.Heap.Push(s)
}

func (pl *PrioList) Pop() (any, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.Heap.Pop()
}
