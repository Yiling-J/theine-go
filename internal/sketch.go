package internal

type CountMinSketch struct {
	table          []uint64
	rowCounterSize uint
	row64Size      uint
	rowMask        uint
	additions      uint
	sampleSize     uint
	blockMask      uint
}

func NewCountMinSketch(size uint) *CountMinSketch {
	rowCounterSize := next2Power(size * 3)
	row64Size := rowCounterSize / 16
	rowMask := rowCounterSize - 1
	table := make([]uint64, rowCounterSize>>2)
	return &CountMinSketch{
		rowCounterSize: rowCounterSize,
		row64Size:      row64Size,
		rowMask:        rowMask,
		table:          table,
		additions:      0,
		sampleSize:     10 * rowCounterSize,
		blockMask:      (rowCounterSize>>2)>>3 - 1,
	}
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
	if s.table[index]&mask != mask {
		s.table[index] += 1 << offset
		return true
	}
	return false
}

func (s *CountMinSketch) Add(h uint64) bool {
	hn := spread(h)
	block := (hn & uint64(s.blockMask)) << 3
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
		s.additions += 1
		if s.additions == s.sampleSize {
			s.reset()
			return true
		}
	}
	return false
}

func (s *CountMinSketch) reset() {
	for i := range s.table {
		s.table[i] = s.table[i] >> 1
	}
	s.additions = s.additions >> 1
}

func (s *CountMinSketch) count(h uint64, block uint64, offset uint8) uint {
	index, off := s.indexOf(h, block, offset)
	count := (s.table[index] >> off) & 0xF
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
	block := (hn & uint64(s.blockMask)) << 3
	hc := rehash(h)
	m := min(s.count(hc, block, 0), 100)
	m = min(s.count(hc, block, 1), m)
	m = min(s.count(hc, block, 2), m)
	m = min(s.count(hc, block, 3), m)
	return m
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
