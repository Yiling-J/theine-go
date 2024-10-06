package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueue_UpdateCost(t *testing.T) {
	q := NewStripedQueue[int, int](1, 10, func() int32 { return -1 })
	entry := &Entry[int, int]{}
	entry.cost.Store(1)
	entry.queueIndex.Store(-2)
	q.Push(20, entry, 1, false)
	require.Equal(t, 1, q.qs[0].len)

	q.UpdateCost(20, entry, 4)
	require.Equal(t, 5, q.qs[0].len)

	q.UpdateCost(20, entry, -2)
	require.Equal(t, 3, q.qs[0].len)
}
