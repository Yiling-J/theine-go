package internal

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type Foo struct {
	Bar string
}

type FooWithKey struct {
	Bar string
}

func (k *FooWithKey) StringKey() string {
	return k.Bar
}

func TestStringKey(t *testing.T) {
	hasher := NewHasher[string]()
	h := hasher.hash(strconv.Itoa(123456))
	for i := 0; i < 10; i++ {
		require.Equal(t, h, hasher.hash(strconv.Itoa(123456)))
	}
}

func TestStructStringKey(t *testing.T) {
	hasher1 := NewHasher[Foo]()
	hasher2 := NewHasher[FooWithKey]()
	h1 := uint64(0)
	h2 := uint64(0)
	for i := 0; i < 10; i++ {
		foo := Foo{Bar: strconv.Itoa(123456)}
		if h1 == 0 {
			h1 = hasher1.hash(foo)
		} else {
			require.NotEqual(t, h1, hasher1.hash(foo))
		}
	}
	for i := 0; i < 10; i++ {
		foo := FooWithKey{Bar: strconv.Itoa(123456)}
		if h2 == 0 {
			h2 = hasher2.hash(foo)
		} else {
			require.Equal(t, h2, hasher2.hash(foo))
		}
	}
}
