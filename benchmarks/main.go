package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	_ "net/http/pprof"

	"github.com/Yiling-J/theine-go/benchmarks/clients"
	"github.com/brianvoe/gofakeit/v6"
)

//go:generate msgp
type Foo struct {
	ID            int
	Str           string
	Int           int
	Pointer       *int
	Name          string         `fake:"{firstname}"`  // Any available function all lowercase
	Sentence      string         `fake:"{sentence:3}"` // Can call with parameters
	RandStr       string         `fake:"{randomstring:[hello,world]}"`
	Number        string         `fake:"{number:1,10}"`       // Comma separated for multiple values
	Regex         string         `fake:"{regex:[abcdef]{5}}"` // Generate string from regex
	Map           map[string]int `fakesize:"2"`
	Array         []string       `fakesize:"2"`
	ArrayRange    []string       `fakesize:"2,6"`
	Bar           Bar
	Skip          *string   `fake:"skip"` // Set to "skip" to not generate data for
	Created       time.Time // Can take in a fake tag as well as a format tag
	CreatedFormat time.Time `fake:"{year}-{month}-{day}" format:"2006-01-02"`
	ByteData      []byte
}

type Bar struct {
	Name   string
	Number int
	Float  float32
}

type MsgpSerializer struct{}

func (m MsgpSerializer) Marshal(o Foo) ([]byte, error) {
	return (&o).MarshalMsg(nil)
}

func (m MsgpSerializer) Unmarshal(d []byte, o *Foo) error {
	_, err := (o).UnmarshalMsg(d)
	return err
}

func infinite(client clients.Client[string, Foo], cap int, concurrency int) {
	// statsviz.RegisterDefault()

	// go func() {
	// 	log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	// }()
	client.Init(cap)
	var wg sync.WaitGroup
	total := atomic.Uint64{}
	miss := atomic.Uint64{}

	fakeData := []Foo{}
	for i := 0; i < 1000; i++ {
		var f Foo
		gofakeit.Struct(&f)
		if i%2 == 0 {
			f.ByteData = make([]byte, 6<<10)
		} else {
			f.ByteData = make([]byte, 100)
		}
		fakeData = append(fakeData, f)
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			z := rand.NewZipf(
				rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, 5000000,
			)
			for {
				total.Add(1)
				n := z.Uint64()
				k := strconv.Itoa(int(n))
				v, ok := client.Get(k)
				if ok {
					if v.ID != int(n) {
						panic("")
					}
				} else {
					miss.Add(1)
					v = fakeData[n%1000]
					v.ID = int(n)
					client.Set(k, v)
				}
			}
		}()
	}
	for {
		time.Sleep(3 * time.Second)
		t := total.Load()
		m := miss.Load()
		fmt.Printf("total: %d, hit ratio: %.2f\n", t, float32(t-m)/float32(t))
		total.Store(0)
		miss.Store(0)
	}
	wg.Wait()

}

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

func main() {
	client := &clients.TheineNvm[string, Foo]{}
	client.KeySerializer = NewMemorySerializer[string]()
	client.ValueSerializer = MsgpSerializer{}
	infinite(client, 100000, 200)
}
