package internal

import (
	"fmt"
	"strings"
)

const (
	LIST       uint8 = 1
	WHEEL_LIST uint8 = 2
)

// List represents a doubly linked list.
// The zero value for List is an empty list ready to use.
type List[K comparable, V any] struct {
	listType uint8       // 1 tinylfu list, 2 timerwheel list
	root     Entry[K, V] // sentinel list element, only &root, root.prev, and root.next are used
	len      int         // current list length excluding (this) sentinel element
	capacity uint
}

// New returns an initialized list.
func NewList[K comparable, V any](size uint, listType uint8) *List[K, V] {
	l := &List[K, V]{listType: listType, capacity: size, root: Entry[K, V]{}}
	l.root.setNext(&l.root, l.listType)
	l.root.setPrev(&l.root, l.listType)
	l.len = 0
	l.capacity = size
	return l
}

func (l *List[K, V]) Reset() {
	l.root.setNext(&l.root, l.listType)
	l.root.setPrev(&l.root, l.listType)
	l.len = 0
}

// Len returns the number of elements of list l.
// The complexity is O(1).
func (l *List[K, V]) Len() int { return l.len }

func (l *List[K, V]) display(listType uint8) string {
	var s []string
	for e := l.Front(); e != nil; e = e.Next(listType) {
		s = append(s, fmt.Sprintf("%v", e.key))
	}
	return strings.Join(s, "/")
}

func (l *List[K, V]) displayReverse(listType uint8) string {
	var s []string
	for e := l.Back(); e != nil; e = e.Prev(listType) {
		s = append(s, fmt.Sprintf("%v", e.key))
	}
	return strings.Join(s, "/")
}

// Front returns the first element of list l or nil if the list is empty.
func (l *List[K, V]) Front() *Entry[K, V] {
	if l.len == 0 {
		return nil
	}
	return l.root.next(l.listType)
}

// Back returns the last element of list l or nil if the list is empty.
func (l *List[K, V]) Back() *Entry[K, V] {
	if l.len == 0 {
		return nil
	}
	return l.root.prev(l.listType)
}

// insert inserts e after at, increments l.len, and evicted entry if capacity exceed
func (l *List[K, V]) insert(e, at *Entry[K, V]) *Entry[K, V] {
	var evicted *Entry[K, V]
	if l.len == int(l.capacity) {
		evicted = l.PopTail()
	}
	switch l.listType {
	case LIST:
		e.meta._list = l
	case WHEEL_LIST:
		e.meta._wheelList = l
	}
	e.setPrev(at, l.listType)
	e.setNext(at.next(l.listType), l.listType)
	e.prev(l.listType).setNext(e, l.listType)
	e.next(l.listType).setPrev(e, l.listType)
	l.len++
	return evicted
}

// PushFront push entry to list head
func (l *List[K, V]) PushFront(e *Entry[K, V]) *Entry[K, V] {
	return l.insert(e, &l.root)
}

// remove removes e from its list, decrements l.len
func (l *List[K, V]) remove(e *Entry[K, V]) {
	e.prev(l.listType).setNext(e.next(l.listType), l.listType)
	e.next(l.listType).setPrev(e.prev(l.listType), l.listType)
	e.setNext(nil, l.listType)
	e.setPrev(nil, l.listType)
	switch l.listType {
	case LIST:
		e.meta._list = nil
	case WHEEL_LIST:
		e.meta._wheelList = nil
	}
	l.len--
}

// move moves e to next to at.
func (l *List[K, V]) move(e, at *Entry[K, V]) {
	if e == at {
		return
	}
	e.prev(l.listType).setNext(e.next(l.listType), l.listType)
	e.next(l.listType).setPrev(e.prev(l.listType), l.listType)

	e.setPrev(at, l.listType)
	e.setNext(at.next(l.listType), l.listType)
	e.prev(l.listType).setNext(e, l.listType)
	e.next(l.listType).setPrev(e, l.listType)
}

// Remove removes e from l if e is an element of list l.
// It returns the element value e.Value.
// The element must not be nil.
func (l *List[K, V]) Remove(e *Entry[K, V]) {
	if e.list(l.listType) == l {
		// if e.list == l, l must have been initialized when e was inserted
		// in l or l == nil (e is a zero Element) and l.remove will crash
		l.remove(e)
	}
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *List[K, V]) MoveToFront(e *Entry[K, V]) {
	if e.list(l.listType) != l || l.root.next(l.listType) == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, &l.root)
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *List[K, V]) MoveToBack(e *Entry[K, V]) {
	if e.list(l.listType) != l || l.root.prev(l.listType) == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, l.root.prev(l.listType))
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *List[K, V]) MoveBefore(e, mark *Entry[K, V]) {
	if e.list(l.listType) != l || e == mark || mark.list(l.listType) != l {
		return
	}
	l.move(e, mark.prev(l.listType))
}

// MoveAfter moves element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *List[K, V]) MoveAfter(e, mark *Entry[K, V]) {
	if e.list(l.listType) != l || e == mark || mark.list(l.listType) != l {
		return
	}
	l.move(e, mark)
}

func (l *List[K, V]) PopTail() *Entry[K, V] {
	entry := l.root.prev(l.listType)
	if entry != nil && entry != &l.root {
		l.remove(entry)
		return entry
	}
	return nil
}
