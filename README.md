# Theine
[![codecov](https://codecov.io/gh/Yiling-J/theine-go/branch/main/graph/badge.svg?token=E1HJLJH07V)](https://codecov.io/gh/Yiling-J/theine-go)

High performance in-memory cache inspired by [Caffeine](https://github.com/ben-manes/caffeine).


- Good performance
- Support for Generics
- High hit ratio with adaptive [W-TinyLFU](https://arxiv.org/pdf/1512.00727.pdf) eviction policy
- Expired data are removed automatically using [hierarchical timer wheel](http://www.cs.columbia.edu/~nahum/w6998/papers/ton97-timing-wheels.pdf)

  > TTL must be considered in in-memory caching because
it limits the effective (unexpired) working set size. Efficiently removing expired objects from cache needs to be
prioritized over cache eviction. - [A large scale analysis of hundreds of in-memory
cache clusters at Twitter](https://www.usenix.org/system/files/osdi20-yang.pdf)
- Simple API

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [API](#api)
- [Benchmarks](#benchmarks)
  * [throughput](#throughput)
  * [hit ratios](#hit-ratios)
- [Support](#support)

## Requirements
Go 1.19+

## Installation
```
go get github.com/Yiling-J/theine-go
```

## API

Key should be **comparable**, and value can be any.

- build client


```GO
import "github.com/Yiling-J/theine-go"

// simple cache
// key type string, value type string, max size 1000
// max size is the only required configuration to build a client
client, err := theine.NewBuilder[string, string](1000).Build()
if err != nil {
	panic(err)
}

// loading cache, values are automatically loaded by the cache
// loader function shoud return cache value, cost and ttl.
client, err := theine.NewBuilder[string, string](1000).BuildWithLoader(
	func(ctx context.Context, key string) (theine.Loaded[string], error) {
		return theine.Loaded[string]{Value: key, Cost: 1, TTL: 0}, nil
	},
	false,
)
if err != nil {
	panic(err)
}

// builder also provide several optional configurations
builder := theine.NewBuilder[string, string](1000)

// dynamic cost function based on value
// use 0 in Set will call this function to evaluate cost at runtime
builder.Cost(func(v string) int64 {
		return int64(len(v))
})

// doorkeeper
// doorkeeper will drop Set if they are not in bloomfilter yet
// this can improve write peroformance, but may lower hit ratio
builder.Doorkeeper(true)

// removal listener, this function will be called when entry is removed
// RemoveReason could be REMOVED/EVICTED/EXPIRED
// REMOVED: remove by API
// EVICTED: evicted by Window-TinyLFU policy
// EXPIRED: expired by timing wheel
builder.RemovalListener(func(key K, value V, reason theine.RemoveReason) {})

```

- use client API

```Go
// set, key foo, value bar, cost 1
// success will be false if cost > max size
success := client.Set("foo", "bar", 1)
// cost 0 means using dynamic cost function
// success := client.Set("foo", "bar", 0)

// set with ttl
success = client.SetWithTTL("foo", "bar", 1, 1*time.Second)

// get
value, ok := client.Get("foo")

// remove
client.Delete("foo")

```
## Benchmarks
	
### throughput

Source Code: https://github.com/Yiling-J/theine-go/blob/main/benchmark_test.go

```
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz

BenchmarkGetTheineParallel-12           32432190                36.39 ns/op            0 B/op          0 allocs/op
BenchmarkGetRistrettoParallel-12        63978058                18.86 ns/op           17 B/op          1 allocs/op
BenchmarkSetTheineParallel-12           20791834                84.49 ns/op            0 B/op          0 allocs/op
BenchmarkSetRistrettoParallel-12        23354626                65.53 ns/op          116 B/op          3 allocs/op
BenchmarkZipfTheineParallel-12          14771362                74.72 ns/op            1 B/op          0 allocs/op
BenchmarkZipfRistrettoParallel-12       21031435                61.82 ns/op          100 B/op          3 allocs/op
```

### hit ratios

Source Code: https://github.com/Yiling-J/theine-go/blob/main/benchmarks/main.go

ristretto v0.1.1: https://github.com/dgraph-io/ristretto
> from Ristretto [README](https://github.com/dgraph-io/ristretto#hit-ratios), the hit ratio should be higher. But I can't reproduce their benchmark results. So I open an issue: https://github.com/dgraph-io/ristretto/issues/336

golang-lru v2.0.2: https://github.com/hashicorp/golang-lru

**zipf**

![hit ratios](benchmarks/results/zipf.png)
**search**

This trace is described as "disk read accesses initiated by a large commercial search engine in response to various web search requests."

![hit ratios](benchmarks/results/s3.png)
**database**

This trace is described as "a database server running at a commercial site running an ERP application on top of a commercial database."

![hit ratios](benchmarks/results/ds1.png)
**Scarabresearch database trace**

Scarabresearch 1 hour database trace from this [issue](https://github.com/ben-manes/caffeine/issues/106)

![hit ratios](benchmarks/results/scarab1h.png)
**Meta anonymized trace**

Meta shared anonymized trace captured from large scale production cache services, from [cachelib](https://cachelib.org/docs/Cache_Library_User_Guides/Cachebench_FB_HW_eval/#running-cachebench-with-the-trace-workload)

![hit ratios](benchmarks/results/meta.png)

## Support
Open an issue, ask question in discussions or join discord channel: https://discord.gg/StrgfPaQqE 
