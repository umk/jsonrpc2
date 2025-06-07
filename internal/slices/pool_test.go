package slices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlicePool_GetAndPut(t *testing.T) {
	pool := NewSlicePool[int](5)

	// Get a slice of size 3
	slice := pool.Get(3)
	assert.Equal(t, 3, len(*slice), "slice length should match requested size")
	assert.Equal(t, 5, cap(*slice), "slice capacity should match pool size")

	// Put the slice back
	pool.Put(slice)

	// Get again, should reuse the pooled slice
	slice2 := pool.Get(2)
	assert.Equal(t, 2, len(*slice2), "reused slice length should match requested size")
	assert.Equal(t, 5, cap(*slice2), "reused slice capacity should match pool size")

	// Get a slice larger than pool size
	bigSlice := pool.Get(10)
	assert.Equal(t, 10, len(*bigSlice), "oversized slice length should match requested size")
	assert.Equal(t, 10, cap(*bigSlice), "oversized slice capacity should match requested size")
}

func TestSlicePool_PutNonPooledSize(t *testing.T) {
	pool := NewSlicePool[int](4)
	s := make([]int, 0, 6)

	// Should not panic or add to pool
	assert.NotPanics(t, func() {
		pool.Put(&s)
	}, "putting non-pooled size slice should not panic")
}
