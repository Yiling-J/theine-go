package nvm

import (
	"bytes"
	"errors"
	"sync"
	"time"
	"unsafe"

	"github.com/Yiling-J/theine-go/internal/alloc"
	"github.com/Yiling-J/theine-go/internal/clock"
	"github.com/Yiling-J/theine-go/internal/nvm/serializers"
	"github.com/zeebo/xxh3"
)

const (
	alignSize   = 512
	readBufSize = 64
)

func spread(h uint64) uint64 {
	h ^= h >> 17
	h *= 0xed5ad4bb
	h ^= h >> 11
	h *= 0xac4c1b51
	h ^= h >> 15
	return h
}

func align(n int) int {
	l := n % alignSize
	if l != 0 {
		return n + alignSize - l
	}
	return n
}

type BlockInfo struct {
	address  uint32
	sizeHint uint32
	removed  bool
}

type BlockInfoP struct {
	KeyHash  uint64
	Address  uint32
	SizeHint uint32
}

type BlockEntry struct {
	keySize   uint64
	valueSize uint64
	cost      int64
	expire    int64
	checksum  uint64
}

type BlockCache struct {
	Offset          uint64
	mu              *sync.RWMutex
	Clock           *clock.Clock
	CacheSize       uint64
	entrySize       uint64
	RegionSize      uint32
	index           map[uint64]*BlockInfo
	entrySerializer serializers.Serializer[BlockEntry]
	regionManager   *RegionManager
}

func NewBlockCache(cacheSize int, regionSize int, cleanRegionSize uint32, offset uint64, allocator *alloc.Allocator, errHandler func(err error)) *BlockCache {
	regionSize = align(regionSize)
	regionCount := cacheSize / regionSize
	b := &BlockCache{
		Offset:          offset,
		mu:              &sync.RWMutex{},
		entrySerializer: serializers.NewMemorySerializer[BlockEntry](),
		CacheSize:       uint64(cacheSize),
		RegionSize:      uint32(regionSize),
		entrySize:       uint64(unsafe.Sizeof(BlockEntry{})),
		index:           make(map[uint64]*BlockInfo, cacheSize/regionSize),
		Clock:           &clock.Clock{Start: time.Now().UTC()},
	}
	if errHandler == nil {
		errHandler = func(err error) {}
	}
	b.regionManager = NewRegionManager(
		offset, uint32(regionSize), uint32(regionCount), cleanRegionSize, b.removeRegion,
		allocator, errHandler,
	)

	return b

}

func (c *BlockCache) realAddress(address uint32) (uint64, uint64) {
	base := uint64(address) * uint64(alignSize)
	return base / uint64(c.RegionSize), base % uint64(c.RegionSize)
}

func (c *BlockCache) Lookup(key []byte) (item *alloc.AllocItem, cost int64, expire int64, ok bool, err error) {
	kh := xxh3.Hash(key)
	c.mu.RLock()
	index, ok := c.index[kh]
	if !ok {
		c.mu.RUnlock()
		return nil, cost, expire, false, nil
	}
	c.mu.RUnlock()

	rid, offset := c.realAddress(index.address)
	item, err = c.regionManager.GetData(
		index, rid, offset, uint64(index.sizeHint)*alignSize,
	)
	if err != nil {
		return item, cost, expire, false, err
	}
	if item == nil {
		return item, cost, expire, false, nil
	}
	var entry BlockEntry
	err = c.entrySerializer.Unmarshal(item.Data[:c.entrySize], &entry)
	if err != nil {
		return item, cost, expire, false, err
	}
	checksum := xxh3.Hash(item.Data[c.entrySize : c.entrySize+entry.keySize+entry.valueSize])
	if checksum != entry.checksum {
		return item, cost, expire, false, errors.New("checksum mismatch")
	}

	if !bytes.Equal(key, item.Data[c.entrySize:c.entrySize+entry.keySize]) {
		return item, cost, expire, false, err
	}
	offset = c.entrySize + entry.keySize
	item.Data = item.Data[offset : offset+entry.valueSize]
	return item, entry.cost, entry.expire, true, err

}

func (c *BlockCache) Insert(key []byte, value []byte, cost int64, expire int64) error {
	kh := xxh3.Hash(key)
	header := BlockEntry{
		keySize:   uint64(len(key)),
		valueSize: uint64(len(value)),
		cost:      cost,
		expire:    expire,
	}
	size := int(c.entrySize) + len(key) + len(value)
	res := size % alignSize
	if res != 0 {
		size += (alignSize - res)
	}
	rid, offset, buffer, cb, err := c.regionManager.Allocate(size)
	if err != nil {
		return err
	}

	// esacpe
	_, err = buffer.Write(make([]byte, c.entrySize))
	if err != nil {
		return err
	}
	_, err = buffer.Write(key)
	if err != nil {
		return err
	}
	_, err = buffer.Write(value)
	if err != nil {
		return err
	}
	b := buffer.Bytes()
	header.checksum = xxh3.Hash(b[int(c.entrySize):])
	hb, err := c.entrySerializer.Marshal(header)
	if err != nil {
		return err
	}
	_ = copy(b[:], hb)
	cb()
	c.mu.Lock()
	c.index[kh] = &BlockInfo{
		sizeHint: uint32(size / alignSize),
		address:  uint32((uint64(rid)*uint64(c.RegionSize) + offset) / alignSize),
	}
	c.mu.Unlock()
	return nil
}

func (c *BlockCache) removeRegion(data []byte, endOffset uint64) error {
	offset := 0
	for offset < int(endOffset) {
		var entry BlockEntry
		err := c.entrySerializer.Unmarshal(data[offset:offset+int(c.entrySize)], &entry)
		if err != nil {
			return err
		}
		offset += int(c.entrySize)
		checksum := xxh3.Hash(data[offset : offset+int(entry.keySize+entry.valueSize)])
		if checksum != entry.checksum {
			return errors.New("checksum mismatch")
		}
		keyh := xxh3.Hash(data[offset : offset+int(entry.keySize)])
		c.mu.Lock()
		i, ok := c.index[keyh]
		if ok {
			i.removed = true
			delete(c.index, keyh)
		}
		c.mu.Unlock()
		offset += int(entry.keySize + entry.valueSize)
		offset = align(offset)
	}
	return nil
}

func (c *BlockCache) Delete(key []byte) {
	kh := xxh3.Hash(key)
	c.mu.Lock()
	delete(c.index, kh)
	c.mu.Unlock()
}
