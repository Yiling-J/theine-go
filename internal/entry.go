package internal

const (
	NEW int8 = iota
	ALIVE
	REMOVED
)

type MetaData struct {
	prev       *Entry
	next       *Entry
	wheelPrev  *Entry
	wheelNext  *Entry
	_list      *List
	_wheelList *List
}

type Entry struct {
	status int8 // 1 wait remove, 2 removed
	key    string
	value  any
	expire int64
	meta   MetaData
}

func (e *Entry) Clean() {
	e.value = nil
	e.meta.prev = nil
	e.meta.next = nil
	e.meta.wheelPrev = nil
	e.meta.wheelNext = nil
	e.meta._list = nil
	e.meta._wheelList = nil
}

func (e *Entry) Next(listType uint8) *Entry {
	if listType == WHEEL_LIST {
		if p := e.meta.wheelNext; e.meta._wheelList != nil && p != &e.meta._wheelList.root {
			return p
		}
		return nil
	}
	if p := e.meta.next; e.meta._list != nil && p != &e.meta._list.root {
		return p
	}
	return nil
}

func (e *Entry) Prev(listType uint8) *Entry {
	if listType == WHEEL_LIST {
		if p := e.meta.wheelPrev; e.meta._wheelList != nil && p != &e.meta._wheelList.root {
			return p
		}
		return nil
	}
	if p := e.meta.prev; e.meta._list != nil && p != &e.meta._list.root {
		return p
	}
	return nil
}

func (e *Entry) list(listType uint8) *List {
	if listType == WHEEL_LIST {
		return e.meta._wheelList
	}
	return e.meta._list
}

func (e *Entry) setList(list *List, listType uint8) {
	if listType == WHEEL_LIST {
		e.meta._wheelList = list
	}
	e.meta._list = list
}

func (e *Entry) prev(listType uint8) *Entry {
	if listType == WHEEL_LIST {
		return e.meta.wheelPrev
	}
	return e.meta.prev
}

func (e *Entry) next(listType uint8) *Entry {
	if listType == WHEEL_LIST {
		return e.meta.wheelNext
	}
	return e.meta.next
}

func (e *Entry) setPrev(entry *Entry, listType uint8) {
	if listType == WHEEL_LIST {
		e.meta.wheelPrev = entry
	} else {
		e.meta.prev = entry
	}
}

func (e *Entry) setNext(entry *Entry, listType uint8) {
	if listType == WHEEL_LIST {
		e.meta.wheelNext = entry
	} else {
		e.meta.next = entry
	}
}
