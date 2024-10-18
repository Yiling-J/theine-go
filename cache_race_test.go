package theine

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go/internal"
	"github.com/stretchr/testify/require"
)

func keyGen() []uint64 {
	keys := []uint64{}
	r := rand.New(rand.NewSource(0))
	z := rand.NewZipf(r, 1.01, 9.0, 200000)
	for i := 0; i < 2<<16; i++ {
		keys = append(keys, z.Uint64())
	}
	return keys
}

func getSet(t *testing.T, entrypool bool) {
	for _, size := range []int{500, 2000, 10000, 50000} {
		builder := NewBuilder[uint64, uint64](int64(size))
		if entrypool {
			builder.UseEntryPool(true)
		}

		builder.RemovalListener(func(key, value uint64, reason RemoveReason) {})
		client, err := builder.Build()
		require.Nil(t, err)
		var wg sync.WaitGroup
		keys := keyGen()

		for i := 1; i <= 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rd := rand.Intn(2 << 16)
				for i := 0; i < 100000; i++ {
					keyGet := keys[(i+rd)&(2<<16-1)]
					keyUpdate := keys[(i+3*rd)&(2<<16-1)]

					v, ok := client.Get(keyGet)
					if ok && v != keyGet {
						panic(keyGet)
					}
					if !ok {
						client.SetWithTTL(keyGet, keyGet, 1, 0)
					}

					client.SetWithTTL(keyUpdate, keyUpdate, int64(i%10+1), 0)
				}
			}()
		}
		wg.Wait()
		client.store.Wait()

		require.True(
			t, client.Len() < size+internal.WriteBufferSize,
		)

		di := client.store.DebugInfo()

		require.Equal(t, client.Len(), int(di.TotalCount()))
		require.True(t, di.TotalWeight() <= int64(size+size/10))
		require.Equal(t, di.ProbationWeight, di.ProbationWeightField)
		require.Equal(t, di.ProtectedWeight, di.ProtectedWeightField)

		for i := 0; i < len(di.QueueWeight); i++ {
			require.Equal(t, di.QueueWeight[i], di.QueueWeightField[i])
		}

		client.store.RangeEntry(func(entry *internal.Entry[uint64, uint64]) {
			require.Equal(t, entry.Weight(), entry.PolicyWeight(), entry.Position())
		})

		client.Close()
	}
}

func TestCacheRace_EntryPool_GetSet(t *testing.T) {
	getSet(t, true)

}
func TestCacheRace_NoPool_GetSet(t *testing.T) {
	getSet(t, false)

}

func getSetDeleteExpire(t *testing.T, entrypool bool) {
	for _, size := range []int{500, 2000, 10000, 50000} {
		builder := NewBuilder[uint64, uint64](int64(size))
		if entrypool {
			builder.UseEntryPool(true)
		}
		builder.RemovalListener(func(key, value uint64, reason RemoveReason) {})
		client, err := builder.Build()
		require.Nil(t, err)
		var wg sync.WaitGroup
		keys := keyGen()

		for i := 1; i <= 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rd := rand.Intn(2 << 16)
				for i := 0; i < 100000; i++ {
					key := keys[(i+rd)&(2<<16-1)]
					v, ok := client.Get(key)
					if ok && v != key {
						panic(key)
					}
					if !ok {
						client.SetWithTTL(key, key, int64(i%10+1), time.Second*time.Duration(i%25+5))
					}
					if i%5 == 0 {
						client.Delete(key)
					}
					if i%5000 == 0 {
						client.Range(func(key, value uint64) bool {
							return true
						})
					}
				}
			}()
		}
		wg.Wait()

		client.store.Wait()

		require.True(
			t, client.Len() < size+internal.WriteBufferSize,
		)

		di := client.store.DebugInfo()

		require.Equal(t, client.Len(), int(di.TotalCount()))
		require.True(t, di.TotalWeight() <= int64(size+size/10))
		require.Equal(t, di.ProbationWeight, di.ProbationWeightField)
		require.Equal(t, di.ProtectedWeight, di.ProtectedWeightField)

		for i := 0; i < len(di.QueueWeight); i++ {
			require.Equal(t, di.QueueWeight[i], di.QueueWeightField[i])
		}

		client.store.RangeEntry(func(entry *internal.Entry[uint64, uint64]) {
			require.Equal(t, entry.Weight(), entry.PolicyWeight(), entry.Position())
		})

		client.Close()

	}
}

func TestCacheRace_EntryPool_GetSetDeleteExpire(t *testing.T) {
	getSetDeleteExpire(t, true)
}

func TestCacheRace_NoPool_GetSetDeleteExpire(t *testing.T) {
	getSetDeleteExpire(t, false)
}
