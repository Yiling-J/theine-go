// Copyright 2019, Joshua J Baker

// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.

package internal

import (
	"unsafe"

	"github.com/zeebo/xxh3"
)

const (
	loadFactor  = 0.85                      // must be above 50%
	dibBitSize  = 16                        // 0xFFFF
	hashBitSize = 64 - dibBitSize           // 0xFFFFFFFFFFFF
	maxHash     = ^uint64(0) >> dibBitSize  // max 28,147,497,671,0655
	maxDIB      = ^uint64(0) >> hashBitSize // max 65,535
)

// hash returns a 48-bit hash for 64-bit environments, or 32-bit hash for
// 32-bit environments.
func (m *Map[K, V]) hash(key K) int {
	// The unsafe package is used here to cast the key into a string container
	// so that the hasher can work. The hasher normally only accept a string or
	// []byte, but this effectively allows it to accept value type.
	// The m.kstr bool, which is set from the New function, indicates that the
	// key is known to already be a true string. Otherwise, a fake string is
	// derived by setting the string data to value of the key, and the string
	// length to the size of the value.
	var strKey string
	if m.kstr {
		strKey = *(*string)(unsafe.Pointer(&key))
	} else {
		strKey = *(*string)(unsafe.Pointer(&struct {
			data unsafe.Pointer
			len  int
		}{unsafe.Pointer(&key), m.ksize}))
	}
	// Now for the actual hashing.
	return int(xxh3.HashString(strKey) >> dibBitSize)
}

// Map is a hashmap. Like map[string]interface{}
type Map[K comparable, V any] struct {
	cap      int
	length   int
	mask     int
	growAt   int
	shrinkAt int
	buckets  []*Entry[K, V]
	ksize    int
	kstr     bool
}

// New returns a new Map. Like map[string]interface{}
func NewMap[K comparable, V any](cap int) *Map[K, V] {
	m := new(Map[K, V])
	m.cap = cap
	sz := 8
	for sz < m.cap {
		sz *= 2
	}
	if m.cap > 0 {
		m.cap = sz
	}
	m.buckets = make([]*Entry[K, V], sz)
	m.mask = len(m.buckets) - 1
	m.growAt = int(float64(len(m.buckets)) * loadFactor)
	m.shrinkAt = int(float64(len(m.buckets)) * (1 - loadFactor))
	m.detectHasher()
	return m
}

func (m *Map[K, V]) detectHasher() {
	// Detect the key type. This is needed by the hasher.
	var k K
	switch ((interface{})(k)).(type) {
	case string:
		m.kstr = true
	default:
		m.ksize = int(unsafe.Sizeof(k))
	}
}

func (m *Map[K, V]) resize(newCap int) {
	nmap := NewMap[K, V](newCap)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			nmap.set(m.buckets[i].hash(), m.buckets[i].key, m.buckets[i])
		}
	}
	cap := m.cap
	*m = *nmap
	m.cap = cap
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (m *Map[K, V]) Set(key K, entry *Entry[K, V]) (V, bool) {
	if len(m.buckets) == 0 {
		*m = *NewMap[K, V](0)
	}
	if m.length >= m.growAt {
		m.resize(len(m.buckets) * 2)
	}
	return m.set(m.hash(key), key, entry)
}

func (m *Map[K, V]) set(hash int, key K, entry *Entry[K, V]) (prev V, ok bool) {
	entry.hdib = makeHDIB(hash, 1)
	e := entry
	i := e.hash() & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			m.buckets[i] = e
			m.length++
			return prev, false
		}
		if e.hash() == m.buckets[i].hash() && e.key == m.buckets[i].key {
			prev = m.buckets[i].value
			m.buckets[i].value = e.value
			return prev, true
		}
		if m.buckets[i].dib() < e.dib() {
			e, m.buckets[i] = m.buckets[i], e
		}
		i = (i + 1) & m.mask
		e.setDIB(e.dib() + 1)
	}
}

// Get returns a value for a key.
// Returns false when no value has been assign for key.
func (m *Map[K, V]) Get(key K) (entry *Entry[K, V], ok bool) {
	if len(m.buckets) == 0 {
		return nil, false
	}
	hash := m.hash(key)
	i := hash & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			return nil, false
		}
		if m.buckets[i].hash() == hash && m.buckets[i].key == key {
			return m.buckets[i], true
		}
		i = (i + 1) & m.mask
	}
}

// Len returns the number of values in map.
func (m *Map[K, V]) Len() int {
	return m.length
}

// Delete deletes a value for a key.
// Returns the deleted value, or false when no value was assigned.
func (m *Map[K, V]) Delete(key K) (prev V, deleted bool) {
	if len(m.buckets) == 0 {
		return prev, false
	}
	hash := m.hash(key)
	i := hash & m.mask
	for {
		if m.buckets[i].dib() == 0 {
			return prev, false
		}
		if m.buckets[i].hash() == hash && m.buckets[i].key == key {
			prev = m.buckets[i].value
			m.remove(i)
			return prev, true
		}
		i = (i + 1) & m.mask
	}
}

func (m *Map[K, V]) remove(i int) {
	m.buckets[i].setDIB(0)
	for {
		pi := i
		i = (i + 1) & m.mask
		if m.buckets[i].dib() <= 1 {
			m.buckets[pi] = nil
			break
		}
		m.buckets[pi] = m.buckets[i]
		m.buckets[pi].setDIB(m.buckets[pi].dib() - 1)
	}
	m.length--
	if len(m.buckets) > m.cap && m.length <= m.shrinkAt {
		m.resize(m.length)
	}
}

// Scan iterates over all key/values.
// It's not safe to call or Set or Delete while scanning.
func (m *Map[K, V]) Scan(iter func(key K, value V) bool) {
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			if !iter(m.buckets[i].key, m.buckets[i].value) {
				return
			}
		}
	}
}

// Keys returns all keys as a slice
func (m *Map[K, V]) Keys() []K {
	keys := make([]K, 0, m.length)
	for i := 0; i < len(m.buckets); i++ {
		if m.buckets[i].dib() > 0 {
			keys = append(keys, m.buckets[i].key)
		}
	}
	return keys
}
