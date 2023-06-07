package nvm

import (
	"bytes"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/Yiling-J/theine-go/internal"
	"github.com/Yiling-J/theine-go/internal/alloc"
	"github.com/Yiling-J/theine-go/internal/bf"
	"github.com/Yiling-J/theine-go/internal/clock"
	"github.com/Yiling-J/theine-go/internal/nvm/serializers"
	"github.com/ncw/directio"
	"github.com/zeebo/xxh3"
)

type BucketHeader struct {
	checksum       uint64
	generationTime uint64
	capacity       uint64
	size           uint64
	endOffset      uint64
}

// func (h *BucketHeader) remainingCapacity() uint64 {
// 	return h.capacity - h.endOffset
// }

type BucketEntry struct {
	keySize   uint64
	valueSize uint64
	cost      int64
	expire    int64
	hash      uint64
}

type Bucket struct {
	mu          sync.RWMutex // used to lock bloomfilter
	Bloomfilter *bf.Bloomfilter
}

// used in persisit data
type BucketP struct {
	Index       uint64
	Bloomfilter *bf.Bloomfilter
}

type BigHash struct {
	CacheSize        uint64
	numBuckets       uint64
	headerSize       uint64
	entrySize        uint64
	BucketSize       uint64
	GenerationTime   uint64
	buckets          []*Bucket
	Clock            *clock.Clock
	headerSerializer serializers.Serializer[BucketHeader]
	entrySerializer  serializers.Serializer[BucketEntry]
	bufferPool       sync.Pool
	file             *os.File
	allocator        *alloc.Allocator
	bucketCache      *internal.Store[int, []byte]
}

func NewBigHash(cacheSize uint64, bucketSize uint64, allocator *alloc.Allocator) (*BigHash, error) {

	b := &BigHash{
		headerSize:       uint64(unsafe.Sizeof(BucketHeader{})),
		entrySize:        uint64(unsafe.Sizeof(BucketEntry{})),
		CacheSize:        uint64(cacheSize),
		BucketSize:       bucketSize,
		GenerationTime:   uint64(time.Now().UnixNano()),
		numBuckets:       uint64(cacheSize / bucketSize),
		buckets:          []*Bucket{},
		Clock:            &clock.Clock{Start: time.Now().UTC()},
		headerSerializer: serializers.NewMemorySerializer[BucketHeader](),
		entrySerializer:  serializers.NewMemorySerializer[BucketEntry](),
		bufferPool: sync.Pool{New: func() any {
			return bytes.NewBuffer(directio.AlignedBlock(int(bucketSize)))
		}},
		allocator:   allocator,
		bucketCache: internal.NewStore[int, []byte](int64(uint64(64<<20)/bucketSize), false, nil, nil, nil),
	}
	for i := 0; i < int(cacheSize/bucketSize); i++ {
		b.buckets = append(b.buckets, &Bucket{Bloomfilter: bf.NewWithSize(8)})
	}
	return b, nil
}

// load bucket data to bytes
func (h *BigHash) loadBucketData(index int) (*alloc.AllocItem, *BucketHeader, error) {
	bucketData, ok := h.bucketCache.Get(index)
	alloc := h.allocator.Allocate(int(h.BucketSize))
	if !ok {
		offset := index * int(h.BucketSize)
		_, err := h.file.ReadAt(alloc.Data, int64(offset))
		if err != nil {
			return alloc, nil, err
		}
	} else {
		_ = copy(alloc.Data, bucketData)
	}
	var header BucketHeader
	err := h.headerSerializer.Unmarshal(alloc.Data[:h.headerSize], &header)
	if err != nil {
		return alloc, nil, err
	}
	var checksumMatch bool
	if header.endOffset >= h.headerSize && header.endOffset <= h.BucketSize {
		checksum := xxh3.Hash(alloc.Data[h.headerSize:header.endOffset])
		checksumMatch = (checksum == header.checksum)
	}
	if !checksumMatch || header.generationTime != h.GenerationTime {
		header = BucketHeader{
			checksum:       xxh3.Hash(alloc.Data[h.headerSize:h.headerSize]),
			generationTime: h.GenerationTime,
			capacity:       h.BucketSize,
			size:           0,
			endOffset:      h.headerSize,
		}
		hb, err := h.headerSerializer.Marshal(header)
		if err != nil {
			return alloc, nil, err
		}
		_ = copy(alloc.Data[:h.headerSize], hb)

	}
	return alloc, &header, nil
}

func (h *BigHash) saveBucketData(index int, data []byte) error {
	_, err := h.file.WriteAt(data, int64(index*int(h.BucketSize)))
	if err != nil {
		return err
	}
	cached := make([]byte, len(data))
	_ = copy(cached, data)
	_ = h.bucketCache.Set(index, cached, 1, 0)
	return nil
}

// delete from bucket
func (h *BigHash) deleteFromBucket(keyh uint64, keyb []byte) error {
	index := keyh % h.numBuckets
	bucket := h.buckets[index]
	// hold lock until write back to file
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	alloc, header, err := h.loadBucketData(int(index))
	defer alloc.Deallocate()
	if err != nil {
		return err
	}
	bucketBytes := alloc.Data
	// init header
	if header.capacity == 0 {
		header.capacity = uint64(h.BucketSize)
		header.endOffset = h.headerSize
	}
	bucket.Bloomfilter.Reset()
	// init new buffer
	newAlloc := h.allocator.Allocate(int(h.BucketSize))
	defer newAlloc.Deallocate()
	block := newAlloc.Data
	offset := int(h.headerSize)

	now := h.Clock.NowNano()
	// write existing entries to new block, stop when no space left in block
	offsetOld := int(h.headerSize)
	count := 0
	for i := 0; i < int(header.size); i++ {
		// nead to check entry meta first and remove duplicate/expired entries
		var entry BucketEntry
		err := h.entrySerializer.Unmarshal(
			bucketBytes[offsetOld:offsetOld+int(h.entrySize)],
			&entry,
		)
		if err != nil {
			return err
		}
		total := entry.keySize + entry.valueSize + h.entrySize
		if entry.hash == keyh || (entry.expire > 0 && entry.expire <= now) {
			offsetOld += int(total)
			continue
		}
		// not enough space, stop loop
		if total > h.BucketSize-uint64(offset) {
			break
		}
		n := copy(block[offset:], bucketBytes[offsetOld:offsetOld+int(total)])
		offset += n
		offsetOld += n
		count += 1
		bucket.Bloomfilter.Insert(entry.hash)
	}
	header.endOffset = uint64(offset)
	header.checksum = xxh3.Hash(block[h.headerSize:header.endOffset])
	header.size = uint64(count)
	hb, err := h.headerSerializer.Marshal(*header)
	if err != nil {
		return err
	}
	// add header
	_ = copy(block[0:], hb)
	return h.saveBucketData(int(index), block)
}

// insert new key to bucket
func (h *BigHash) addToBucket(keyh uint64, keyb []byte, value []byte, cost int64, expire int64) error {
	index := keyh % h.numBuckets
	bucket := h.buckets[index]
	// hold lock until write back to file
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	alloc, header, err := h.loadBucketData(int(index))
	defer alloc.Deallocate()
	if err != nil {
		return err
	}
	bucketBytes := alloc.Data
	// init header
	if header.capacity == 0 {
		header.capacity = uint64(h.BucketSize)
		header.endOffset = h.headerSize
	}
	bucket.Bloomfilter.Reset()
	if int(header.size)-bucket.Bloomfilter.Capacity/10 > 5 {
		bucket.Bloomfilter.EnsureCapacity(int(10 * header.size))
	}
	bucket.Bloomfilter.EnsureCapacity(int(header.size) * 10)
	// init new buffer
	allocNew := h.allocator.Allocate(int(h.BucketSize))
	defer allocNew.Deallocate()
	block := allocNew.Data
	offset := int(h.headerSize)
	// write new entry first
	bn, err := h.entrySerializer.Marshal(BucketEntry{
		keySize:   uint64(len(keyb)),
		valueSize: uint64(len(value)),
		cost:      cost,
		expire:    expire,
		hash:      keyh,
	})
	if err != nil {
		return err
	}
	n := copy(block[offset:], bn)
	offset += n
	n = copy(block[offset:], keyb)
	offset += n
	n = copy(block[offset:], value)
	offset += n
	bucket.Bloomfilter.Insert(keyh)

	now := h.Clock.NowNano()
	// write existing entries to new block, stop when no space left in block
	offsetOld := int(h.headerSize)
	count := 1 // new entry
	for i := 0; i < int(header.size); i++ {
		// nead to check entry meta first and remove duplicate/expired entries
		var entry BucketEntry
		err := h.entrySerializer.Unmarshal(
			bucketBytes[offsetOld:offsetOld+int(h.entrySize)],
			&entry,
		)
		if err != nil {
			return err
		}
		total := entry.keySize + entry.valueSize + h.entrySize
		if entry.hash == keyh || (entry.expire > 0 && entry.expire <= now) {
			offsetOld += int(total)
			continue
		}
		// not enough space, stop loop
		if total > h.BucketSize-uint64(offset) {
			break
		}
		n = copy(block[offset:], bucketBytes[offsetOld:offsetOld+int(total)])
		offset += n
		offsetOld += n
		count += 1
		bucket.Bloomfilter.Insert(entry.hash)
	}
	header.endOffset = uint64(offset)
	header.checksum = xxh3.Hash(block[h.headerSize:header.endOffset])
	header.size = uint64(count)
	hb, err := h.headerSerializer.Marshal(*header)
	if err != nil {
		return err
	}
	// add header
	_ = copy(block[0:], hb)
	err = h.saveBucketData(int(index), block)
	return err
}

func (h *BigHash) Insert(key []byte, value []byte, cost int64, expire int64) error {
	kh := xxh3.Hash(key)
	return h.addToBucket(kh, key, value, cost, expire)
}

func (h *BigHash) Delete(key []byte) error {
	kh := xxh3.Hash(key)
	return h.deleteFromBucket(kh, key)
}

func (h *BigHash) getFromBucket(keyh uint64, key []byte) (entry BucketEntry, item *alloc.AllocItem, ok bool, err error) {
	index := keyh % h.numBuckets
	bucket := h.buckets[index]
	bucket.mu.RLock()
	if !bucket.Bloomfilter.Exist(keyh) {
		bucket.mu.RUnlock()
		return entry, nil, ok, err
	}
	alloc, header, err := h.loadBucketData(int(index))
	bucket.mu.RUnlock()
	if err != nil {
		return entry, alloc, ok, err
	}
	if header.size == 0 {
		return entry, alloc, false, nil
	}
	bucketData := alloc.Data

	// empty bucket
	if header.size == 0 {
		return entry, alloc, false, err
	}
	offset := int(h.headerSize)
	for i := 0; i < int(header.size); i++ {
		err := h.entrySerializer.Unmarshal(
			bucketData[offset:offset+int(h.entrySize)],
			&entry,
		)
		if err != nil {
			return entry, alloc, ok, err
		}
		offset += int(h.entrySize)
		if entry.hash == keyh && bytes.Equal(bucketData[offset:offset+int(entry.keySize)], key) {
			offset += int(entry.keySize)
			alloc.Data = alloc.Data[offset : offset+int(entry.valueSize)]
			return entry, alloc, true, nil

		} else {
			offset += int(entry.keySize + entry.valueSize)
		}
	}
	return entry, alloc, ok, err
}

func (h *BigHash) Lookup(key []byte) (item *alloc.AllocItem, cost int64, expire int64, ok bool, err error) {
	kh := xxh3.Hash(key)
	entry, item, ok, err := h.getFromBucket(kh, key)
	if err != nil {
		return nil, cost, expire, ok, err
	}
	if ok {
		return item, entry.cost, entry.expire, true, nil
	}
	return nil, cost, expire, false, nil
}
