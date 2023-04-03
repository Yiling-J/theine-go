package internal

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueue_PushPop(t *testing.T) {
	q := NewQueue()

	q.Push(1)
	q.Push(2)
	assert.Equal(t, 1, q.Pop())
	assert.Equal(t, 2, q.Pop())
	assert.True(t, q.Empty())
}

func TestQueue_Empty(t *testing.T) {
	q := NewQueue()
	assert.True(t, q.Empty())
	q.Push(1)
	assert.False(t, q.Empty())
}

func BenchmarkPushPopOneProducer(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(1)
	q := NewQueue()
	go func() {
		i := 0
		for {
			r := q.Pop()
			if r == nil {
				runtime.Gosched()
				continue
			}
			if r.(string) == "-1" {
				wg.Done()
				return
			}
			i++
		}
	}()

	keys := []string{}
	for i := 0; i < 10000; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			q.Push(keys[rand.Intn(10000)])
		}
	})
	q.Push("-1")
	wg.Wait()
}
