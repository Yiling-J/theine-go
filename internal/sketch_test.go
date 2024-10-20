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

func TestSketch_EnsureCapacity(t *testing.T) {
	sketch := NewCountMinSketch()
	sketch.EnsureCapacity(1)
	require.Equal(t, 16, len(sketch.Table))
}

func TestSketch_Basic(t *testing.T) {
	sketch := NewCountMinSketch()
	sketch.EnsureCapacity(100)
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
}

func TestSketch_ResetFreq(t *testing.T) {
	sketch := NewCountMinSketch()
	sketch.EnsureCapacity(1000)
	for i := 0; i < len(sketch.Table); i++ {
		sketch.Table[i].Store(^uint64(0))
	}
	keyh := xxhash.Sum64String("key1")
	require.Equal(t, 15, int(sketch.Estimate(keyh)))
	sketch.reset()
	require.Equal(t, 7, int(sketch.Estimate(keyh)))

}

func TestSketch_ResetAddition(t *testing.T) {
	sketch := NewCountMinSketch()
	sketch.EnsureCapacity(100)
	require.Equal(t, 128, len(sketch.Table))
	require.Equal(t, uint(1000), sketch.SampleSize)
	// override sampleSize so test won't reset
	sketch.SampleSize = 5120

	keyh := xxhash.Sum64String("k1")
	sketch.Add(keyh)
	sketch.Add(keyh)
	sketch.Add(keyh)
	sketch.Add(keyh)
	sketch.Add(keyh)
	keyh2 := xxhash.Sum64String("k1b")
	sketch.Add(keyh2)
	sketch.Add(keyh2)
	sketch.Add(keyh2)

	es1 := sketch.Estimate(keyh)
	es2 := sketch.Estimate(keyh2)
	additions := sketch.Additions
	sketch.reset()
	additionsNew := sketch.Additions
	es1h := sketch.Estimate(keyh)
	es2h := sketch.Estimate(keyh2)
	require.Equal(t, es1/2, es1h)
	require.Equal(t, es2/2, es2h)
	require.Equal(t, additions-(es1-es1h)-(es2-es2h), additionsNew)

}

func BenchmarkSketch(b *testing.B) {
	sketch := NewCountMinSketch()
	sketch.EnsureCapacity(50000000)
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
