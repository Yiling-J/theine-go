package theine

import (
	"unsafe"

	"github.com/Yiling-J/theine-go/internal/nvm/serializers"
)

func NewMemorySerializer[V any]() *serializers.MemorySerializer[V] {
	var v V
	serializer := &serializers.MemorySerializer[V]{Size: int(unsafe.Sizeof(v))}
	switch ((interface{})(v)).(type) {
	case string:
		serializer.Str = true
	default:
		serializer.Size = int(unsafe.Sizeof(v))
	}
	return serializer
}
