package nvm

import (
	"errors"
	"os"
	"unsafe"

	"github.com/Yiling-J/theine-go/internal"
	"github.com/Yiling-J/theine-go/internal/alloc"
	"github.com/Yiling-J/theine-go/internal/clock"
	"github.com/Yiling-J/theine-go/internal/nvm/directio"
	"github.com/Yiling-J/theine-go/internal/nvm/preallocate"
)

type NvmStore[K comparable, V any] struct {
	file                   *os.File
	bighash                *BigHash
	blockcache             *BlockCache
	keySerializer          internal.Serializer[K]
	valueSerializer        internal.Serializer[V]
	errorHandler           func(err error)
	bigHashMaxEntrySize    int
	blockCacheMaxEntrySize int
}

const (
	BigHashMetaBlock uint8 = iota
	BigHashMetaBucketBlock
	BlockCacheMetaBlock
	BlockCacheMetaIndexBlock
	DataBlock
)

func alignDown(num int, alignment int) int {
	return num - num%alignment
}

func alignUp(num int, alignment int) int {
	return alignDown(num+alignment-1, alignment)
}

func NewNvmStore[K comparable, V any](
	file string, blockSize int, cacheSize int, bucketSize int, regionSize int,
	cleanRegionSize int, sizePct uint8, bigHashMaxEntrySize int, bfSize int, errorHandler func(err error),
	keySerializer internal.Serializer[K],
	valueSerializer internal.Serializer[V],
) (*NvmStore[K, V], error) {
	if sizePct > 100 {
		return nil, errors.New("sizePct larger than 100")
	}

	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	fs, err := f.Stat()
	if err != nil {
		return nil, err
	}
	allocSize := int64(cacheSize) - fs.Size()
	if allocSize < 0 {
		err = f.Truncate(int64(cacheSize))
		if err != nil {
			return nil, err
		}
	}
	if allocSize > 0 {
		err = preallocate.Preallocate(f, allocSize, false)
		if err != nil {
			return nil, err
		}
		err = f.Truncate(int64(cacheSize))
		if err != nil {
			return nil, err
		}
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}
	f, err = directio.OpenFile(file, os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	store := &NvmStore[K, V]{
		file: f, keySerializer: keySerializer, valueSerializer: valueSerializer,
		errorHandler: errorHandler,
	}
	if store.errorHandler == nil {
		store.errorHandler = func(err error) {}
	}
	bhSize := cacheSize * int(sizePct) / 100
	bcSize := cacheSize - bhSize
	bucketSize = alignUp(bucketSize, blockSize)
	regionSize = alignUp(regionSize, blockSize)
	allocator := alloc.NewAllocator(bucketSize, regionSize, blockSize)
	if bhSize > 0 {
		bhSize = alignDown(bhSize, blockSize)
		bh := NewBigHash(uint64(bhSize), uint64(bucketSize), uint32(bfSize), allocator)
		bh.file = f
		store.bighash = bh
	}
	if bcSize > 0 {
		bcSize = alignDown(bcSize, blockSize)
		bc := NewBlockCache(
			bcSize, regionSize, uint32(cleanRegionSize), uint64(bhSize), allocator, errorHandler,
		)
		bc.regionManager.file = f
		store.blockcache = bc
	}

	max := bucketSize - int(unsafe.Sizeof(BucketHeader{})) + int(unsafe.Sizeof(BucketEntry{}))
	if bigHashMaxEntrySize > 0 {
		if bigHashMaxEntrySize >= max {
			return nil, errors.New("bigHashMaxEntrySize too large")
		}
		store.bigHashMaxEntrySize = bigHashMaxEntrySize
	} else {
		store.bigHashMaxEntrySize = max
	}
	store.blockCacheMaxEntrySize = align(regionSize)
	return store, nil
}

func (n *NvmStore[K, V]) SetClock(clock *clock.Clock) {
	if n.bighash != nil {
		n.bighash.Clock = clock
	}
	if n.blockcache != nil {
		n.blockcache.Clock = clock
	}

}

func (n *NvmStore[K, V]) Get(key K) (value V, cost int64, expire int64, ok bool, err error) {
	kb, err := n.keySerializer.Marshal(key)
	if err != nil {
		return value, cost, expire, false, err
	}
	alloc, cost, expire, ok, err := n.get(kb)
	if alloc != nil {
		defer alloc.Deallocate()
	}
	if err != nil {
		return value, cost, expire, false, err
	}
	if !ok {
		return value, cost, expire, false, &internal.NotFound{}
	}
	err = n.valueSerializer.Unmarshal(alloc.Data, &value)
	if err != nil {
		return value, cost, expire, false, err
	}
	return value, cost, expire, true, nil
}

func (n *NvmStore[K, V]) get(key []byte) (item *alloc.AllocItem, cost int64, expire int64, ok bool, err error) {
	if n.bighash != nil {
		item, cost, expire, ok, err = n.bighash.Lookup(key)
		if err != nil || ok {
			return
		}
	}
	if n.blockcache != nil {
		return n.blockcache.Lookup(key)
	}
	return item, cost, expire, ok, err
}

func (n *NvmStore[K, V]) Set(key K, value V, cost int64, expire int64) error {
	kb, err := n.keySerializer.Marshal(key)
	if err != nil {
		return err
	}
	vb, err := n.valueSerializer.Marshal(value)
	if err != nil {
		return err
	}
	return n.set(kb, vb, cost, expire)
}

func (n *NvmStore[K, V]) set(key []byte, value []byte, cost int64, expire int64) error {
	if n.bighash != nil && len(key)+len(value) <= n.bigHashMaxEntrySize {
		return n.bighash.Insert(key, value, cost, expire)
	}
	if n.blockcache != nil && len(key)+len(value) <= n.blockCacheMaxEntrySize {
		return n.blockcache.Insert(key, value, cost, expire)
	}
	return nil
}

func (n *NvmStore[K, V]) Delete(key K) error {
	kb, err := n.keySerializer.Marshal(key)
	if err != nil {
		return err
	}
	return n.delete(kb)
}

func (n *NvmStore[K, V]) delete(key []byte) error {
	if n.bighash != nil {
		err := n.bighash.Delete(key)
		if err != nil {
			return err
		}
	}
	if n.blockcache != nil {
		n.blockcache.Delete(key)
		return nil
	}
	return nil
}

func (n *NvmStore[K, V]) HandleAsyncError(err error) {
	n.errorHandler(err)
}
