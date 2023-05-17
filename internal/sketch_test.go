package internal

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"
)

func TestEnsureCapacity(t *testing.T) {
	sketch := NewCountMinSketch()
	sketch.ensureCapacity(1)
	require.Equal(t, 16, len(sketch.Table))
}

func TestSketch(t *testing.T) {
	sketch := NewCountMinSketch()
	sketch.ensureCapacity(100)
	require.Equal(t, 128, len(sketch.Table))
	require.Equal(t, uint(1000), sketch.SampleSize)
	// override sampleSize so test won't reset
	sketch.SampleSize = 5120

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
	require.True(t, sketch.Additions > 3500)
	a := sketch.Additions
	sketch.reset()
	require.Equal(t, a>>1, sketch.Additions)
}

func BenchmarkSketch(b *testing.B) {
	sketch := NewCountMinSketch()
	sketch.ensureCapacity(50000000)
	nums := []uint64{}
	for i := 0; i < 100000; i++ {
		h := xxh3.HashString(strconv.Itoa(rand.Intn(100000)))
		nums = append(nums, h)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sketch.Estimate(nums[i%100000])
	}
}
