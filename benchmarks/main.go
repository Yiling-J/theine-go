package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Yiling-J/theine-go/benchmarks/clients"
)

func infinite(client clients.Client[int, int], cap int, concurrency int) {
	// statsviz.RegisterDefault()

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()
	client.Init(cap)
	var wg sync.WaitGroup
	total := atomic.Uint64{}
	miss := atomic.Uint64{}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			z := rand.NewZipf(
				rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, 10000000,
			)
			for {
				total.Add(1)
				_, get := client.GetSet(int(z.Uint64()), 1)
				if !get {
					miss.Add(1)
				}
			}
		}()
	}
	for {
		time.Sleep(2 * time.Second)
		t := total.Load()
		m := miss.Load()
		fmt.Printf("total: %d, hit ratio: %.2f\n", t, float32(t-m)/float32(t))
	}
	wg.Wait()

}

func main() {
	infinite(&clients.Theine[int, int]{}, 100000, 12)
	// infinite(&clients.Ristretto[int, int]{}, 100000, 12)

}
