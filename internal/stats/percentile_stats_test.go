package stats

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go/internal/clock"
	"github.com/stretchr/testify/require"
)

func TestPercentilsStatsSimple(t *testing.T) {
	clock := clock.Clock{Start: time.Now().UTC()}
	ps := NewPercentileStats(uint64(clock.NowNano()))
	for i := 1; i <= 100; i++ {
		ps.Add(float64(i), uint64(clock.NowNano()))
	}

	d := ps.estimate(uint64(clock.NowNano()))
	data := []float64{}
	for _, r := range []float64{0.001, 0.01, 0.5, 0.99, 0.999} {
		data = append(data, d.Quantile(r))
	}
	require.Equal(t, float64(1), data[0])
	require.Equal(t, 2-0.5, data[1])
	require.Equal(t, 50.5, data[2])
	require.Equal(t, 100-0.5, data[3])
	require.Equal(t, float64(100), data[4])
}

func TestPercentilsStatsLarge(t *testing.T) {
	clock := clock.Clock{Start: time.Now().UTC()}
	ps := NewPercentileStats(uint64(clock.NowNano()))
	for i := 1; i <= 1e6; i++ {
		ps.Add(float64(i), uint64(clock.NowNano()))
	}

	d := ps.estimate(uint64(clock.NowNano()))
	data := []float64{}
	for _, r := range []float64{0.001, 0.01, 0.5, 0.99, 0.999} {
		data = append(data, d.Quantile(r))
	}
	require.Equal(t, 1000.5, data[0])
	require.Equal(t, 10000.5, data[1])
	require.Equal(t, 500000.5, data[2])
	require.Equal(t, 990000.5, data[3])
	require.Equal(t, 999000.5, data[4])
}

func TestPercentilsStatsHalfSlide(t *testing.T) {
	clock := clock.Clock{Start: time.Now().UTC()}
	ps := NewPercentileStats(uint64(clock.NowNano()))
	for i := 101; i <= 200; i++ {
		ps.Add(float64(i), uint64(clock.NowNano()))
	}

	for i := 1; i <= 100; i++ {
		ps.Add(float64(i), uint64(clock.ExpireNano(30*time.Second)))
	}

	d := ps.estimate(uint64(clock.ExpireNano(30 * time.Second)))
	data := []float64{}
	for _, r := range []float64{0.001, 0.01, 0.5, 0.99, 0.999} {
		data = append(data, d.Quantile(r))
	}
	require.Equal(t, float64(1), data[0])
	require.Equal(t, 2.5, data[1])
	require.Equal(t, 100.5, data[2])
	require.Equal(t, 198.5, data[3])
	require.Equal(t, float64(200), data[4])
}

func TestPercentilsStatsFullSlide(t *testing.T) {
	clock := clock.Clock{Start: time.Now().UTC()}
	ps := NewPercentileStats(uint64(clock.NowNano()))
	for i := 101; i <= 200; i++ {
		ps.Add(float64(i), uint64(clock.NowNano()))
	}

	// previous data should be cleared
	for i := 1; i <= 100; i++ {
		ps.Add(float64(i), uint64(clock.ExpireNano(65*time.Second)))
	}

	d := ps.estimate(uint64(clock.ExpireNano(65 * time.Second)))
	data := []float64{}
	for _, r := range []float64{0.001, 0.01, 0.5, 0.99, 0.999} {
		data = append(data, d.Quantile(r))
	}
	require.Equal(t, float64(1), data[0])
	require.Equal(t, 2-0.5, data[1])
	require.Equal(t, 50.5, data[2])
	require.Equal(t, 100-0.5, data[3])
	require.Equal(t, float64(100), data[4])
}

func TestPercentilsStatsParallel(t *testing.T) {
	clock := clock.Clock{Start: time.Now().UTC()}
	ps := NewPercentileStats(uint64(clock.NowNano()))
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		start := 1e6 / 10 * i
		go func(start int) {
			defer wg.Done()
			for i := start + 1; i <= start+1e5; i++ {
				ps.Add(float64(i), uint64(clock.NowNano()))
			}
		}(start)
	}
	wg.Wait()

	d := ps.estimate(uint64(clock.NowNano()))
	data := []float64{}
	for _, r := range []float64{0.001, 0.01, 0.5, 0.99, 0.999} {
		data = append(data, d.Quantile(r))
	}

	require.True(t, math.Abs(data[0]-1000.5)/1000.5 < 0.05)
	require.True(t, math.Abs(data[1]-10000.5)/10000.5 < 0.05)
	require.True(t, math.Abs(data[2]-500000.5)/500000.5 < 0.05)
	require.True(t, math.Abs(data[3]-990000.5)/990000.5 < 0.05)
	require.True(t, math.Abs(data[4]-999000.5)/999000.5 < 0.05)
}
