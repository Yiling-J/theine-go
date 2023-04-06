package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTlfu(t *testing.T) {
	hasher := NewHasher[string]()
	tlfu := NewTinyLfu[string, string](1000, hasher)
	require.Equal(t, uint(10), tlfu.lru.list.capacity)
	require.Equal(t, uint(990), tlfu.slru.probation.capacity)
	require.Equal(t, uint(792), tlfu.slru.protected.capacity)
	require.Equal(t, 0, tlfu.lru.list.len)
	require.Equal(t, 0, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)

	entries := []*Entry[string, string]{}
	for i := 0; i < 200; i++ {
		e := &Entry[string, string]{key: fmt.Sprintf("%d", i), cost: 1}
		evicted := tlfu.Set(e)
		entries = append(entries, e)
		require.Nil(t, evicted)
	}

	require.Equal(t, "199/198/197/196/195/194/193/192/191/190", tlfu.lru.list.display(LIST))
	require.Equal(t, "190/191/192/193/194/195/196/197/198/199", tlfu.lru.list.displayReverse(LIST))
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 190, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)

	// probation -> protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 189, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 189, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	for i := 200; i < 1000; i++ {
		e := &Entry[string, string]{key: fmt.Sprintf("%d", i), cost: 1}
		entries = append(entries, e)
		evicted := tlfu.Set(e)
		require.Nil(t, evicted)
	}
	// access protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 989, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	evicted := tlfu.Set(&Entry[string, string]{key: "0a", cost: 1})
	require.Equal(t, "990", evicted.key)
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 989, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	victim1 := tlfu.lru.list.Back()
	require.Equal(t, entries[991].key, victim1.key)
	victim := tlfu.slru.victim()
	require.Equal(t, "0", victim.key)
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	evicted = tlfu.Set(&Entry[string, string]{key: "1a", cost: 1})
	require.Equal(t, entries[992].key, evicted.key)
	require.Equal(t, 989, tlfu.slru.probation.len)

	entries2 := []*Entry[string, string]{}
	fmt.Println("zzzz")
	for i := 0; i < 1000; i++ {
		e := &Entry[string, string]{key: fmt.Sprintf("%d*", i), cost: 1}
		tlfu.Set(e)
		entries2 = append(entries2, e)
	}
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 989, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	require.Equal(t, "999*/998*/997*/996*/995*/994*/993*/992*/991*/990*", tlfu.lru.list.display(LIST))
	require.Equal(
		t, "990*/991*/992*/993*/994*/995*/996*/997*/998*/999*", tlfu.lru.list.displayReverse(LIST),
	)
	for _, i := range []int{997, 998, 999} {
		tlfu.Remove(entries2[i])
		tlfu.slru.probation.display(LIST)
		tlfu.slru.probation.displayReverse(LIST)
		tlfu.slru.protected.display(LIST)
		tlfu.slru.protected.displayReverse(LIST)
	}

}
