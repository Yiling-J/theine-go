package internal

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type testEventType uint8

const (
	TestEventGet testEventType = iota
	TestEventSet
	TestEventUpdate
	TestEventFreq
	TestEventRemove
	TestEventResizeWindow
)

type testEvent struct {
	event testEventType
	key   int
	value int
}

type testCase struct {
	name     string
	events   []testEvent
	expected string
}

var weightTests = []testCase{
	{
		"window promote",
		[]testEvent{
			{TestEventGet, 13, 1},
		},
		"13/14/12/11/10:9/8/7/6/5/4/3/2/1/0:",
	},
	{
		"probation promote",
		[]testEvent{
			{TestEventGet, 7, 1},
		},
		"14/13/12/11/10:9/8/6/5/4/3/2/1/0:7",
	},
	{
		"protect promote",
		[]testEvent{
			{TestEventGet, 5, 1},
			{TestEventGet, 6, 1},
			{TestEventGet, 7, 1},
			{TestEventGet, 8, 1},
			{TestEventGet, 5, 1},
		},
		"14/13/12/11/10:9/4/3/2/1/0:5/8/7/6",
	},
	{
		"simple insert",
		[]testEvent{
			{TestEventSet, 15, 1},
		},
		// 10 is evicted because of frequency
		"15/14/13/12/11:9/8/7/6/5/4/3/2/1/0:",
	},
	{
		"simple insert, low freq",
		[]testEvent{
			{TestEventFreq, 0, 5},
			{TestEventSet, 15, 1},
		},
		// 10 is evicted because of frequency
		"15/14/13/12/11:9/8/7/6/5/4/3/2/1/0:",
	},
	{
		"simple insert, high freq",
		[]testEvent{
			{TestEventFreq, 10, 5},
			{TestEventSet, 15, 1},
		},
		// 0 is evicted because of frequency
		"15/14/13/12/11:10/9/8/7/6/5/4/3/2/1:",
	},
	{
		"simple insert, high weight",
		[]testEvent{
			{TestEventSet, 15, 3},
		},
		// after window evict:
		// 15/14/13:12/11/10/9/8/7/6/5/4/3/2/1/0:
		// compare 10-0, evict 10,
		// compare 11-1, evict 11,
		// compare 12-2, evict 12,
		// now 15/14/13/9/8/7/6/5/4/3/2/1/0:
		"15/14/13:9/8/7/6/5/4/3/2/1/0:",
	},
	{
		"simple insert, high weight, high freq",
		[]testEvent{
			{TestEventFreq, 10, 5},
			{TestEventFreq, 11, 5},
			{TestEventSet, 15, 3},
		},
		// after window evict:
		// 15/14/13:12/11/10/9/8/7/6/5/4/3/2/1/0:
		// compare 10-0, evict 0,
		// compare 11-1, evict 1,
		// compare 12-2, evict 12,
		// now 15/14/13/9/8/7/6/5/4/3/2/1/0:
		"15/14/13:11/10/9/8/7/6/5/4/3/2:",
	},
	{
		"simple insert, weight lt window",
		[]testEvent{
			{TestEventSet, 15, 8},
		},
		// after window evict:
		// :15/14/13/12/11/10/9/8/7/6/5/4/3/2/1/0:
		// evict all winodw entries
		":9/8/7/6/5/4/3/2/1/0:",
	},
	{
		"simple insert, weight lt window, high freq",
		[]testEvent{
			{TestEventFreq, 15, 5},
			{TestEventSet, 15, 8},
		},
		// 10-14 evicted,
		// 0 evicted, 15 remain
		// candidate is nil, keep evict victim 1
		// candidate is nil, keep evict victim 2
		// size fit
		":15/9/8/7/6/5/4/3:",
	},
	{
		"update weight",
		[]testEvent{
			{TestEventUpdate, 13, 7},
		},
		":9/8/7/6/5/4/3/2/1/0:",
	},
	{
		"update protected, delay",
		[]testEvent{
			{TestEventRemove, 14, 1},
			{TestEventRemove, 13, 1},
			{TestEventRemove, 12, 1},
			{TestEventRemove, 9, 1},
			{TestEventRemove, 8, 1},
			{TestEventRemove, 7, 1},
			{TestEventRemove, 6, 1},
			{TestEventGet, 4, 1},
			{TestEventUpdate, 4, 5},
		},
		// protected cap exceed, next resize will demote it
		"11/10:5/3/2/1/0:4",
	},
	{
		"update protected, demote ",
		[]testEvent{
			{TestEventRemove, 14, 1},
			{TestEventRemove, 13, 1},
			{TestEventRemove, 12, 1},
			{TestEventRemove, 9, 1},
			{TestEventRemove, 8, 1},
			{TestEventRemove, 7, 1},
			{TestEventRemove, 6, 1},
			{TestEventGet, 4, 1},
			{TestEventUpdate, 4, 5},
			{TestEventResizeWindow, 0, 0},
		},
		// protected cap exceed, demote
		"11/10:4/5/3/2/1/0:",
	},
	{
		"update protected, demote not run",
		[]testEvent{
			{TestEventRemove, 14, 1},
			{TestEventRemove, 13, 1},
			{TestEventRemove, 12, 1},
			{TestEventRemove, 9, 1},
			{TestEventRemove, 8, 1},
			{TestEventRemove, 7, 1},
			{TestEventRemove, 6, 1},
			{TestEventGet, 4, 1},
			{TestEventUpdate, 4, 5},
			{TestEventGet, 4, 1},
		},
		"11/10:5/3/2/1/0:4",
	},
	{
		"update protected, demote auto run",
		[]testEvent{
			{TestEventRemove, 14, 1},
			{TestEventRemove, 13, 1},
			{TestEventRemove, 12, 1},
			{TestEventRemove, 9, 1},
			{TestEventRemove, 8, 1},
			{TestEventRemove, 7, 1},
			{TestEventRemove, 6, 1},
			{TestEventGet, 4, 1},
			{TestEventUpdate, 4, 5},
			{TestEventSet, 12, 1},
		},
		"12/11/10:4/5/3/2/1/0:",
	},
	{
		"window too large",
		[]testEvent{
			{TestEventGet, 6, 7},
			{TestEventFreq, 14, 5},
			{TestEventUpdate, 14, 16},
		},
		// larger than cap will be evicted immediately
		"13/12/11/10:9/8/7/5/4/3/2/1/0:6",
	},
	{
		"probation too large",
		[]testEvent{
			{TestEventGet, 6, 7},
			{TestEventFreq, 7, 5},
			{TestEventUpdate, 7, 16},
		},
		// larger than cap will be evicted immediately
		"14/13/12/11/10:9/8/5/4/3/2/1/0:6",
	},
	{
		"protected too large",
		[]testEvent{
			{TestEventGet, 6, 7},
			{TestEventUpdate, 6, 16},
		},
		// larger than cap will be evicted immediately
		"14/13/12/11/10:9/8/7/5/4/3/2/1/0:",
	},
	{
		"window very large",
		[]testEvent{
			{TestEventGet, 6, 7},
			{TestEventFreq, 14, 5},
			{TestEventUpdate, 14, 13},
		},
		":14:6",
	},
	{
		"probation very large",
		[]testEvent{
			{TestEventGet, 6, 7},
			{TestEventFreq, 7, 5},
			{TestEventUpdate, 7, 13},
		},
		// larger than cap will be evicted immediately
		"::7/6",
	},
	{
		"protected very large",
		[]testEvent{
			{TestEventGet, 6, 7},
			{TestEventUpdate, 6, 14},
		},
		// key 6 is in protected and has eight larger than capacity,
		// so all probation will be evicted
		"::6",
	},
}

func newTinyLfuSized[K comparable, V any](wsize, msize, psize uint, hasher *Hasher[K]) *TinyLfu[K, V] {
	tlfu := &TinyLfu[K, V]{
		capacity: wsize + msize,
		slru: &Slru[K, V]{
			maxsize: msize,
			// probation list size is dynamic
			probation: NewList[K, V](0, LIST_PROBATION),
			protected: NewList[K, V](psize, LIST_PROTECTED),
		},
		sketch: NewCountMinSketch(),
		step:   -float32(wsize+msize) * 0.0625,
		hasher: hasher,
		misses: NewUnsignedCounter(),
		hits:   NewUnsignedCounter(),
		window: NewList[K, V](wsize, LIST_WINDOW),
	}

	return tlfu
}

func assertLen(t *testing.T, list *List[int, int]) {
	sum := 0
	for _, e := range list.entries() {
		sum += int(e.PolicyWeight())
	}
	require.Equal(t, list.Len(), sum)
}

func TestTlfu_Weight(t *testing.T) {
	hasher := NewHasher[int](nil)
	for _, cs := range weightTests {
		t.Run(cs.name, func(t *testing.T) {
			// window size 5, main size 10, protected size 5
			tlfu := newTinyLfuSized[int, int](5, 10, 5, hasher)
			tlfu.removeCallback = func(entry *Entry[int, int]) {}
			em := map[int]*Entry[int, int]{}

			// fill tlfu with 15 entries
			for i := 0; i < 15; i++ {
				entry := &Entry[int, int]{key: i, value: i, policyWeight: 1}
				em[i] = entry
				tlfu.Set(entry)
			}
			tlfu.EvictEntries()

			for _, event := range cs.events {
				switch event.event {
				case TestEventGet:
					for i := 0; i < event.value; i++ {
						entry := em[event.key]
						tlfu.Access(ReadBufItem[int, int]{
							entry: entry,
							hash:  tlfu.hasher.hash(event.key),
						})
					}
				case TestEventFreq:
					tlfu.sketch.Addn(hasher.hash(event.key), event.value)
				case TestEventSet:
					entry := &Entry[int, int]{
						key: event.key, value: event.key,
						policyWeight: int64(event.value),
					}
					tlfu.Set(entry)
				case TestEventUpdate:
					entry := em[event.key]
					entry.policyWeight += int64(event.value)
					tlfu.UpdateCost(entry, int64(event.value))
				case TestEventRemove:
					entry := em[event.key]
					tlfu.Remove(entry, true)
				case TestEventResizeWindow:
					tlfu.resizeWindow()
				}
			}

			assertLen(t, tlfu.window)
			assertLen(t, tlfu.slru.probation)
			assertLen(t, tlfu.slru.protected)
			require.Equal(t, int(tlfu.weightedSize), tlfu.window.Len()+tlfu.slru.len())

			result := strings.Join(
				[]string{
					tlfu.window.display(), tlfu.slru.probation.display(),
					tlfu.slru.protected.display()}, ":")

			require.Equal(t, cs.expected, result)

		})
	}

}

func groupNumbers(input []string) string {
	if len(input) == 0 {
		return ""
	}

	var result []string
	var currentGroup []string
	prev, _ := strconv.Atoi(input[0])
	currentGroup = append(currentGroup, input[0])

	for i := 1; i < len(input); i++ {
		num, _ := strconv.Atoi(input[i])
		if num == prev+1 || num == prev-1 {
			currentGroup = append(currentGroup, input[i])

		} else {
			result = append(result, fmt.Sprintf("%s-%s", currentGroup[0], currentGroup[len(currentGroup)-1]))
			currentGroup = []string{input[i]}

		}
		prev = num
	}

	// Append the last group
	result = append(result, fmt.Sprintf("%s-%s", currentGroup[0], currentGroup[len(currentGroup)-1]))

	return strings.Join(result, ">")
}

type adaptiveTestEvent struct {
	hrChanges []float32
	expected  string
}

var adaptiveTests = []adaptiveTestEvent{
	// init, default hr will be 0.2
	{[]float32{}, "149-100:99-80:79-0"},
	// same hr, repeat increase/decrease
	{[]float32{0.2}, "149-109:108-80:79-0"},
	// hr increase, decrease window
	{[]float32{0.4}, "149-109:108-80:79-0"},
	// hr decrease, increase window, decrease protected
	{[]float32{0.1}, "88-80>149-100:8-0>99-89:79-9"},
	// increase twice
	{[]float32{0.4, 0.6}, "149-118:117-80:79-0"},
	// decrease twice
	{[]float32{0.1, 0.08}, "88-80>149-109:108-100>8-0>99-89:79-9"},
	// increase decrease
	{[]float32{0.4, 0.2}, "88-80>149-109:108-89:79-0"},
	// decrease increase
	{[]float32{0.1, 0.2}, "97-80>149-100:17-0>99-98:79-18"},
}

func TestTlfu_Adaptive(t *testing.T) {
	hasher := NewHasher[int](nil)
	for _, cs := range adaptiveTests {
		t.Run(fmt.Sprintf("%v", cs.hrChanges), func(t *testing.T) {
			// window size 50, main size 100, protected size 80
			tlfu := newTinyLfuSized[int, int](50, 100, 80, hasher)
			tlfu.hr = 0.2
			tlfu.removeCallback = func(entry *Entry[int, int]) {}
			em := map[int]*Entry[int, int]{}

			for i := 0; i < 150; i++ {
				entry := &Entry[int, int]{key: i, value: i, policyWeight: 1}
				em[i] = entry
				tlfu.Set(entry)
			}
			tlfu.EvictEntries()

			for i := 0; i < 80; i++ {
				entry := em[i]
				tlfu.Access(ReadBufItem[int, int]{
					entry: entry,
					hash:  tlfu.hasher.hash(i),
				})
			}

			for _, hrc := range cs.hrChanges {
				newHits := int(hrc * 100)
				newMisses := 100 - newHits
				tlfu.hits.Add(uint64(newHits))
				tlfu.misses.Add(uint64(newMisses))
				tlfu.climb()
				tlfu.resizeWindow()
			}

			assertLen(t, tlfu.window)
			assertLen(t, tlfu.slru.probation)
			assertLen(t, tlfu.slru.protected)
			require.Equal(t, int(tlfu.weightedSize), tlfu.window.Len()+tlfu.slru.len())

			result, total := grouped(tlfu)
			require.Equal(t, 150, total)
			require.Equal(t, cs.expected, result)

		})
	}
}

func grouped(tlfu *TinyLfu[int, int]) (string, int) {
	total := 0
	l := strings.Split(tlfu.window.display(), "/")
	total += len(l)
	windowSeq := groupNumbers(l)

	l = strings.Split(tlfu.slru.probation.display(), "/")
	total += len(l)
	probationSeq := groupNumbers(l)
	l = strings.Split(tlfu.slru.protected.display(), "/")
	total += len(l)
	protectedSeq := groupNumbers(l)

	result := strings.Join(
		[]string{
			windowSeq, probationSeq,
			protectedSeq}, ":")
	return result, total
}

func TestTlfu_AdaptiveAmountRemain(t *testing.T) {
	hasher := NewHasher[int](nil)
	// window size 50, main size 100, protected size 80
	tlfu := newTinyLfuSized[int, int](50, 100, 80, hasher)
	tlfu.hr = 0.2
	tlfu.removeCallback = func(entry *Entry[int, int]) {}
	em := map[int]*Entry[int, int]{}

	for i := 0; i < 150; i++ {
		entry := &Entry[int, int]{key: i, value: i, policyWeight: 1}
		em[i] = entry
		tlfu.Set(entry)
	}
	tlfu.EvictEntries()

	for i := 0; i < 80; i++ {
		entry := em[i]
		tlfu.Access(ReadBufItem[int, int]{
			entry: entry,
			hash:  tlfu.hasher.hash(i),
		})
	}

	require.Equal(t, -9.375, float64(tlfu.step))

	// increase entry 100 weight to 4,
	entry := em[100]
	entry.policyWeight += int64(3)
	tlfu.window.len.Add(3)
	// increase entry 101 weight to 4,
	entry = em[101]
	entry.policyWeight += int64(3)
	tlfu.window.len.Add(3)
	// increase entry 102 weight to 3,
	entry = em[102]
	entry.policyWeight += int64(2)
	tlfu.window.len.Add(2)

	// the step is 9, so 100 and 101 will move but 102 can't
	newHits := int(0.2 * 100)
	newMisses := 100 - newHits
	tlfu.hits.Add(uint64(newHits))
	tlfu.misses.Add(uint64(newMisses))
	tlfu.climb()
	tlfu.resizeWindow()

	require.Equal(t, -1, tlfu.amount)
	require.Equal(t, 42, int(tlfu.window.capacity))
	require.Equal(t, 88, int(tlfu.slru.protected.capacity))

	result, total := grouped(tlfu)
	require.Equal(t, 150, total)
	require.Equal(t, "149-102:101-80:79-0", result)

	// manually add one entry, so window tail(101) changed
	entry = &Entry[int, int]{key: 998, value: 998, policyWeight: 1}
	em[998] = entry
	tlfu.Set(entry)

	result, total = grouped(tlfu)
	require.Equal(t, 150, total)
	require.Equal(t, "998-998>149-109:108-103>101-80:79-0", result)

	// apply remaining amount
	tlfu.resizeWindow()
	require.Equal(t, 0, tlfu.amount)
	require.Equal(t, 41, int(tlfu.window.capacity))
	require.Equal(t, 89, int(tlfu.slru.protected.capacity))
	result, total = grouped(tlfu)
	require.Equal(t, 150, total)
	require.Equal(t, "998-998>149-110:109-103>101-80:79-0", result)

}

func TestTlfu_SketchResize(t *testing.T) {
	hasher := NewHasher[int](nil)
	tlfu := NewTinyLfu[int, int](10000, hasher)

	for i := 0; i < 10000; i++ {
		tlfu.Set(&Entry[int, int]{key: i, value: i, policyWeight: 1})
		require.True(t, len(tlfu.sketch.Table) >= i, fmt.Sprintf("sketch size %d < %d", len(tlfu.sketch.Table), i))
	}

	size := len(tlfu.sketch.Table)
	require.Equal(t, 16384, size)

	for i := 10000; i < 20000; i++ {
		require.Equal(t, size, len(tlfu.sketch.Table))
	}
}
