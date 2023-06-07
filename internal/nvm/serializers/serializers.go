package serializers

type Serializer[T any] interface {
	Marshal(v T) ([]byte, error)
	Unmarshal(raw []byte, v *T) error
}
