package internal

type Lru[K comparable, V any] struct {
	list *List[K, V]
}

func NewLru[K comparable, V any](size uint) *Lru[K, V] {
	return &Lru[K, V]{
		list: NewList[K, V](size, LIST),
	}
}

func (s *Lru[K, V]) insert(entry *Entry[K, V]) *Entry[K, V] {
	return s.list.PushFront(entry)
}

func (s *Lru[K, V]) access(entry *Entry[K, V]) {
	s.list.MoveToFront(entry)
}

func (s *Lru[K, V]) pop() *Entry[K, V] {
	return s.list.PopTail()
}

type Slru[K comparable, V any] struct {
	probation *List[K, V]
	protected *List[K, V]
	maxsize   uint
}

func NewSlru[K comparable, V any](size uint) *Slru[K, V] {
	return &Slru[K, V]{
		maxsize:   size,
		probation: NewList[K, V](size, LIST),
		protected: NewList[K, V](uint(float32(size)*0.8), LIST),
	}
}

func (s *Slru[K, V]) insert(entry *Entry[K, V]) *Entry[K, V] {
	var evicted *Entry[K, V]
	if s.probation.Len()+s.protected.Len() >= int(s.maxsize) {
		evicted = s.probation.PopTail()
	}
	s.probation.PushFront(entry)
	return evicted
}

func (s *Slru[K, V]) victim() *Entry[K, V] {
	if s.probation.Len()+s.protected.Len() < int(s.maxsize) {
		return nil
	}
	return s.probation.Back()
}

func (s *Slru[K, V]) access(entry *Entry[K, V]) {
	switch entry.list(LIST) {
	case s.probation:
		s.probation.remove(entry)
		evicted := s.protected.PushFront(entry)
		if evicted != nil {
			s.probation.PushFront(evicted)
		}
	case s.protected:
		s.protected.MoveToFront(entry)
	}
}
