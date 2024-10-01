package internal

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func writer(entry *Entry[int, int], num_iterations int, cdone chan bool, c *atomic.Uint32) {
	for i := 0; i < num_iterations; i++ {
		entry.Lock()
		if n := entry.value; n != 0 {
			entry.Unlock()
			panic(fmt.Sprintf("read %d\n", n))
		}
		if i&1 == 0 {
			c.Add(1)
			entry.key = 5
		} else {
			entry.key = 1
		}
		entry.value += 10000
		for i := 0; i < 100; i++ {
		}
		entry.value -= 10000
		entry.Unlock()
	}
	cdone <- true
}

func reader(entry *Entry[int, int], num_iterations int, cdone chan bool, c *atomic.Uint32) {
	var prev uint32
	for i := 0; i < num_iterations; i++ {
		v, ok := entry.Read(1)
		if !ok {
			if n := c.Load(); n < prev {
				panic(fmt.Sprintf("keychange new %d prev %d\n", n, prev))
			} else {
				prev = n
			}
		}
		if v != 0 && v != 10000 {
			panic("wrong value")
		}
		for i := 0; i < 100; i++ {
		}
	}
	cdone <- true
}

func TestSeqlock_Parallel(t *testing.T) {
	entry := &Entry[int, int]{key: 1, value: 0}
	var keyChange atomic.Uint32
	cdone := make(chan bool)
	go writer(entry, 1000, cdone, &keyChange)
	var i int
	for i = 0; i < 10; i++ {
		go reader(entry, 1000, cdone, &keyChange)
	}

	go writer(entry, 1000, cdone, &keyChange)
	for ; i < 20; i++ {
		go reader(entry, 1000, cdone, &keyChange)
	}
	// Wait for the 2 writers and all readers to finish.
	for i := 0; i < 22; i++ {
		<-cdone
	}

	require.Equal(t, 1000, int(keyChange.Load()))

}

type triple struct {
	a int
	q [50]int
	b int
	e [100]int
	c int
}

func TestSeqlock_Full(t *testing.T) {
	const (
		goroutines         = 100
		numberOfIterations = 1000000
	)

	entry := &Entry[int, triple]{key: 1, value: triple{}}
	var ready atomic.Int64
	for i := 0; i < goroutines; i++ {
		go func() {
			for ready.Load() == 0 {
				runtime.Gosched()
			}

			for j := 0; j < numberOfIterations; j++ {
				v, _ := entry.Read(1)
				if v.a+100 != v.b || v.c != v.a+v.b {
					t.Errorf("not valid value state. got: %+v", v)
				}
			}

			ready.Add(int64(-1))
		}()
	}

	counter := 0
	for {
		entry.UpdateValue(triple{
			a: counter,
			b: counter + 100,
			c: 2*counter + 100,
		})
		counter++
		if counter == 1 {
			ready.Add(int64(goroutines))
		}
		if ready.Load() == 0 {
			break
		}
	}

	t.Logf("counter: %d", counter)
}
