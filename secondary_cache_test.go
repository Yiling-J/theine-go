package theine_test

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/Yiling-J/theine-go/internal"
	"github.com/stretchr/testify/require"
)

func TestSecondaryCache_GetSetGetDeleteGet(t *testing.T) {
	secondary := internal.NewSimpleMapSecondary[int, int]()
	client, err := theine.NewBuilder[int, int](100).Hybrid(secondary).Workers(8).AdmProbability(1).Build()
	require.Nil(t, err)
	for i := 0; i < 1000; i++ {
		_, ok, _ := client.Get(i)
		require.False(t, ok)
		ok = client.Set(i, i, 1)
		require.True(t, ok)
		v, ok, _ := client.Get(i)
		require.True(t, ok)
		require.Equal(t, i, v)
		err = client.Delete(i)
		require.Nil(t, err)
		_, ok, _ = client.Get(i)
		require.False(t, ok)
	}
}

func TestSecondaryCache_AdmProb(t *testing.T) {
	secondary := internal.NewSimpleMapSecondary[int, int]()
	client, err := theine.NewBuilder[int, int](100).Hybrid(secondary).Workers(8).AdmProbability(0.5).Build()
	require.Nil(t, err)
	for i := 0; i < 1000; i++ {
		success := client.Set(i, i, 1)
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

func TestSecondaryCache_ErrorHandler(t *testing.T) {
	secondary := internal.NewSimpleMapSecondary[int, int]()
	secondary.ErrMode = true
	client, err := theine.NewBuilder[int, int](100).Hybrid(secondary).Workers(8).AdmProbability(1).Build()
	require.Nil(t, err)

	for i := 0; i < 1000; i++ {
		success := client.Set(i, i, 1)
		require.Nil(t, err)
		require.True(t, success)
	}

	require.True(t, secondary.ErrCounter.Load() > 0)

}

func TestSecondaryCache_GetSetNoRace(t *testing.T) {
	secondary := internal.NewSimpleMapSecondary[int, int]()
	client, err := theine.NewBuilder[int, int](100).Hybrid(secondary).Workers(8).AdmProbability(1).Build()
	require.Nil(t, err)
	var wg sync.WaitGroup
	for i := 1; i <= runtime.GOMAXPROCS(0)*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20000; i++ {
				key := i
				v, ok, err := client.Get(key)
				if err != nil {
					panic(err)
				}
				if !ok {
					if i%2 == 0 {
						_ = client.Set(key, i, 1)
					}
					if i%5 == 0 {
						err := client.Delete(key)
						if err != nil {
							panic(err)
						}
					}
				} else {
					if i != v {
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

func TestSecondaryCache_LoadingCache(t *testing.T) {
	secondary := internal.NewSimpleMapSecondary[int, int]()
	client, err := theine.NewBuilder[int, int](100).Hybrid(secondary).Workers(8).AdmProbability(1).
		Loading(func(ctx context.Context, key int) (theine.Loaded[int], error) {
			return theine.Loaded[int]{Value: key, Cost: 1, TTL: 0}, nil
		}).Build()
	require.Nil(t, err)

	for i := 0; i < 1000; i++ {
		value, err := client.Get(context.TODO(), i)
		require.Nil(t, err)
		require.Equal(t, i, value)
	}

	for i := 0; i < 1000; i++ {
		value, err := client.Get(context.TODO(), i)
		require.Nil(t, err)
		require.Equal(t, i, value)
	}

	success := client.Set(999, 999, 1)
	require.True(t, success)
	_, err = client.Get(context.TODO(), 999)
	require.Nil(t, err)
	err = client.Delete(999)
	require.Nil(t, err)
	_, err = client.Get(context.TODO(), 999)
	require.Nil(t, err)
	success = client.SetWithTTL(999, 999, 1, 5*time.Second)
	require.True(t, success)

}
