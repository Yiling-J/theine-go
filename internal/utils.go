package internal

import (
	"reflect"
	"unsafe"

	"github.com/zeebo/xxh3"
)

type Hasher[K comparable] struct {
	ksize int
	kstr  bool
	kfunc func(K) string
}

type StringKey interface {
	StringKey() string
}

func NewHasher[K comparable]() *Hasher[K] {
	h := &Hasher[K]{}
	var k K
	var tmp interface{} = k
	var pointer = true
	if reflect.ValueOf(k).Kind() == reflect.Struct {
		tmp = &k
		pointer = false
	}
	switch (tmp).(type) {
	case string:
		h.kstr = true
	case StringKey:
		if pointer {
			h.kfunc = func(key K) string { return ((interface{})(key)).(StringKey).StringKey() }
		} else {
			h.kfunc = func(key K) string { return ((interface{})(&key)).(StringKey).StringKey() }
		}
	default:
		h.ksize = int(unsafe.Sizeof(k))
	}
	return h
}

func (h *Hasher[K]) hash(key K) uint64 {
	var strKey string
	if h.kstr {
		strKey = *(*string)(unsafe.Pointer(&key))
	} else if h.kfunc != nil {
		strKey = h.kfunc(key)
	} else {
		strKey = *(*string)(unsafe.Pointer(&struct {
			data unsafe.Pointer
			len  int
		}{unsafe.Pointer(&key), h.ksize}))
	}
	return xxh3.HashString(strKey)
}
