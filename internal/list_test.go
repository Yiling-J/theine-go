package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestList_PushPop(t *testing.T) {
	l := NewList[string, string](5, LIST_PROBATION)
	require.Equal(t, uint(5), l.capacity)
	require.Equal(t, LIST_PROBATION, l.listType)
	for i := 0; i < 5; i++ {
		l.PushFront(NewEntry(fmt.Sprintf("%d", i), "", 1, 0))
	}
	require.Equal(t, 5, int(l.len.Load()))
	require.Equal(t, "4/3/2/1/0", l.display())
	require.Equal(t, "0/1/2/3/4", l.displayReverse())

	for i := 0; i < 5; i++ {
		entry := l.PopTail()
		require.Equal(t, fmt.Sprintf("%d", i), entry.key)
	}
	entry := l.PopTail()
	require.Nil(t, entry)

	entries := []*Entry[string, string]{}
	for i := 0; i < 5; i++ {
		new := NewEntry(fmt.Sprintf("%d", i), "", 1, 0)
		l.PushFront(new)
		entries = append(entries, new)
	}
	require.Equal(t, "4/3/2/1/0", l.display())
	l.MoveToBack(entries[2])
	require.Equal(t, "4/3/1/0/2", l.display())
	require.Equal(t, "2/0/1/3/4", l.displayReverse())
	l.MoveBefore(entries[1], entries[3])
	require.Equal(t, "4/1/3/0/2", l.display())
	require.Equal(t, "2/0/3/1/4", l.displayReverse())
	l.MoveAfter(entries[2], entries[4])
	require.Equal(t, "4/2/1/3/0", l.display())
	require.Equal(t, "0/3/1/2/4", l.displayReverse())
	l.Remove(entries[1])
	require.Equal(t, "4/2/3/0", l.display())
	require.Equal(t, "0/3/2/4", l.displayReverse())

}

func TestList_CountCost(t *testing.T) {
	l := NewList[string, string](100, LIST_PROBATION)
	require.Equal(t, uint(100), l.capacity)
	require.Equal(t, LIST_PROBATION, l.listType)
	for i := 0; i < 5; i++ {
		l.PushFront(NewEntry(fmt.Sprintf("%d", i), "", 20, 0))
	}
	require.Equal(t, 100, int(l.len.Load()))
	require.Equal(t, 5, l.count)
	for i := 0; i < 3; i++ {
		entry := l.PopTail()
		require.NotNil(t, entry)
	}
	require.Equal(t, 40, int(l.len.Load()))
	require.Equal(t, 2, l.count)
}

func TestList_WheelList(t *testing.T) {
	l := NewList[string, string](5, WHEEL_LIST)
	require.Equal(t, uint(5), l.capacity)
	require.Equal(t, WHEEL_LIST, l.listType)
	for i := 0; i < 5; i++ {
		l.PushFront(NewEntry(fmt.Sprintf("%d", i), "", 1, 0))
	}
	require.Equal(t, 5, int(l.len.Load()))
	require.Equal(t, "4/3/2/1/0", l.display())
	require.Equal(t, "0/1/2/3/4", l.displayReverse())

	for i := 0; i < 5; i++ {
		entry := l.PopTail()
		require.Equal(t, fmt.Sprintf("%d", i), entry.key)
	}
	entry := l.PopTail()
	require.Nil(t, entry)

}
