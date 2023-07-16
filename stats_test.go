package theine_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

func TestCacheStats(t *testing.T) {
	nvm, err := theine.NewNvmBuilder[int, []byte]("afoo", 150<<20).BigHashPct(30).KeySerializer(&IntSerializer{}).ValueSerializer(&ByteSerializer{}).ErrorHandler(func(err error) {}).Build()
	require.Nil(t, err)
	defer os.Remove("afoo")
	client, err := theine.NewBuilder[int, []byte](500).RecordStats().Hybrid(nvm).Workers(8).AdmProbability(1).Loading(
		func(ctx context.Context, key int) (theine.Loaded[[]byte], error) {
			time.Sleep(1 * time.Millisecond)
			var value []byte
			s := &IntSerializer{}
			base, err := s.Marshal(key)
			require.Nil(t, err)
			if key < 1000 {
				value = base
			} else {
				value = make([]byte, 4200)
				copy(value, base)
			}
			return theine.Loaded[[]byte]{
				Value: value,
				Cost:  1,
			}, nil
		},
	).Build()
	require.Nil(t, err)
	for i := 0; i < 2000; i++ {
		_, err = client.Get(context.TODO(), i)
		require.Nil(t, err)
	}
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 2000; i++ {
		_, err = client.Get(context.TODO(), i)
		require.Nil(t, err)
	}
	stats := client.CollectStats()
	require.True(t, stats.HitRatio() > 0)
	require.True(t, stats.NvmHitRatio() > 0)
	require.True(t, stats.LoadingCacheLatency.P90 > 0)
	require.True(t, stats.NvmInsertLatency.P90 > 0)
	require.True(t, stats.NvmLookupLatency.P90 > 0)
}
