package internal

import (
	"math/bits"
	"sync/atomic"
)

const (
	resetMask = 0x7777777777777777
	oneMask   = 0x1111111111111111
)

type CountMinSketch struct {
	Table      []atomic.Uint64
	Additions  uint
	SampleSize uint
	BlockMask  uint
	mu         *RBMutex
}

// sketch for persistence
type CountMinSketchPersist struct {
	Table      []uint64
	Additions  uint
	SampleSize uint
	BlockMask  uint
}

func (s *CountMinSketchPersist) CountMinSketch() *CountMinSketch {
	p := &CountMinSketch{
		Additions: s.Additions, SampleSize: s.SampleSize, BlockMask: s.BlockMask,
		Table: make([]atomic.Uint64, len(s.Table)),
		mu:    NewRBMutex(),
	}
	for i := 0; i < len(s.Table); i++ {
		p.Table[i].Store(s.Table[i])
	}
	return p
}

func NewCountMinSketch() *CountMinSketch {
	new := &CountMinSketch{mu: NewRBMutex()}
	new.EnsureCapacity(16)
	return new
}

func (s *CountMinSketch) CountMinSketchPersist() *CountMinSketchPersist {
	p := &CountMinSketchPersist{
		Additions: s.Additions, SampleSize: s.SampleSize, BlockMask: s.BlockMask,
		Table: make([]uint64, 0, len(s.Table)),
	}
	for i := 0; i < len(s.Table); i++ {
		p.Table = append(p.Table, s.Table[i].Load())
	}
	return p
}

// indexOf return table index and counter index together
func (s *CountMinSketch) indexOf(h uint64, block uint64, offset uint8) (uint, uint) {
	counterHash := h + uint64(1+offset)*(h>>32)
	// max block + 7(8 * 8 bytes), fit 64 bytes cache line
	index := block + counterHash&1 + uint64(offset<<1)
	return uint(index), uint((counterHash & 0xF) << 2)
}

func (s *CountMinSketch) inc(index uint, offset uint) bool {
	mask := uint64(0xF << offset)
	v := s.Table[index].Load()
	if v&mask != mask {
		s.Table[index].Store(v + 1<<offset)
		return true
	}
	return false
}

func (s *CountMinSketch) Add(h uint64) bool {
	hn := spread(h)
	block := (hn & uint64(s.BlockMask)) << 3
	hc := rehash(h)
	index0, offset0 := s.indexOf(hc, block, 0)
	index1, offset1 := s.indexOf(hc, block, 1)
	index2, offset2 := s.indexOf(hc, block, 2)
	index3, offset3 := s.indexOf(hc, block, 3)

	added := s.inc(index0, offset0)
	added = s.inc(index1, offset1) || added
	added = s.inc(index2, offset2) || added
	added = s.inc(index3, offset3) || added

	if added {
		s.Additions += 1
		if s.Additions == s.SampleSize {
			s.reset()
			return true
		}
	}
	return false
}

// used in test
func (s *CountMinSketch) Addn(h uint64, n int) {
	hn := spread(h)
	block := (hn & uint64(s.BlockMask)) << 3
	hc := rehash(h)
	index0, offset0 := s.indexOf(hc, block, 0)
	index1, offset1 := s.indexOf(hc, block, 1)
	index2, offset2 := s.indexOf(hc, block, 2)
	index3, offset3 := s.indexOf(hc, block, 3)

	for i := 0; i < n; i++ {

		s.inc(index0, offset0)
		s.inc(index1, offset1)
		s.inc(index2, offset2)
		s.inc(index3, offset3)
	}

}

func (s *CountMinSketch) reset() {
	count := 0
	for i := range s.Table {
		v := s.Table[i].Load()
		count += bits.OnesCount64(v & oneMask)
		s.Table[i].Store((v >> 1) & resetMask)
	}
	s.Additions = (s.Additions - uint(count>>2)) >> 1
}

func (s *CountMinSketch) count(h uint64, block uint64, offset uint8) uint {
	index, off := s.indexOf(h, block, offset)
	count := (s.Table[index].Load() >> off) & 0xF
	return uint(count)
}

func min(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}

func (s *CountMinSketch) Estimate(h uint64) uint {
	hn := spread(h)
	t := s.mu.RLock()
	block := (hn & uint64(s.BlockMask)) << 3
	hc := rehash(h)
	m := min(s.count(hc, block, 0), 100)
	m = min(s.count(hc, block, 1), m)
	m = min(s.count(hc, block, 2), m)
	m = min(s.count(hc, block, 3), m)
	s.mu.RUnlock(t)
	return m
}

func (s *CountMinSketch) EnsureCapacity(size uint) {
	if len(s.Table) >= int(size) {
		return
	}
	if size < 16 {
		size = 16
	}
	newSize := next2Power(size)
	s.mu.Lock()
	s.Table = make([]atomic.Uint64, newSize)
	s.SampleSize = 10 * size
	s.BlockMask = uint((len(s.Table) >> 3) - 1)
	s.Additions = 0
	s.mu.Unlock()
}

func spread(h uint64) uint64 {
	h ^= h >> 17
	h *= 0xed5ad4bb
	h ^= h >> 11
	h *= 0xac4c1b51
	h ^= h >> 15
	return h
}

func rehash(h uint64) uint64 {
	h *= 0x31848bab
	h ^= h >> 14
	return h
}
