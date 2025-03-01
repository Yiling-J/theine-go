package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/Yiling-J/theine-go"
	// "github.com/go-echarts/statsview"
)

// A simple infinite loop script to monitor heap and GC status during concurrent get/set/update operations.
// Install github.com/go-echarts/statsview, uncomment the relevant code,
// then start the script and visit http://localhost:18066/debug/statsview to view the results.

const CACHE_SIZE = 500000

func keyGen() []uint64 {
	keys := []uint64{}
	r := rand.New(rand.NewSource(0))
	z := rand.NewZipf(r, 1.01, 9.0, CACHE_SIZE*100)
	for i := 0; i < 2<<23; i++ {
		keys = append(keys, z.Uint64())
	}
	return keys
}

type v128 struct {
	_v [128]byte
}

// GetU64 retrieves the first 8 bytes of _v as a uint64
func (v *v128) GetU64() uint64 {
	return binary.LittleEndian.Uint64(v._v[:8])
}

// SetU64 sets the first 8 bytes of _v to the value of the provided uint64
func (v *v128) SetU64(val uint64) {
	binary.LittleEndian.PutUint64(v._v[:8], val)
}

func NewV128(val uint64) v128 {
	var v v128
	v.SetU64(val)
	return v
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// http://localhost:18066/debug/statsview
	// mgr := statsview.New()
	// go func() { _ = mgr.Start() }()

	builder := theine.NewBuilder[uint64, v128](int64(CACHE_SIZE))
	builder.RemovalListener(func(key uint64, value v128, reason theine.RemoveReason) {})
	client, err := builder.Build()
	if err != nil {
		panic("client build failed")
	}
	var wg sync.WaitGroup
	keys := keyGen()

	for i := 0; i < CACHE_SIZE; i++ {
		client.SetWithTTL(
			uint64(i), NewV128(uint64(i)), 1, 500*time.Second,
		)
	}

	fmt.Println("==== start ====")
	for i := 1; i <= 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rd := rand.Intn(2 << 16)
			i := 0
			for {
				keyGet := keys[(i+rd)&(2<<23-1)]
				keyUpdate := keys[(i+3*rd)&(2<<23-1)]

				v, ok := client.Get(keyGet)
				if ok && v.GetU64() != keyGet {
					panic(keyGet)
				}
				if !ok {
					client.SetWithTTL(
						keyGet, NewV128(keyGet), 1, time.Second*time.Duration(i%255+10),
					)
				}

				client.SetWithTTL(
					keyUpdate, NewV128(keyUpdate), int64(keyUpdate&7+1),
					time.Second*time.Duration(i&63+30),
				)
				i++
			}
		}()
	}
	wg.Wait()
	client.Close()
	// mgr.Stop()
}
