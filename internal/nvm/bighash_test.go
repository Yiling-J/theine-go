package nvm

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go/internal/alloc"
	"github.com/Yiling-J/theine-go/internal/nvm/directio"
	"github.com/Yiling-J/theine-go/internal/nvm/preallocate"
	"github.com/stretchr/testify/require"
)

func TestBigHash(t *testing.T) {
	f, err := directio.OpenFile("bfoo", os.O_RDWR|os.O_CREATE, 0666)
	require.Nil(t, err)
	defer os.Remove("bfoo")
	err = f.Truncate(4096 * 50)
	require.Nil(t, err)
	err = preallocate.Preallocate(f, 4096*50, true)
	require.Nil(t, err)
	bh := NewBigHash(4096*50, 4096, 8, alloc.NewAllocator(4096, 16<<20, 4096))
	require.Equal(t, 64, int(bh.buckets[0].Bloomfilter.M))
	bh.file = f
	for i := 0; i < 100; i++ {
		key := []byte(strconv.Itoa(i))
		err := bh.Insert(key, key, 1, 0)
		require.Nil(t, err)
	}
	for i := 0; i < 100; i++ {
		key := []byte(strconv.Itoa(i))
		v, _, _, _, err := bh.Lookup(key)
		require.Nil(t, err)
		require.Equal(t, key, v.Data)
	}

	// test expire
	key := []byte(strconv.Itoa(500))
	err = bh.Insert(key, key, 1, bh.Clock.ExpireNano(10*time.Millisecond))
	require.Nil(t, err)
	time.Sleep(30 * time.Millisecond)
	_, _, _, ok, err := bh.Lookup(key)
	require.Nil(t, err)
	require.False(t, ok)
}
