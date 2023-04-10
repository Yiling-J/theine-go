package internal

type CountMinSketch struct {
	rowCounterSize uint
	row64Size      uint
	rowMask        uint
	table          []uint64
	additions      uint
	sampleSize     uint
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
	}
}

func (s *CountMinSketch) indexOf(h uint64, offset uint8) (uint, uint) {
	hn := h + uint64(offset)*(h>>32)
	i := uint(hn & uint64(s.rowMask))
	index := uint(offset)*s.row64Size + (i >> 4)
	off := (i & 0xF) << 2
	return index, off
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
	index0, offset0 := s.indexOf(h, 0)
	index1, offset1 := s.indexOf(h, 1)
	index2, offset2 := s.indexOf(h, 2)
	index3, offset3 := s.indexOf(h, 3)

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

func (s *CountMinSketch) count(h uint64, offset uint8) uint {
	index, off := s.indexOf(h, offset)
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
	m := min(s.count(h, 0), 100)
	m = min(s.count(h, 1), m)
	m = min(s.count(h, 2), m)
	m = min(s.count(h, 3), m)
	return m
}
