package theine

import (
	"errors"

	"github.com/Yiling-J/theine-go/internal/nvm"
)

type NvmBuilder[K comparable, V any] struct {
	file            string
	cacheSize       int
	blockSize       int
	bucketSize      int
	regionSize      int
	cleanRegionSize int
	bhPct           int
	errorHandler    func(err error)
	keySerializer   Serializer[K]
	valueSerializer Serializer[V]
}

func NewNvmBuilder[K comparable, V any](file string, cacheSize int) *NvmBuilder[K, V] {
	return &NvmBuilder[K, V]{
		file:            file,
		cacheSize:       cacheSize,
		blockSize:       4096,
		regionSize:      16 << 20, // 16mb
		cleanRegionSize: 3,
		bucketSize:      4 << 10, // 4kb
		bhPct:           10,      // 10%
		errorHandler:    func(err error) {},
	}
}

func (b *NvmBuilder[K, V]) BlockSize(size int) *NvmBuilder[K, V] {
	b.blockSize = size
	return b
}

func (b *NvmBuilder[K, V]) RegionSize(size int) *NvmBuilder[K, V] {
	b.regionSize = size
	return b
}

func (b *NvmBuilder[K, V]) BucketSize(size int) *NvmBuilder[K, V] {
	b.bucketSize = size
	return b
}

func (b *NvmBuilder[K, V]) BigHashPct(pct int) *NvmBuilder[K, V] {
	b.bhPct = pct
	return b
}

func (b *NvmBuilder[K, V]) CleanRegionSize(size int) *NvmBuilder[K, V] {
	b.cleanRegionSize = size
	return b
}

func (b *NvmBuilder[K, V]) ErrorHandler(fn func(err error)) *NvmBuilder[K, V] {
	b.errorHandler = fn
	return b
}

func (b *NvmBuilder[K, V]) KeySerializer(s Serializer[K]) *NvmBuilder[K, V] {
	b.keySerializer = s
	return b
}

func (b *NvmBuilder[K, V]) ValueSerializer(s Serializer[V]) *NvmBuilder[K, V] {
	b.valueSerializer = s
	return b
}

func (b *NvmBuilder[K, V]) Build() (*nvm.NvmStore[K, V], error) {
	if b.keySerializer == nil || b.valueSerializer == nil {
		return nil, errors.New("missing serializer")
	}
	return nvm.NewNvmStore[K, V](
		b.file, b.blockSize, b.cacheSize, b.bucketSize,
		b.regionSize, b.cleanRegionSize, uint8(b.bhPct), b.errorHandler,
		b.keySerializer, b.valueSerializer,
	)
}
