//go:build go1.24
// +build go1.24

package hasher

import (
	"hash/maphash"
)

type Hasher[K comparable] struct {
	seed  maphash.Seed
	kstr  bool           // Deprecated
	kfunc func(K) string // Deprecated
}

func NewHasher[K comparable](stringKeyFunc func(K) string) *Hasher[K] {
	// TODO: stringKeyFunc was used prior to Go 1.24 when Comparable was unavailable.
	// With the introduction of Comparable, special handling for string keys is no longer necessary.
	return &Hasher[K]{kfunc: stringKeyFunc, seed: maphash.MakeSeed()}
}

func (h *Hasher[K]) Hash(key K) uint64 {
	if h.kfunc != nil {
		return maphash.Comparable(h.seed, h.kfunc(key))
	}
	return maphash.Comparable(h.seed, key)
}
