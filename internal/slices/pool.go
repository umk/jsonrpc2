package slices

import (
	"sync"
)

type SlicePool[T any] struct {
	p    sync.Pool
	size int
}

func NewSlicePool[T any](size int) *SlicePool[T] {
	return &SlicePool[T]{
		p: sync.Pool{
			New: func() any {
				slice := make([]T, 0, size)
				return &slice
			},
		},
		size: size,
	}
}

func (sp *SlicePool[T]) Get(size int) *[]T {
	if size > sp.size {
		s := make([]T, size)
		return &s
	} else {
		s := sp.p.Get().(*[]T)
		*s = (*s)[:size]
		return s
	}
}

func (sp *SlicePool[T]) Put(s *[]T) {
	if cap(*s) == sp.size {
		sp.p.Put(s)
	}
}
