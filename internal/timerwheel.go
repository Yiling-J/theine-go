package internal

import (
	"math/bits"
	"time"
)

func next2Power(x uint) uint {
	x--
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	x |= x >> 32
	x++
	return x
}

type Clock struct {
	start time.Time
}

func (c *Clock) nowNano() int64 {
	return time.Since(c.start).Nanoseconds()
}

func (c *Clock) expireNano(ttl time.Duration) int64 {
	return c.nowNano() + ttl.Nanoseconds()
}

type TimerWheel[K comparable, V any] struct {
	buckets  []uint
	spans    []uint
	shift    []uint
	wheel    [][]*List[K, V]
	clock    *Clock
	nanos    int64
	writebuf *LockedBuf[K, V]
}

func NewTimerWheel[K comparable, V any](size uint, writebuf *LockedBuf[K, V]) *TimerWheel[K, V] {
	clock := &Clock{start: time.Now()}
	buckets := []uint{64, 64, 32, 4, 1}
	spans := []uint{
		next2Power(uint((1 * time.Second).Nanoseconds())),
		next2Power(uint((1 * time.Minute).Nanoseconds())),
		next2Power(uint((1 * time.Hour).Nanoseconds())),
		next2Power(uint((24 * time.Hour).Nanoseconds())),
		next2Power(uint((24 * time.Hour).Nanoseconds())) * 4,
		next2Power(uint((24 * time.Hour).Nanoseconds())) * 4,
	}

	shift := []uint{
		uint(bits.TrailingZeros(spans[0])),
		uint(bits.TrailingZeros(spans[1])),
		uint(bits.TrailingZeros(spans[2])),
		uint(bits.TrailingZeros(spans[3])),
		uint(bits.TrailingZeros(spans[4])),
	}

	wheel := [][]*List[K, V]{}
	for i := 0; i < 5; i++ {
		tmp := []*List[K, V]{}
		for j := 0; j < int(buckets[i]); j++ {
			tmp = append(tmp, NewList[K, V](size, WHEEL_LIST))
		}
		wheel = append(wheel, tmp)
	}

	return &TimerWheel[K, V]{
		buckets:  buckets,
		spans:    spans,
		shift:    shift,
		wheel:    wheel,
		nanos:    clock.nowNano(),
		clock:    clock,
		writebuf: writebuf,
	}

}

func (tw *TimerWheel[K, V]) findIndex(expire int64) (int, int) {
	duration := expire - tw.nanos
	for i := 0; i < 5; i++ {
		if duration < int64(tw.spans[i+1]) {
			ticks := expire >> int(tw.shift[i])
			slot := int(ticks) & (int(tw.buckets[i]) - 1)
			return i, slot
		}
	}
	return 4, 0
}

func (tw *TimerWheel[K, V]) deschedule(entry *Entry[K, V]) {
	if list := entry.list(WHEEL_LIST); list != nil {
		list.remove(entry)
	}
}

func (tw *TimerWheel[K, V]) schedule(entry *Entry[K, V]) {
	tw.deschedule(entry)
	x, y := tw.findIndex(entry.expire)
	tw.wheel[x][y].PushFront(entry)
}

func (tw *TimerWheel[K, V]) advance(now int64, drain func()) {
	if now == 0 {
		now = tw.clock.nowNano()
	}
	previous := tw.nanos
	tw.nanos = now

	for i := 0; i < 5; i++ {
		prevTicks := previous >> int64(tw.shift[i])
		currentTicks := tw.nanos >> int64(tw.shift[i])
		if currentTicks <= prevTicks {
			break
		}
		tw.expire(i, prevTicks, currentTicks-prevTicks, drain)
	}
}

func (tw *TimerWheel[K, V]) expire(index int, prevTicks int64, delta int64, drain func()) {
	mask := tw.buckets[index] - 1
	steps := tw.buckets[index]
	if delta < int64(steps) {
		steps = uint(delta)
	}
	start := prevTicks & int64(mask)
	end := start + int64(steps)
	for i := start; i < end; i++ {
		list := tw.wheel[index][i&int64(mask)]
		entry := list.Front()
		for entry != nil {
			next := entry.Next(WHEEL_LIST)
			if entry.expire <= tw.nanos {
				tw.deschedule(entry)
				full := tw.writebuf.Push(BufItem[K, V]{entry: entry, code: EXPIRED})
				if full {
					drain()
				}
			} else {
				tw.schedule(entry)
			}
			entry = next

		}
		list.Reset()
	}
}
