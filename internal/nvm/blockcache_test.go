package nvm

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/Yiling-J/theine-go/internal/alloc"
	"github.com/Yiling-J/theine-go/internal/nvm/preallocate"
	"github.com/ncw/directio"
	"github.com/stretchr/testify/require"
)

func TestBlockCacheSimple(t *testing.T) {
	bc, err := NewBlockCache(500<<10, 20<<10, 3, 0, alloc.NewAllocator(0, 20<<10, 4096), nil)
	require.Nil(t, err)

	f, err := os.OpenFile("bfoo", os.O_RDWR|os.O_CREATE, 0666)
	require.Nil(t, err)
	defer os.Remove("bfoo")
	err = f.Truncate(int64(bc.CacheSize))
	require.Nil(t, err)
	err = preallocate.Preallocate(f, int64(bc.CacheSize), true)
	require.Nil(t, err)
	bc.regionManager.file = f

	for i := 0; i < 10; i++ {
		key := []byte(strconv.Itoa(i))
		value := make([]byte, 10<<10+i)
		err := bc.Insert(key, value, 1, 0)
		require.Nil(t, err)
	}
	for i := 0; i < 10; i++ {
		key := []byte(strconv.Itoa(i))
		v, _, _, _, err := bc.Lookup(key)
		require.Nil(t, err)
		require.Equal(t, 10<<10+i, len(v.Data))
	}
}

type IntSerializer struct{}

func (s *IntSerializer) Marshal(i int) ([]byte, error) {
	buff := bytes.NewBuffer(make([]byte, 0))
	err := binary.Write(buff, binary.BigEndian, uint64(i))
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func (s *IntSerializer) Unmarshal(raw []byte, v *int) error {
	num := binary.BigEndian.Uint64(raw)
	*v = int(num)
	return nil
}

func TestBlockCacheParallel(t *testing.T) {
	bc, err := NewBlockCache(5000<<10, 50<<10, 3, 0, alloc.NewAllocator(0, 50<<10, 4096), nil)
	require.Nil(t, err)

	f, err := os.OpenFile("bfoo", os.O_RDWR|os.O_CREATE, 0666)
	require.Nil(t, err)
	err = f.Truncate(int64(bc.CacheSize))
	require.Nil(t, err)
	err = preallocate.Preallocate(f, int64(bc.CacheSize), true)
	require.Nil(t, err)
	f.Close()
	f, err = directio.OpenFile("bfoo", os.O_RDWR, 0666)
	require.Nil(t, err)
	defer os.Remove("bfoo")
	bc.regionManager.file = f

	var wg sync.WaitGroup
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := &IntSerializer{}
			for i := 0; i < 200; i++ {
				n := rand.Intn(200)
				key, err := s.Marshal(n)
				require.Nil(t, err)
				v, _, _, ok, err := bc.Lookup(key)
				if err != nil {
					panic(err)
				}
				if ok {
					expected, err := s.Marshal(n)
					require.Nil(t, err)
					require.Equal(t, expected, v.Data[:8])
				} else {
					base, err := s.Marshal(n)
					require.Nil(t, err)
					value := make([]byte, 10<<10)
					copy(value, base)
					err = bc.Insert(key, value, 1, 0)
					if err != nil {
						panic(err)
					}
				}

			}
		}()
	}
	wg.Wait()
}
