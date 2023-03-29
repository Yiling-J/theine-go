package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/dgraph-io/ristretto"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

func fullSyncRun() {
	client, _ := theine.NewSync(100)
	for i := 0; i < 50000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(100000))
		client.SetSync(key, key)
	}
	fmt.Println(client.Len())
}

func syncRun() {
	client, _ := theine.New(100)
	for i := 0; i < 3000000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(100000))
		client.Set(key, key)
	}
	time.Sleep(1 * time.Second)
	fmt.Println(client.Len())
}

func parallelRun() {
	client, _ := theine.New(3000)
	var wg sync.WaitGroup
	for i := 1; i <= 36; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100000; i++ {
				key := fmt.Sprintf("key:%d", rand.Intn(100000))
				client.Set(key, key)
			}

		}()
	}
	wg.Wait()
	fmt.Println("total", client.Len())
}

func parallelRunR() {
	client, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 30000,
		MaxCost:     3000,
		BufferItems: 64,
	})
	var wg sync.WaitGroup
	for i := 1; i <= 36; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100000; i++ {
				key := fmt.Sprintf("key:%d", rand.Intn(100000))
				client.Set(key, key, 1)
			}

		}()
	}
	wg.Wait()
}

func NewZipfian(s, v float64, n uint64) func() (uint64, error) {
	z := rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), s, v, n)
	return func() (uint64, error) {
		return z.Uint64(), nil
	}
}

func parallelZipf() {
	read := &atomic.Int64{}
	write := &atomic.Int64{}
	client, _ := theine.New(3000)
	var wg sync.WaitGroup
	for i := 1; i <= 36; i++ {
		wg.Add(1)
		go func() {
			zipf := NewZipfian(1.0001, 1, 1000000)
			defer wg.Done()
			for i := 0; i < 100000; i++ {
				n, _ := zipf()
				key := fmt.Sprintf("key:%d", n)
				v, ok := client.Get(key)
				if ok {
					read.Add(1)
					if key != v.(string) {
						panic("")
					}
				} else {
					write.Add(1)
					client.Set(key, key)
				}
			}

		}()
	}
	wg.Wait()
	fmt.Println("rw", read.Load(), write.Load(), client.Len())
}

func parallelZipfR() {
	read := &atomic.Int64{}
	write := &atomic.Int64{}
	client, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 30000,
		MaxCost:     3000,
		BufferItems: 64,
	})
	var wg sync.WaitGroup
	for i := 1; i <= 36; i++ {
		wg.Add(1)
		go func() {
			zipf := NewZipfian(1.0001, 1, 1000000)
			defer wg.Done()
			for i := 0; i < 100000; i++ {
				n, _ := zipf()
				key := fmt.Sprintf("key:%d", n)
				v, ok := client.Get(key)
				if ok {
					read.Add(1)
					if key != v.(string) {
						panic("")
					}
				} else {
					write.Add(1)
					client.Set(key, key, 1)
				}
			}

		}()
	}
	wg.Wait()
	fmt.Println("rw", read.Load(), write.Load())
}

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
	now := time.Now()
	parallelZipf()
	fmt.Println(time.Since(now))

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
