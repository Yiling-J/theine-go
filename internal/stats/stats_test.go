package stats

import (
	"testing"
	"time"

	"github.com/Yiling-J/theine-go/internal/clock"
	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	clock := &clock.Clock{Start: time.Now().UTC()}
	stats := NewStats(clock)
	require.True(t, len(stats.counterData) == 9)
	require.True(t, len(stats.percentileData) == 4)

	stats.Add(NumCacheGets, 1)
	stats.Add(NumItems, 1)
	stats.Add(NumNvmEvictions, 1)

	for i := 1; i <= 100; i++ {
		stats.Add(LoadingCacheLatency, uint64(i))
	}

	data := stats.Collect()
	require.Equal(t, uint64(1), data.NumCacheGets)
	require.Equal(t, uint64(1), data.NumItems)
	require.Equal(t, uint64(1), data.NumNvmEvictions)

	require.Equal(t, 50.5, data.LoadingCacheLatency.P50)
	require.Equal(t, 90.5, data.LoadingCacheLatency.P90)
	require.Equal(t, 99.5, data.LoadingCacheLatency.P99)
	require.Equal(t, 100.0, data.LoadingCacheLatency.P100)
}
