package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueue_PushPop(t *testing.T) {
	q := NewQueue[int]()

	q.Push(1)
	q.Push(2)
	v, ok := q.Pop()
	assert.True(t, ok)
	assert.Equal(t, 1, v)
	v, ok = q.Pop()
	assert.True(t, ok)
	assert.Equal(t, 2, v)
	_, ok = q.Pop()
	assert.False(t, ok)
}

func TestQueue_Empty(t *testing.T) {
	q := NewQueue[int]()
	assert.True(t, q.Empty())
	q.Push(1)
	assert.False(t, q.Empty())
}
