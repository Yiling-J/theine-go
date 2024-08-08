package clock

import (
	"time"
)

type Clock struct {
	Start time.Time
}

func (c *Clock) NowNano() int64 {
	return time.Since(c.Start).Nanoseconds()
}

func (c *Clock) ExpireNano(ttl time.Duration) int64 {
	return c.NowNano() + ttl.Nanoseconds()
}

func (c *Clock) SetStart(ts int64) {
	c.Start = time.Unix(0, ts)
}
