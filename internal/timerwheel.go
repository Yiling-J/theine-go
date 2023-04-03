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

type TimerWheel struct {
	buckets  []uint
	spans    []uint
	shift    []uint
	wheel    [][]*List
	clock    *Clock
	nanos    int64
	writebuf *Queue
}

func NewTimerWheel(size uint, writebuf *Queue) *TimerWheel {
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

	wheel := [][]*List{}
	for i := 0; i < 5; i++ {
		tmp := []*List{}
		for j := 0; j < int(buckets[i]); j++ {
			tmp = append(tmp, NewList(size, WHEEL_LIST))
		}
		wheel = append(wheel, tmp)
	}

	return &TimerWheel{
		buckets:  buckets,
		spans:    spans,
		shift:    shift,
		wheel:    wheel,
		nanos:    clock.nowNano(),
		clock:    clock,
		writebuf: writebuf,
	}

}

func (tw *TimerWheel) findIndex(expire int64) (int, int) {
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

func (tw *TimerWheel) deschedule(entry *Entry) {
	if list := entry.list(WHEEL_LIST); list != nil {
		list.remove(entry)
	}
}

func (tw *TimerWheel) schedule(entry *Entry) {
	tw.deschedule(entry)
	x, y := tw.findIndex(entry.expire)
	tw.wheel[x][y].PushFront(entry)
}

func (tw *TimerWheel) advance() {
	previous := tw.nanos
	tw.nanos = tw.clock.nowNano()

	for i := 0; i < 5; i++ {
		prevTicks := previous >> int64(tw.shift[i])
		currentTicks := tw.nanos >> int64(tw.shift[i])
		if currentTicks <= prevTicks {
			break
		}
		tw.expire(i, prevTicks, currentTicks-prevTicks)
	}
}

func (tw *TimerWheel) expire(index int, prevTicks int64, delta int64) {
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
				tw.writebuf.Push(&BufItem{entry: entry, code: EXPIRED})
			} else {
				tw.schedule(entry)
			}
			entry = next

		}
		list.Reset()
	}
}
