package alloc

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/ncw/directio"
)

func alignDown(num int, alignment int) int {
	return num - num%alignment
}

func alignUp(num int, alignment int) int {
	return alignDown(num+alignment-1, alignment)
}

type Allocator struct {
	pool       sync.Pool
	itemPool   sync.Pool
	blockSize  int
	bucketSize int
	regionSize int
	mu         sync.Mutex
	current    *BufferItem
}

type BufferItem struct {
	pool   *sync.Pool
	buffer *bytes.Buffer
	full   atomic.Bool
	count  atomic.Int32
}

type AllocItem struct {
	buffer    *BufferItem
	allocator *Allocator
	Data      []byte
}

func (a *AllocItem) Deallocate() {
	if a != nil {
		new := a.buffer.count.Add(-1)
		if new == 0 && a.buffer.full.Load() {
			a.buffer.pool.Put(a.buffer)
		}
		a.allocator.itemPool.Put(a)
	}
}

func NewAllocator(bucketSize int, regionSize int, blockSize int) *Allocator {
	a := &Allocator{
		pool: sync.Pool{New: func() any {
			return &BufferItem{
				buffer: bytes.NewBuffer(directio.AlignedBlock(int(regionSize))),
			}
		}},
		itemPool: sync.Pool{New: func() any {
			return &AllocItem{}
		}},
		blockSize:  blockSize,
		bucketSize: bucketSize,
		regionSize: regionSize,
	}
	a.current = a.pool.Get().(*BufferItem)
	a.current.pool = &a.pool
	return a
}

func (a *Allocator) Allocate(size int) *AllocItem {
	item := a.itemPool.Get().(*AllocItem)
	a.mu.Lock()
	current := a.current
	size = alignUp(size, a.blockSize)
	if current.buffer.Len() < size {
		if current.count.Load() == 0 {
			// reuse directly
			current.buffer.Reset()
			current.buffer = bytes.NewBuffer(current.buffer.Bytes()[:current.buffer.Cap()])

		} else {
			// put back to pool in item dealloc callback
			current.full.Store(true)
			current = a.pool.Get().(*BufferItem)
			a.current = current
			current.buffer.Reset()
			current.buffer = bytes.NewBuffer(current.buffer.Bytes()[:current.buffer.Cap()])
			current.pool = &a.pool
			current.count.Store(0)
			current.full.Store(false)
		}
	}
	item.Data = current.buffer.Next(size)
	item.buffer = current
	item.allocator = a
	current.count.Add(1)
	a.mu.Unlock()
	return item
}
