package bf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBloom(t *testing.T) {
	bf := NewWithSize(5)
	bf.EnsureCapacity(5)
	bf.EnsureCapacity(500)
	bf.EnsureCapacity(200)

	success := bf.Insert(123)
	require.True(t, success)

	exist := bf.Exist(123)
	require.True(t, exist)

	exist = bf.Exist(456)
	require.False(t, exist)
}
