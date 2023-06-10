package internal

import (
	"github.com/Yiling-J/theine-go/internal/clock"
)

type Serializer[T any] interface {
	Marshal(v T) ([]byte, error)
	Unmarshal(raw []byte, v *T) error
}

type SecondaryCacheItem[K comparable, V any] struct {
	entry  *Entry[K, V]
	reason RemoveReason
	shard  *Shard[K, V]
}

type SecondaryCache[K comparable, V any] interface {
	Get(key K) (value V, cost int64, expire int64, ok bool, err error)
	Set(key K, value V, cost int64, expire int64) error
	Delete(key K) error
	SetClock(clock *clock.Clock)
	HandleAsyncError(err error)
}
