package theine_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

type ByteSerializer struct{}

func (s *ByteSerializer) Marshal(i []byte) ([]byte, error) {
	return i, nil
}

func (s *ByteSerializer) Unmarshal(raw []byte, v *[]byte) error {
	*v = make([]byte, len(raw))
	copy(*v, raw)
	return nil
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

func TestHybridCacheBighashOnly(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(100).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).Build()
	require.Nil(t, err)
	for i := 0; i < 1000; i++ {
		success := client.Set(i, []byte(strconv.Itoa(i)), 1)
		require.True(t, success)
	}
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 1000; i++ {
		value, success, err := client.Get(i)
		require.Nil(t, err)
		require.True(t, success)
		require.Equal(t, strconv.Itoa(i), string(value))
	}
}

func TestHybridCacheBlockCacheOnly(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(0).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).Build()
	require.Nil(t, err)
	s := &IntSerializer{}
	for i := 0; i < 1000; i++ {
		base, err := s.Marshal(i)
		require.Nil(t, err)
		value := make([]byte, 40<<10)
		copy(value, base)
		success := client.Set(i, value, 1)
		require.True(t, success)
	}
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 1000; i++ {
		value, success, err := client.Get(i)
		require.Nil(t, err)
		require.True(t, success)
		expected, err := s.Marshal(i)
		require.Nil(t, err)
		require.Equal(t, expected, value[:8])
	}
}

func TestHybridCacheGetSetBlockCacheOnly(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 40<<20).RegionSize(4 << 20).BigHashPct(0).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).Build()
	require.Nil(t, err)
	s := &IntSerializer{}
	var wg sync.WaitGroup
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := rand.Intn(100)
			for i := q; i < 3000+q; i++ {
				base, err := s.Marshal(i)
				require.Nil(t, err)
				value, success, err := client.Get(i)
				require.Nil(t, err)
				if !success {
					value := make([]byte, 40<<10)
					copy(value, base)
					success := client.Set(i, value, 1)
					require.True(t, success)
				} else {
					expected, err := s.Marshal(i)
					require.Nil(t, err)
					require.Equal(t, expected, value[:8])
				}
			}
		}()
	}
	wg.Wait()
}

func TestHybridCacheGetSetGetDeleteGet(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(30).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).AdmProbability(1).Build()
	require.Nil(t, err)
	s := &IntSerializer{}
	for i := 0; i < 1000; i++ {
		var value []byte
		base, err := s.Marshal(i)
		require.Nil(t, err)
		if i < 600 {
			value = base
		} else {
			value = make([]byte, 4200)
			copy(value, base)
		}
		_, ok, _ := client.Get(i)
		require.False(t, ok)
		ok = client.Set(i, value, 1)
		require.True(t, ok)
		v, ok, _ := client.Get(i)
		require.True(t, ok)
		require.Equal(t, value, v)
		err = client.Delete(i)
		require.Nil(t, err)
		_, ok, _ = client.Get(i)
		require.False(t, ok)
	}
}

func TestHybridCacheMix(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(30).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).AdmProbability(1).Build()
	require.Nil(t, err)
	s := &IntSerializer{}
	for i := 0; i < 1000; i++ {
		var value []byte
		base, err := s.Marshal(i)
		require.Nil(t, err)
		if i < 600 {
			value = base
		} else {
			value = make([]byte, 4200)
			copy(value, base)
		}
		success := client.Set(i, value, 1)
		require.Nil(t, err)
		require.True(t, success)
	}
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 1000; i++ {
		value, success, err := client.Get(i)
		require.Nil(t, err)
		require.True(t, success)
		expected, err := s.Marshal(i)
		require.Nil(t, err)
		require.Equal(t, expected, value[:8])
	}
}

func TestHybridCacheMixProb(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(30).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).AdmProbability(0.5).Build()
	require.Nil(t, err)
	s := &IntSerializer{}
	for i := 0; i < 1000; i++ {
		var value []byte
		base, err := s.Marshal(i)
		require.Nil(t, err)
		if i < 600 {
			value = base
		} else {
			value = make([]byte, 4200)
			copy(value, base)
		}
		success := client.Set(i, value, 1)
		require.Nil(t, err)
		require.True(t, success)
	}
	time.Sleep(50 * time.Millisecond)

	counter := 0
	for i := 0; i < 1000; i++ {
		_, success, err := client.Get(i)
		require.Nil(t, err)
		if success {
			counter += 1
		}
	}
	require.True(t, counter < 600)
}

func TestHybridCacheErrorHandler(t *testing.T) {
	var errCounter atomic.Uint32
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(100).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {
		errCounter.Add(1)
	}).Build()
	require.Nil(t, err)
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).Build()
	require.Nil(t, err)
	err = os.Truncate("afoo", 1)
	require.Nil(t, err)
	defer os.Remove("afoo")
	for i := 0; i < 1000; i++ {
		success := client.Set(i, []byte(strconv.Itoa(i)), 1)
		require.Nil(t, err)
		require.True(t, success)
	}
	require.True(t, errCounter.Load() > 0)

}

func TestHybridCacheGetSetNoRace(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 1000<<20).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).Build()
	require.Nil(t, err)
	var wg sync.WaitGroup
	for i := 1; i <= runtime.GOMAXPROCS(0)*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := &IntSerializer{}
			for i := 0; i < 20000; i++ {
				key := i
				v, ok, err := client.Get(key)
				if err != nil {
					panic(err)
				}
				if !ok {
					base, err := s.Marshal(key)
					if err != nil {
						panic(err)
					}
					if i%2 == 0 {
						value := make([]byte, 1<<10)
						copy(value, base)
						_ = client.Set(key, value, 1)
					} else {
						value := make([]byte, 120<<10)
						copy(value, base)
						_ = client.Set(key, value, 1)
					}
					if i%5 == 0 {
						err := client.Delete(key)
						if err != nil {
							panic(err)
						}
					}
				} else {
					expected, err := s.Marshal(key)
					if err != nil {
						panic(err)
					}
					if !bytes.Equal(v[:8], expected) {
						panic("value mismatch")
					}
				}
			}
		}()
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond)
	client.Close()
}

func TestHybridLoadingCache(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(30).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	s := &IntSerializer{}
	client, err := theine.NewBuilder[int, []byte](100).Hybrid(nvm).Workers(8).AdmProbability(1).
		Loading(func(ctx context.Context, key int) (theine.Loaded[[]byte], error) {
			var value []byte
			base, err := s.Marshal(key)
			require.Nil(t, err)
			if key < 600 {
				value = base
			} else {
				value = make([]byte, 4200)
				copy(value, base)
			}
			return theine.Loaded[[]byte]{Value: value, Cost: 1, TTL: 0}, nil
		}).Build()
	require.Nil(t, err)

	for i := 0; i < 1000; i++ {
		value, err := client.Get(context.TODO(), i)
		require.Nil(t, err)
		expected, err := s.Marshal(i)
		require.Nil(t, err)
		require.Equal(t, expected, value[:8])
	}

	for i := 0; i < 1000; i++ {
		value, err := client.Get(context.TODO(), i)
		require.Nil(t, err)
		expected, err := s.Marshal(i)
		require.Nil(t, err)
		require.Equal(t, expected, value[:8])
	}

	success := client.Set(999, []byte{1}, 1)
	require.True(t, success)
	_, err = client.Get(context.TODO(), 999)
	require.Nil(t, err)
	err = client.Delete(999)
	require.Nil(t, err)
	_, err = client.Get(context.TODO(), 999)
	require.Nil(t, err)
	success = client.SetWithTTL(999, []byte{}, 1, 5*time.Second)
	require.True(t, success)

}
