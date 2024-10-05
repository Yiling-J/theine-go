package clock

import (
	"sync/atomic"
	"time"
)

type Clock struct {
	Start time.Time
	now   atomic.Int64
}

func (c *Clock) NowNano() int64 {
	return time.Since(c.Start).Nanoseconds()
}

func (c *Clock) NowNanoCached() int64 {
	return c.now.Load()
}

func (c *Clock) RefreshNowCache() {
	c.now.Store(c.NowNano())
}

// used in test only
func (c *Clock) SetNowCache(n int64) {
	c.now.Store(n)
}

func (c *Clock) ExpireNano(ttl time.Duration) int64 {
	return c.NowNano() + ttl.Nanoseconds()
}

func (c *Clock) SetStart(ts int64) {
	c.Start = time.Unix(0, ts)
}
