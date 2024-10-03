package internal

import (
	"math/rand"
	"testing"
)

func BenchmarkPolicy_Read(b *testing.B) {

	store := NewStore[uint64, bool](100000, false, nil, nil, nil, 0, 0, nil)
	r := rand.New(rand.NewSource(0))
	z := rand.NewZipf(r, 1.4, 9.0, 100000)

	witems := []WriteBufItem[uint64, bool]{}
	ritems := []ReadBufItem[uint64, bool]{}
	for i := 0; i < 100000; i++ {
		k := z.Uint64()
		witems = append(witems, WriteBufItem[uint64, bool]{
			entry: &Entry[uint64, bool]{
				key:   k,
				value: true,
				cost:  1,
			},
			cost: 1,
			code: NEW,
		})
	}
	for _, wi := range witems {
		store.policy.Set(wi.entry)
		ritems = append(ritems, ReadBufItem[uint64, bool]{
			entry: wi.entry,
			hash:  store.hasher.hash(wi.entry.key),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.policy.Access(ritems[i&65535])
	}

}

func BenchmarkPolicy_Write(b *testing.B) {

	store := NewStore[uint64, bool](100000, false, nil, nil, nil, 0, 0, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.sinkWrite(WriteBufItem[uint64, bool]{
			entry: &Entry[uint64, bool]{
				key:   uint64(i),
				value: true,
				cost:  1,
			},
			cost: 1,
			code: NEW,
		})
	}

}
