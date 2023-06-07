package serializers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemorySerializer(t *testing.T) {
	s := NewMemorySerializer[int]()
	b, err := s.Marshal(12)
	require.Nil(t, err)
	var q int
	err = s.Unmarshal(b, &q)
	require.Nil(t, err)
	require.Equal(t, 12, q)

	s1 := NewMemorySerializer[string]()
	b, err = s1.Marshal("foo")
	require.Nil(t, err)
	var q1 string
	err = s1.Unmarshal(b, &q1)
	require.Nil(t, err)
	require.Equal(t, "foo", q1)

	s3 := NewMemorySerializer[[]byte]()
	b, err = s3.Marshal([]byte{1, 3, 4, 1})
	require.Nil(t, err)
	var q3 []byte
	err = s3.Unmarshal(b, &q3)
	require.Nil(t, err)
	require.Equal(t, []byte{1, 3, 4, 1}, q3)
}
