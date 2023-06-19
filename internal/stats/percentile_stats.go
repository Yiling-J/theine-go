package stats

import (
	"sync"
	"time"

	"github.com/influxdata/tdigest"
)

var windowSize = time.Second.Nanoseconds()

var kQuantiles = []float32{
	0,
	0.05,
	0.1,
	0.25,
	0.5,
	0.75,
	0.9,
	0.95,
	0.99,
	0.999,
	0.9999,
	0.99999,
	0.999999,
	1.0,
}

type PercentileStatsData struct {
	p0      float64
	p5      float64
	p10     float64
	p25     float64
	p50     float64
	p75     float64
	p90     float64
	p95     float64
	p99     float64
	p999    float64
	p9999   float64
	p99999  float64
	p999999 float64
	p100    float64
}

type DigestBuilder struct {
	bufferSize int
	buffer     []float64
	digest     *tdigest.TDigest
}

func NewDigestBuilder(bufferSize int) *DigestBuilder {
	return &DigestBuilder{
		bufferSize: bufferSize,
		buffer:     make([]float64, 0, bufferSize),
		digest:     tdigest.New(),
	}
}

func (b *DigestBuilder) add(v float64) {
	b.buffer = append(b.buffer, v)
	if len(b.buffer) == b.bufferSize {
		for _, v := range b.buffer {
			b.digest.Add(v, 1)
		}
		b.buffer = b.buffer[:0]
	}
}

func (b *DigestBuilder) build() {
	for _, v := range b.buffer {
		b.digest.Add(v, 1)
	}
	b.buffer = b.buffer[:0]
}

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

type SlidingWindow struct {
	digests []tdigest.TDigest
	curHead int
}

func NewSlidingWindow(size int) *SlidingWindow {
	s := &SlidingWindow{curHead: 0, digests: make([]tdigest.TDigest, 0)}
	for i := 0; i < size; i++ {
		s.digests = append(s.digests, *tdigest.New())
	}
	return s
}

func (sw *SlidingWindow) slide(n int) {
	count := minInt(n, len(sw.digests))
	for i := 0; i < count; i++ {
		if sw.curHead == 0 {
			sw.curHead = len(sw.digests) - 1
		} else {
			sw.curHead -= 1
		}
		sw.digests[sw.curHead].Reset()
	}
}

func (sw *SlidingWindow) set(d *tdigest.TDigest) {
	sw.digests[sw.curHead].Merge(d)
}

func (sw *SlidingWindow) get() []tdigest.TDigest {
	data := make([]tdigest.TDigest, len(sw.digests))
	copy(data, sw.digests)
	return data
}

func (sw *SlidingWindow) newDigest(d *tdigest.TDigest, newExpiry uint64, oldExpiry uint64) {
	// set current head to new digest first
	// if newExpiry is much longer, for example 100 seconds, the new digest also expired
	sw.set(d)
	if newExpiry > oldExpiry {
		diff := newExpiry - oldExpiry
		sw.slide(int(diff / uint64(windowSize)))
	}
}

type PercentileStats struct {
	expiry        uint64
	slidingWindow *SlidingWindow
	builder       *DigestBuilder
	mu            sync.Mutex
}

func NewPercentileStats(now uint64) *PercentileStats {
	return &PercentileStats{
		expiry:        now + uint64(windowSize),
		slidingWindow: NewSlidingWindow(60),
		builder:       NewDigestBuilder(1000),
	}
}

func (ps *PercentileStats) update(now uint64, force bool) {
	if now > ps.expiry || force {
		ps.builder.build()
		ps.slidingWindow.newDigest(ps.builder.digest, now, ps.expiry)
		// reset builder
		ps.builder.buffer = ps.builder.buffer[:0]
		ps.builder.digest.Reset()
		ps.expiry = now + uint64(windowSize)
	}
}

func (ps *PercentileStats) Add(v float64, now uint64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.update(now, false)
	ps.builder.add(v)

}

func (ps *PercentileStats) estimate(now uint64) *tdigest.TDigest {
	final := tdigest.New()
	ps.mu.Lock()
	ps.update(now, true)
	all := ps.slidingWindow.get()
	// all will copy all existing digests first, so safe to unlock now
	ps.mu.Unlock()
	for _, d := range all {
		final.Merge(&d)
	}
	return final
}
