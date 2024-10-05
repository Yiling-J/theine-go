# Theine
[![Go Reference](https://pkg.go.dev/badge/github.com/Yiling-J/theine-go.svg)](https://pkg.go.dev/github.com/Yiling-J/theine-go)
[![codecov](https://codecov.io/gh/Yiling-J/theine-go/branch/main/graph/badge.svg?token=E1HJLJH07V)](https://codecov.io/gh/Yiling-J/theine-go)

High performance in-memory & hybrid cache inspired by [Caffeine](https://github.com/ben-manes/caffeine).


- Good performance
- Support for Generics
- High hit ratio with adaptive [W-TinyLFU](https://arxiv.org/pdf/1512.00727.pdf) eviction policy
- Expired data are removed automatically using [hierarchical timer wheel](http://www.cs.columbia.edu/~nahum/w6998/papers/ton97-timing-wheels.pdf)
- Simple API

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [API](#api)
- [Cache Persistence](#cache-persistence)
- [Benchmarks](#benchmarks)
  * [throughput](#throughput)
  * [hit ratios](#hit-ratios)
- [Hybrid Cache(Experimental)](#hybrid-cacheexperimental)
- [Support](#support)

## Requirements
Go 1.19+

## Installation
```
go get github.com/Yiling-J/theine-go
```

## API

**Builder API**

Theine provides two types of client, simple cache and loading cache. Both of them are initialized from a builder. The difference between simple cache and loading cache is: loading cache's Get method will compute the value using loader function when there is a miss, while simple cache client only return false and do nothing.

Loading cache uses [singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight) to prevent concurrent loading to same key(thundering herd).

simple cache:

```GO
import "github.com/Yiling-J/theine-go"

// key type string, value type string, max size 1000
// max size is the only required configuration to build a client
client, err := theine.NewBuilder[string, string](1000).Build()
if err != nil {
	panic(err)
}

// builder also provide several optional configurations
// you can chain them together and call build once
// client, err := theine.NewBuilder[string, string](1000).Cost(...).Doorkeeper(...).Build()

// or create builder first
builder := theine.NewBuilder[string, string](1000)

// dynamic cost function based on value
// use 0 in Set will call this function to evaluate cost at runtime
builder.Cost(func(v string) int64 {
		return int64(len(v))
})

// doorkeeper
// doorkeeper will drop Set if they are not in bloomfilter yet
// this can improve write performance, but may lower hit ratio
builder.Doorkeeper(true)

// removal listener, this function will be called when entry is removed
// RemoveReason could be REMOVED/EVICTED/EXPIRED
// REMOVED: remove by API
// EVICTED: evicted by Window-TinyLFU policy
// EXPIRED: expired by timing wheel
builder.RemovalListener(func(key K, value V, reason theine.RemoveReason) {})

```
loading cache:

```go
import "github.com/Yiling-J/theine-go"

// loader function: func(ctx context.Context, key K) (theine.Loaded[V], error)
// Loaded struct should include cache value, cost and ttl, which required by Set method
client, err := theine.NewBuilder[string, string](1000).Loading(
	func(ctx context.Context, key string) (theine.Loaded[string], error) {
		return theine.Loaded[string]{Value: key, Cost: 1, TTL: 0}, nil
	},
).Build()
if err != nil {
	panic(err)
}

```
Other builder options are same as simple cache(cost, doorkeeper, removal listener).


**Client API**

```Go
// set, key foo, value bar, cost 1
// success will be false if cost > max size
success := client.Set("foo", "bar", 1)
// cost 0 means using dynamic cost function
// success := client.Set("foo", "bar", 0)

// set with ttl
success = client.SetWithTTL("foo", "bar", 1, 1*time.Second)

// get(simple cache version)
value, ok := client.Get("foo")

// get(loading cache version)
value, err := client.Get(ctx, "foo")

// remove
client.Delete("foo")

// iterate key/value in cache and apply custom function
// if function returns false, range stops the iteration
client.Range(func(key, value int) bool {
	return true
})

// returns an estimation of the cache size usage
client.EstimatedSize()

// get cache stats(in-memory cache only), include hits, misses and hit ratio
client.Stats()

// close client, set hashmaps in shard to nil and close all goroutines
client.Close()

```

## Cache Persistence
Theine supports persisting the cache into `io.Writer` and restoring from `io.Reader`. [Gob](https://pkg.go.dev/encoding/gob) is used to encode/decode data, so **make sure your key/value can be encoded by gob correctly first** before using this feature.

#### API
```go
func (c *Cache[K, V]) SaveCache(version uint64, writer io.Writer) error
func (c *Cache[K, V]) LoadCache(version uint64, reader io.Reader) error
```
**- Important:** please `LoadCache` immediately after client created, or existing entries' TTL might be affected.

#### Example:
```go
// save
f, err := os.Create("test")
err := client.SaveCache(0, f)
f.Close()

// load
f, err = os.Open("test")
require.Nil(t, err)
newClient, err := theine.NewBuilder[int, int](100).Build()
// load immediately after client created
err = newClient.LoadCache(0, f)
f.Close()
```
Version number must be same when saving and loading, or `LoadCache` will return `theine.VersionMismatch` error. You can change the version number when you want to ignore persisted cache.
```go
err := newClient.LoadCache(1, f)
// VersionMismatch is a global variable
if err == theine.VersionMismatch {
	// ignore and skip loading
} else if err != nil {
	// panic error
}
```

#### Details
When persisting cache, Theine roughly do:
- Store version number.
- Store clock(used in TTL).
- Store frequency sketch.
- Store entries one by one in protected LRU in most-recently:least-recently order.
- Store entries one by one in probation LRU in most-recently:least-recently order.
- Loop shards and store entries one by one in each shard deque.

When loading cache, Theine roughly do:
- Load version number, compare to current version number.
- Load clock.
- Load frequency sketch.
- Load protected LRU and insert entries back to new protected LRU and shards/timingwheel, expired entries will be ignored. Because cache capacity may change, this step will stop if max protected LRU size reached.
- Load probation LRU and insert entries back to new probation LRU and shards/timingwheel, expired entries will be ignored, Because cache capacity may change, this step will stop if max probation LRU size reached.
- Load deque entries and insert back to shards, expired entries will be ignored.

Theine will save checksum when persisting cache and verify checksum first when loading.

## Benchmarks

Source: https://github.com/maypok86/benchmarks

### throughput

```
goos: darwin
goarch: amd64
pkg: github.com/maypok86/benchmarks/throughput
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz

BenchmarkCache/zipf_otter_reads=100%,writes=0%-8         	97981804	        10.80 ns/op	  92581426 ops/s
BenchmarkCache/zipf_theine_reads=100%,writes=0%-8        	35600335	        32.25 ns/op	  31010119 ops/s
BenchmarkCache/zipf_ristretto_reads=100%,writes=0%-8     	30536737	        40.39 ns/op	  24756714 ops/s
BenchmarkCache/zipf_ccache_reads=100%,writes=0%-8        	10951008	       107.4 ns/op	   9307602 ops/s
BenchmarkCache/zipf_gcache_reads=100%,writes=0%-8        	 3695197	       392.3 ns/op	   2549346 ops/s
BenchmarkCache/zipf_ttlcache_reads=100%,writes=0%-8      	 1901844	       621.0 ns/op	   1610412 ops/s
BenchmarkCache/zipf_golang-lru_reads=100%,writes=0%-8    	 5925349	       209.1 ns/op	   4781422 ops/s

BenchmarkCache/zipf_otter_reads=0%,writes=100%-8         	 2109556	       540.4 ns/op	   1850519 ops/s
BenchmarkCache/zipf_theine_reads=0%,writes=100%-8        	 3066583	       370.8 ns/op	   2696582 ops/s
BenchmarkCache/zipf_ristretto_reads=0%,writes=100%-8     	 2161082	       580.9 ns/op	   1721398 ops/s
BenchmarkCache/zipf_ccache_reads=0%,writes=100%-8        	 1000000	      1033 ns/op	    967961 ops/s
BenchmarkCache/zipf_gcache_reads=0%,writes=100%-8        	 2832288	       418.2 ns/op	   2391415 ops/s
BenchmarkCache/zipf_ttlcache_reads=0%,writes=100%-8      	 2525420	       455.6 ns/op	   2194879 ops/s
BenchmarkCache/zipf_golang-lru_reads=0%,writes=100%-8    	 3691684	       319.3 ns/op	   3132129 ops/s
```

benchmem:
```
BenchmarkCache/zipf_otter_reads=100%,writes=0%-8         	100362195	        11.54 ns/op	  86621545 ops/s	       0 B/op	       0 allocs/op
BenchmarkCache/zipf_theine_reads=100%,writes=0%-8        	31538078	        32.68 ns/op	  30602449 ops/s	       0 B/op	       0 allocs/op
BenchmarkCache/zipf_ristretto_reads=100%,writes=0%-8     	30308824	        40.52 ns/op	  24676203 ops/s	      16 B/op	       1 allocs/op

BenchmarkCache/zipf_otter_reads=0%,writes=100%-8         	 2232979	       544.6 ns/op	   1836201 ops/s	      80 B/op	       1 allocs/op
BenchmarkCache/zipf_theine_reads=0%,writes=100%-8        	 2854908	       485.1 ns/op	   2061454 ops/s	       0 B/op	       0 allocs/op
BenchmarkCache/zipf_ristretto_reads=0%,writes=100%-8     	 2028530	       670.6 ns/op	   1491240 ops/s	     112 B/op	       3 allocs/op
```

### hit ratios

**zipf**

![hit ratios](benchmarks/results/zipf.png)
**s3**

![hit ratios](benchmarks/results/s3.png)
**ds1**

![hit ratios](benchmarks/results/ds1.png)
**oltp**

![hit ratios](benchmarks/results/oltp.png)


## Hybrid Cache(Experimental)

#### Using Hybrid Cache
NVM: https://github.com/Yiling-J/theine-nvm

#### Limitations
- Cache Persistence is not currently supported, but it may be added in the future. You can still use the Persistence API in a hybrid-enabled cache, but only the DRAM part of the cache will be saved or loaded.
- The removal listener will only receive REMOVED events, which are generated when an entry is explicitly removed by calling the Delete API.
- No Range/Len API.


## Support
Feel free to open an issue or ask question in discussions.
