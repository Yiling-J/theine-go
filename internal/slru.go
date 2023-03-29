package internal

type Lru struct {
	list *List
}

func NewLru(size uint) *Lru {
	return &Lru{
		list: NewList(size, LIST),
	}
}

func (s *Lru) insert(entry *Entry) *Entry {
	return s.list.PushFront(entry)
}

func (s *Lru) access(entry *Entry) {
	s.list.MoveToFront(entry)
}

type Slru struct {
	probation *List
	protected *List
	maxsize   uint
}

func NewSlru(size uint) *Slru {
	return &Slru{
		maxsize:   size,
		probation: NewList(size, LIST),
		protected: NewList(uint(float32(size)*0.8), LIST),
	}
}

func (s *Slru) insert(entry *Entry) *Entry {
	var evicted *Entry
	if s.probation.Len()+s.protected.Len() >= int(s.maxsize) {
		evicted = s.probation.PopTail()
	}
	s.probation.PushFront(entry)
	return evicted
}

func (s *Slru) victim() *Entry {
	if s.probation.Len()+s.protected.Len() < int(s.maxsize) {
		return nil
	}
	return s.probation.Back()
}

func (s *Slru) access(entry *Entry) {
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

func (s *Slru) remove(entry *Entry) {
	switch entry.list(LIST) {
	case s.probation:
		s.probation.remove(entry)
	case s.protected:
		s.protected.remove(entry)
	}
}
