package internal

import (
	"fmt"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/require"
)

func TestSketch(t *testing.T) {
	sketch := NewCountMinSketch(100)
	require.Equal(t, uint(512), sketch.rowCounterSize)
	require.Equal(t, uint(511), sketch.rowMask)
	require.Equal(t, 128, len(sketch.table))
	require.Equal(t, uint(5120), sketch.sampleSize)

	failed := 0
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("key:%d", i)
		keyh := xxhash.Sum64String(key)
		sketch.Add(keyh)
		sketch.Add(keyh)
		sketch.Add(keyh)
		sketch.Add(keyh)
		sketch.Add(keyh)
		key = fmt.Sprintf("key:%d:b", i)
		keyh2 := xxhash.Sum64String(key)
		sketch.Add(keyh2)
		sketch.Add(keyh2)
		sketch.Add(keyh2)

		es1 := sketch.Estimate(keyh)
		es2 := sketch.Estimate(keyh2)
		if es2 > es1 {
			failed++
		}
		require.True(t, es1 >= 5)
		require.True(t, es2 >= 3)

	}
	require.True(t, float32(failed)/4000 < 0.1)
	require.True(t, sketch.additions > 3500)
	a := sketch.additions
	sketch.reset()
	require.Equal(t, a>>1, sketch.additions)
}
