package serializers

import "unsafe"

type MemorySerializer[V any] struct {
	Size int
	Str  bool
}

func NewMemorySerializer[V any]() *MemorySerializer[V] {
	var v V
	serializer := &MemorySerializer[V]{Size: int(unsafe.Sizeof(v))}
	switch ((interface{})(v)).(type) {
	case string:
		serializer.Str = true
	default:
		serializer.Size = int(unsafe.Sizeof(v))
	}
	return serializer
}

func (s *MemorySerializer[V]) Marshal(v V) ([]byte, error) {
	if s.Str {
		return []byte(*(*string)(unsafe.Pointer(&v))), nil
	}
	return *(*[]byte)(unsafe.Pointer(&struct {
		data unsafe.Pointer
		len  int
	}{unsafe.Pointer(&v), s.Size})), nil
}

func (s *MemorySerializer[V]) Unmarshal(raw []byte, v *V) error {
	if s.Str {
		s := string(raw)
		*v = *(*V)(unsafe.Pointer(&s))
		return nil
	}
	m := *(*struct {
		data unsafe.Pointer
		len  int
	})(unsafe.Pointer(&raw))
	*v = *(*V)(m.data)
	return nil
}
