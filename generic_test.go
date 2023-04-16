package theine_test

import (
	"testing"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

func assertGeneric[T comparable](t *testing.T, k1 T, k2 T) {
	client, err := theine.NewBuilder[T, int](10).Build()
	require.Nil(t, err)
	client.Set(k1, 1, 1)
	v, ok := client.Get(k1)
	require.True(t, ok)
	require.Equal(t, 1, v)
	_, ok = client.Get(k2)
	require.False(t, ok)
}

type t1 struct {
	id int
}

type t2 struct {
	t1 t1
}

type t3 struct {
	t1 *t1
}

func TestGenericKey(t *testing.T) {
	assertGeneric(t, 1, 2)
	assertGeneric(t, "a", "b")
	assertGeneric(t, 1.1, 1.2)
	assertGeneric(t, true, false)
	assertGeneric(t, t1{id: 1}, t1{id: 2})
	assertGeneric(t, &t1{id: 1}, &t1{id: 1})
	assertGeneric(t, t2{t1: t1{id: 1}}, t2{t1: t1{id: 2}})
	assertGeneric(t, t3{t1: &t1{id: 1}}, t3{t1: &t1{id: 1}})
}
