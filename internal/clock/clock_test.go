package clock_test

import (
	"math"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go/internal/clock"
	"github.com/stretchr/testify/require"
)

func TestClock_NowNano(t *testing.T) {
	c := &clock.Clock{Start: time.Now()}
	start := c.NowNano()
	time.Sleep(5 * time.Millisecond)
	end := c.NowNano()

	require.Greater(t, end, start)
}

func TestClock_ExpireNano(t *testing.T) {
	c := &clock.Clock{Start: time.Now()}
	nano := c.NowNano()

	ttl := 1 * time.Second
	expireNano := c.ExpireNano(ttl)
	lower := nano + ttl.Nanoseconds()
	upper := c.NowNano() + ttl.Nanoseconds()
	require.Greater(t, expireNano, lower)
	require.Less(t, expireNano, upper)

	overflowTTL := time.Duration(math.MaxInt64)
	expireNano = c.ExpireNano(overflowTTL)
	require.Equal(t, int64(math.MaxInt64), expireNano)
}

func TestClock_RefreshNowCache(t *testing.T) {
	c := &clock.Clock{Start: time.Now()}
	now := c.NowNanoCached()
	time.Sleep(5 * time.Millisecond)
	require.Equal(t, now, c.NowNanoCached())

	c.RefreshNowCache()
	require.NotEqual(t, c.NowNanoCached(), now)
}

func TestClock_SetNowCache(t *testing.T) {
	c := &clock.Clock{}
	c.SetNowCache(123456789)
	require.Equal(t, int64(123456789), c.NowNanoCached())
}

func TestClock_SetStart(t *testing.T) {
	c := &clock.Clock{}
	ts := time.Now().UnixNano()
	c.SetStart(ts)
	require.Equal(t, ts, c.Start.UnixNano())
}
