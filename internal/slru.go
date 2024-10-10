package internal

type Slru[K comparable, V any] struct {
	probation *List[K, V]
	protected *List[K, V]
	maxsize   uint
}

func NewSlru[K comparable, V any](size uint) *Slru[K, V] {
	return &Slru[K, V]{
		maxsize:   size,
		probation: NewList[K, V](size, LIST_PROBATION),
		protected: NewList[K, V](uint(float32(size)*0.8), LIST_PROTECTED),
	}
}

func (s *Slru[K, V]) insert(entry *Entry[K, V]) *Entry[K, V] {
	return s.probation.PushFront(entry)
}

func (s *Slru[K, V]) victim() *Entry[K, V] {
	if s.probation.Len()+s.protected.Len() < int(s.maxsize) {
		return nil
	}
	return s.probation.Back()
}

func (s *Slru[K, V]) access(entry *Entry[K, V]) {

	if entry.flag.IsProbation() {
		s.probation.remove(entry)
		evicted := s.protected.PushFront(entry)
		if evicted != nil {
			s.probation.PushFront(evicted)
		}
	} else if entry.flag.IsProtected() {
		s.protected.MoveToFront(entry)
	}

}

func (s *Slru[K, V]) remove(entry *Entry[K, V]) {
	if entry.flag.IsProbation() {
		s.probation.remove(entry)
	} else if entry.flag.IsProtected() {
		s.protected.remove(entry)
	}

}

func (s *Slru[K, V]) updateCost(entry *Entry[K, V], delta int64) {
	if entry.flag.IsProbation() {
		s.probation.len.Add(delta)
	} else if entry.flag.IsProtected() {
		s.protected.len.Add(delta)
	}

}
