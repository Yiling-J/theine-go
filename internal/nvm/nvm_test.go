package nvm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNvmSetup(t *testing.T) {
	// block size is 16k
	// bucket size will align to 16k
	// bighash raw size is 25357910, align to 25346048
	// block cache raw size is 101431640, align to 101416960
	store, err := NewNvmStore[int, int](
		"bfoo", 16<<10, 126789550, 4<<10, 4<<20, 3, 20, 500, 8, func(err error) {},
		nil, nil,
	)
	require.Nil(t, err)
	defer os.Remove("bfoo")
	require.Equal(t, 16<<10, int(store.bighash.BucketSize))
	require.Equal(t, 25346048, int(store.bighash.CacheSize))
	require.Equal(t, 1547, int(store.bighash.numBuckets))

	require.Equal(t, 101416960, int(store.blockcache.CacheSize))
	require.Equal(t, 4<<20, int(store.blockcache.RegionSize))
	require.Equal(t, 24, int(store.blockcache.regionManager.regionCount))
	require.Equal(t, 500, store.bigHashMaxEntrySize)
	require.Equal(t, 64, int(store.bighash.buckets[0].Bloomfilter.M))

	// no bighash
	store, err = NewNvmStore[int, int](
		"bfoo", 16<<10, 126789550, 4<<10, 4<<20, 3, 0, 0, 8, func(err error) {},
		nil, nil,
	)
	require.Nil(t, err)
	require.Nil(t, store.bighash)
	require.Equal(t, 126779392, int(store.blockcache.CacheSize))

	// no block cache
	store, err = NewNvmStore[int, int](
		"bfoo", 16<<10, 126789550, 4<<10, 4<<20, 3, 100, 0, 8, func(err error) {},
		nil, nil,
	)
	require.Nil(t, err)
	require.Nil(t, store.blockcache)
	require.Equal(t, 126779392, int(store.bighash.CacheSize))

	// tool large bighahsh max size
	_, err = NewNvmStore[int, int](
		"bfoo", 16<<10, 126789550, 4<<10, 4<<20, 3, 20, 10<<50, 8, func(err error) {},
		nil, nil,
	)
	require.NotNil(t, err)
}

type ByteSerializer struct{}

func (s *ByteSerializer) Marshal(i []byte) ([]byte, error) {
	return i, nil
}

func (s *ByteSerializer) Unmarshal(raw []byte, v *[]byte) error {
	*v = make([]byte, len(raw))
	copy(*v, raw)
	return nil
}

func TestNvmResize(t *testing.T) {
	defer os.Remove("bfoo")
	for _, size := range []int{30 << 20, 100 << 20, 50 << 20} {
		store, err := NewNvmStore[int, []byte](
			"bfoo", 512, size, 4<<10, 100<<10, 3, 20, 0, 8, func(err error) {
				require.Nil(t, err)
			},
			&IntSerializer{}, &ByteSerializer{},
		)
		require.Nil(t, err)

		// insert to soc
		for i := 0; i < 5000; i++ {
			err = store.Set(i, make([]byte, 1<<10), 1, 0)
			require.Nil(t, err)
		}
		// insert to loc
		for i := 0; i < 5000; i++ {
			err = store.Set(i, make([]byte, 20<<10), 1, 0)
			require.Nil(t, err)
		}

	}

}

type IntSerializerE struct{}

func (s *IntSerializerE) Marshal(i int) ([]byte, error) {
	buff := bytes.NewBuffer(make([]byte, 0))
	err := binary.Write(buff, binary.BigEndian, uint64(i))
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), errors.New("e")
}

func (s *IntSerializerE) Unmarshal(raw []byte, v *int) error {
	num := binary.BigEndian.Uint64(raw)
	*v = int(num)
	return errors.New("e")
}

func TestNvmSerializerError(t *testing.T) {
	defer os.Remove("bfoo")
	errCount := 0
	store, err := NewNvmStore[int, int](
		"bfoo", 512, 1000<<10, 4<<10, 5<<10, 3, 20, 0, 8, func(err error) {
			errCount += 1
		},
		&IntSerializerE{}, &IntSerializerE{},
	)
	require.Nil(t, err)

	err = store.Set(1, 1, 1, 0)
	require.NotNil(t, err)

	_, _, _, _, err = store.Get(1)
	require.NotNil(t, err)
}
