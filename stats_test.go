package theine_test

import (
	"testing"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	client, err := theine.NewBuilder[int, int](1000).Build()
	require.Nil(t, err)
	st := client.Stats()
	require.Equal(t, uint64(0), st.Hits())
	require.Equal(t, uint64(0), st.Misses())
	require.Equal(t, float64(0), st.HitRatio())

	client.Set(1, 1, 1)
	for i := 0; i < 2000; i++ {
		_, ok := client.Get(1)
		require.True(t, ok)
	}

	st = client.Stats()
	require.Equal(t, uint64(2000), st.Hits())
	require.Equal(t, uint64(0), st.Misses())
	require.Equal(t, float64(1), st.HitRatio())

	for i := 0; i < 10000; i++ {
		_, ok := client.Get(1)
		require.True(t, ok)
	}

	for i := 0; i < 10000; i++ {
		_, ok := client.Get(2)
		require.False(t, ok)
	}

	st = client.Stats()
	require.Equal(t, uint64(12000), st.Hits())
	require.Equal(t, uint64(10000), st.Misses())
	require.Equal(t, float64(12000)/float64(12000+10000), st.HitRatio())

}
